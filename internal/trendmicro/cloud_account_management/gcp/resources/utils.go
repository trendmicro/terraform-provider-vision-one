package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1"
	cloudresourcemanagerv2 "google.golang.org/api/cloudresourcemanager/v2"
	cloudresourcemanagerv3 "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

// ===== CAM Project Mutation Utilities =====

const (
	gcpCAMProjectMutationWaitInterval = 5 * time.Second
	gcpCAMProjectMutationWaitTimeout  = 90 * time.Second
	gcpProjectConnectedWaitInterval   = 10 * time.Second
	gcpProjectConnectedWaitTimeout    = 5 * time.Minute
	gcpProjectFailedState             = "failed"
	gcpMissingPrimaryProjectError     = "no primary TF Provider project found"
	gcpCloudPlatformScope             = "https://www.googleapis.com/auth/cloud-platform"

	gcpKeyPropagationWaitTimeout  = 90 * time.Second
	gcpKeyPropagationPollInterval = 10 * time.Second
	gcpRateLimitMaxRetries        = 5

	gcpPrincipalPropagationWaitTimeout  = 90 * time.Second
	gcpPrincipalPropagationPollInterval = 5 * time.Second
)

var (
	gcpPrimaryProjectWaitTimeout  = 10 * time.Minute
	gcpPrimaryProjectWaitInterval = 15 * time.Second
)

type gcpServiceAccountKeyPayload struct {
	ClientEmail string `json:"client_email"`
	ProjectID   string `json:"project_id"`
}

func isGCPKeyPropagationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid_grant") && strings.Contains(msg, "invalid jwt signature")
}

func isGCPPrincipalPropagationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	invalidArg := strings.Contains(msg, "invalid_argument") || strings.Contains(msg, "400")
	if !invalidArg {
		return false
	}
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "does not have any resources") ||
		strings.Contains(msg, "unknown principal")
}

func isPrimaryFromKey(ctx context.Context, encodedKey, projectNumber string) (*bool, error) {
	if encodedKey == "" {
		return nil, nil
	}
	keyJSON, err := decodeServiceAccountKey(encodedKey)
	if err != nil {
		return nil, err
	}
	var payload gcpServiceAccountKeyPayload
	err = json.Unmarshal(keyJSON, &payload)
	if err != nil {
		return nil, fmt.Errorf("invalid service account key JSON: %w", err)
	}
	if payload.ProjectID == "" {
		return nil, fmt.Errorf("service account key missing project_id field")
	}

	cred, err := google.CredentialsFromJSONWithType(ctx, keyJSON, google.ServiceAccount, gcpCloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("failed to create credentials from service account key: %w", err)
	}
	crmClient, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(cred))
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloud Resource Manager client: %w", err)
	}
	number, err := lookupGCPProjectNumber(ctx, crmClient, payload.ProjectID)
	if err != nil {
		return nil, err
	}

	v := fmt.Sprintf("%d", number) == projectNumber
	return &v, nil
}

type gcpProjectNumberLookup struct {
	mu     sync.Mutex
	number int64
	done   bool
}

// gcpProjectNumberCache shares project number lookups across resources in one provider run,
// since many connectors resolve the same primary project against a per-minute read quota.
var gcpProjectNumberCache sync.Map

var gcpRateLimitRetryCooldown = serviceUsageRetryCooldown

func lookupGCPProjectNumber(ctx context.Context, crmClient *cloudresourcemanager.Service, projectID string) (int64, error) {
	value, _ := gcpProjectNumberCache.LoadOrStore(projectID, &gcpProjectNumberLookup{})
	lookup := value.(*gcpProjectNumberLookup)
	lookup.mu.Lock()
	defer lookup.mu.Unlock()
	if lookup.done {
		return lookup.number, nil
	}

	// Retry while the key is still propagating or the read quota is exhausted. Any other
	// failure (a genuine 403, a missing project) is returned immediately rather than waited on.
	deadline := time.Now().Add(gcpKeyPropagationWaitTimeout)
	rateLimitRetries := 0
	for attempt := 1; ; attempt++ {
		project, err := crmClient.Projects.Get(projectID).Context(ctx).Do()
		if err == nil {
			lookup.number, lookup.done = project.ProjectNumber, true
			return lookup.number, nil
		}

		var wait time.Duration
		switch {
		case isGCPKeyPropagationError(err) && time.Now().Before(deadline):
			wait = gcpKeyPropagationPollInterval
		case isGCPRateLimitError(err) && rateLimitRetries < gcpRateLimitMaxRetries:
			wait = gcpRateLimitRetryCooldown(rateLimitRetries)
			rateLimitRetries++
		default:
			return 0, fmt.Errorf("failed to get project %s: %w", projectID, err)
		}
		tflog.Debug(ctx, fmt.Sprintf(
			"[CAM Connector GCP] failed to get project %s (attempt %d), retrying in %s: %s",
			projectID, attempt, wait, err))
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func isGCPRateLimitError(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusTooManyRequests
}

var gcpCAMProjectMutationLocks sync.Map

func lockGCPCAMProjectMutation(projectNumber string) func() {
	lockValue, _ := gcpCAMProjectMutationLocks.LoadOrStore(projectNumber, &sync.Mutex{})
	projectLock := lockValue.(*sync.Mutex)
	projectLock.Lock()

	return projectLock.Unlock
}

func waitForGCPServiceAccountKeyReady(ctx context.Context, encodedKey string) (string, error) {
	keyJSON, err := decodeServiceAccountKey(encodedKey)
	if err != nil {
		return "", err
	}

	var payload gcpServiceAccountKeyPayload
	err = json.Unmarshal(keyJSON, &payload)
	if err != nil {
		return "", fmt.Errorf("invalid service account key JSON: %w", err)
	}
	if payload.ClientEmail == "" {
		return "", fmt.Errorf("invalid service account key JSON: missing client_email")
	}

	jwtConfig, err := google.JWTConfigFromJSON(keyJSON, gcpCloudPlatformScope)
	if err != nil {
		return "", fmt.Errorf("invalid service account key JSON: %w", err)
	}

	tokenSource := jwtConfig.TokenSource(ctx)
	deadline := time.Now().Add(gcpCAMProjectMutationWaitTimeout)

	for {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		token, err := tokenSource.Token()
		if err == nil && token != nil && token.AccessToken != "" {
			return payload.ClientEmail, nil
		}

		if time.Now().After(deadline) {
			if err == nil {
				err = fmt.Errorf("token source returned an empty access token")
			}
			return "", fmt.Errorf(
				"service account key for %s did not become usable within %s: %w",
				payload.ClientEmail,
				gcpCAMProjectMutationWaitTimeout,
				err,
			)
		}

		if err != nil {
			tflog.Debug(ctx, fmt.Sprintf(
				"[GCP CAM] Waiting for service account key for %s to propagate before updating CAM: %s",
				payload.ClientEmail,
				err,
			))
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(gcpCAMProjectMutationWaitInterval):
		}
	}
}

func waitForGCPServiceAccountIAMReady(ctx context.Context, encodedKey, projectID string) (string, error) {
	email, err := waitForGCPServiceAccountKeyReady(ctx, encodedKey)
	if err != nil {
		return "", err
	}

	clientOption, err := newClientOptionFromEncodedServiceAccountKey(ctx, encodedKey)
	if err != nil {
		return email, err
	}

	crmSvc, err := cloudresourcemanager.NewService(ctx, clientOption)
	if err != nil {
		return email, fmt.Errorf("failed to create CRM client for IAM propagation check: %w", err)
	}

	deadline := time.Now().Add(gcpCAMProjectMutationWaitTimeout)
	for {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		_, err := crmSvc.Projects.Get(projectID).Context(ctx).Do()
		if err == nil {
			return email, nil
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf(
				"service account %s IAM bindings for project %s did not propagate within %s: %w",
				email, projectID, gcpCAMProjectMutationWaitTimeout, err,
			)
		}

		tflog.Debug(ctx, fmt.Sprintf(
			"[GCP CAM] Waiting for IAM binding of %s on project %s to propagate: %s",
			email, projectID, err,
		))

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(gcpCAMProjectMutationWaitInterval):
		}
	}
}

type projectNotConnectedError struct {
	projectNumber string
	timeout       time.Duration
	state         string
}

func (e *projectNotConnectedError) Error() string {
	return fmt.Sprintf(
		"project %s did not reach connected state within %s (current state: %q); "+
			"the service account key may not yet have sufficient GCP IAM permissions — wait a few minutes and re-run 'terraform apply'",
		e.projectNumber, e.timeout, e.state,
	)
}

type gcpProjectConnector interface {
	CreateProject(data *api.CreateProjectRequest) error
	ReadProject(projectNumber string) (*api.ProjectResponse, error)
}

func createProjectAndWaitConnected(
	ctx context.Context,
	client gcpProjectConnector,
	body *api.CreateProjectRequest,
	projectNumber string,
	connectTimeout, pollInterval time.Duration,
) (*api.ProjectResponse, error) {
	if err := createProjectWaitingForPrimary(ctx, client, body, gcpPrimaryProjectWaitTimeout, gcpPrimaryProjectWaitInterval); err != nil {
		return nil, err
	}

	return waitForGCPProjectConnected(ctx, client, projectNumber, connectTimeout, pollInterval)
}

// createProjectWaitingForPrimary retries a member project while its primary project, created
// in parallel by the same apply, is not yet registered.
func createProjectWaitingForPrimary(ctx context.Context, client gcpProjectConnector, body *api.CreateProjectRequest, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := client.CreateProject(body)
		if err == nil || strings.Contains(err.Error(), "account-exist") {
			return nil
		}
		if !strings.Contains(err.Error(), gcpMissingPrimaryProjectError) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the primary project was not connected within %s; make sure its connector succeeds, "+
				"then run terraform apply again: %w", timeout, err)
		}

		tflog.Debug(ctx, fmt.Sprintf("[GCP CAM] Waiting for the primary project before connecting project %s", body.ProjectNumber))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func waitForGCPProjectConnected(ctx context.Context, client interface {
	ReadProject(projectNumber string) (*api.ProjectResponse, error)
}, projectNumber string, timeout, interval time.Duration,
) (*api.ProjectResponse, error) {
	deadline := time.Now().Add(timeout)
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		project, err := client.ReadProject(projectNumber)
		if err != nil {
			return nil, err
		}

		if project.State == "managed" || project.State == "outdated" {
			return project, nil
		}

		if time.Now().After(deadline) {
			return nil, &projectNotConnectedError{projectNumber: projectNumber, timeout: timeout, state: project.State}
		}

		tflog.Debug(ctx, fmt.Sprintf(
			"[GCP CAM] Waiting for project %s to reach connected state (current: %q)",
			projectNumber, project.State,
		))

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// ===== Key Decoding Utilities =====

// decodeServiceAccountKey base64-decodes a service account key string into raw JSON bytes.
func decodeServiceAccountKey(encodedKey string) ([]byte, error) {
	keyJSON, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid base64-encoded service account key: %w", err)
	}
	return keyJSON, nil
}

// errLegacyResourceFound is a sentinel returned from Pages() callbacks to stop
// early iteration once a matching legacy resource has been found.
var errLegacyResourceFound = errors.New("found")

func newClientOptionFromEncodedServiceAccountKey(ctx context.Context, encodedKey string) (option.ClientOption, error) {
	keyJSON, err := decodeServiceAccountKey(encodedKey)
	if err != nil {
		return nil, err
	}

	creds, err := google.CredentialsFromJSONWithType(ctx, keyJSON, google.ServiceAccount, gcpCloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("invalid service account key JSON: %w", err)
	}

	return option.WithCredentials(creds), nil
}

// ===== Validation Utilities =====

// ValidateServiceAccountID validates the account ID format.
func ValidateServiceAccountID(accountID string) error {
	if len(accountID) < 6 || len(accountID) > 30 {
		return fmt.Errorf("account_id must be between 6 and 30 characters, got %d characters", len(accountID))
	}
	if strings.HasPrefix(strings.ToLower(accountID), "goog") {
		return fmt.Errorf("account_id cannot start with 'goog' prefix")
	}
	// Regex: ^[a-z](?:[-a-z0-9]{4,28}[a-z0-9])$
	matched, _ := regexp.MatchString(`^[a-z](?:[-a-z0-9]{4,28}[a-z0-9])$`, accountID)
	if !matched {
		return fmt.Errorf("account_id must start with a lowercase letter, followed by lowercase letters, digits, or hyphens, and must end with a letter or digit")
	}
	return nil
}

// ValidateDescription validates the description length in UTF-8 bytes.
func ValidateDescription(description string) error {
	if len([]byte(description)) > 256 {
		return fmt.Errorf("description must be at most 256 UTF-8 bytes, got %d bytes", len([]byte(description)))
	}
	return nil
}

// ===== Project Utilities =====

// IsSystemProject checks if a project is a GCP auto-created project that is not counted for billing.
// Known patterns:
//   - sys-*              : GCP system/infrastructure projects
//   - apps-script-*      : Google Apps Script auto-created projects
//   - google.com:*       : Legacy Google-owned cross-organization projects (e.g. google.com:api-project-*)
func IsSystemProject(project *cloudresourcemanager.Project) bool {
	return IsSystemProjectID(project.ProjectId)
}

func IsSystemProjectID(projectID string) bool {
	return strings.HasPrefix(projectID, "sys-") ||
		strings.HasPrefix(projectID, "apps-script-") ||
		strings.HasPrefix(projectID, "google.com:")
}

// IsFreeTrialProject checks if a project is a free trial project.
func IsFreeTrialProject(project *cloudresourcemanager.Project) bool {
	// Check labels for free trial indicators
	if labels := project.Labels; labels != nil {
		if trial, ok := labels["free-trial"]; ok && trial == "true" {
			return true
		}
		if tier, ok := labels["billing-tier"]; ok && tier == "free" {
			return true
		}
	}
	return false
}

// FilterProjects removes excluded projects from the list.
func FilterProjects(projects, excludeList []string) []string {
	excludeMap := make(map[string]bool)
	for _, proj := range excludeList {
		excludeMap[proj] = true
	}

	var filtered []string
	for _, proj := range projects {
		if !excludeMap[proj] {
			filtered = append(filtered, proj)
		}
	}
	return filtered
}

// ===== Folder Discovery Utilities =====

// DiscoverAllFolders recursively discovers all folders and sub-folders under a given folder.
func DiscoverAllFolders(
	ctx context.Context,
	gcpClients *api.GCPClients,
	folderID string,
) ([]string, error) {
	allFolders := []string{folderID} // Include the parent folder itself

	// Recursively discover sub-folders
	err := DiscoverSubFolders(ctx, gcpClients, folderID, &allFolders)
	if err != nil {
		return nil, err
	}

	return allFolders, nil
}

// DiscoverSubFolders is a recursive helper function to discover all sub-folders.
func DiscoverSubFolders(
	ctx context.Context,
	gcpClients *api.GCPClients,
	parentFolderID string,
	allFolders *[]string,
) error {
	parent := fmt.Sprintf("folders/%s", parentFolderID)

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Listing sub-folders under: %s", parent))

	err := gcpClients.CRMClientV2.Folders.List().Parent(parent).Pages(ctx,
		func(resp *cloudresourcemanagerv2.ListFoldersResponse) error {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Found %d sub-folders under %s", len(resp.Folders), parent))

			for _, folder := range resp.Folders {
				// Only process active folders
				if folder.LifecycleState != config.LIFECYCLE_STATE_ACTIVE {
					tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Skipping inactive folder: %s (state: %s)", folder.Name, folder.LifecycleState))
					continue
				}

				// Extract folder ID from folder name (format: folders/{folder_id})
				folderID := folder.Name[len("folders/"):]
				*allFolders = append(*allFolders, folderID)
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Added sub-folder: %s", folderID))

				// Recursively discover sub-folders
				if err := DiscoverSubFolders(ctx, gcpClients, folderID, allFolders); err != nil {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key] Failed to discover sub-folders under %s: %s", folderID, err.Error()))
					return err
				}
			}
			return nil
		})
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[Service Account Key] Error listing folders under %s: %s", parent, err.Error()))
	}

	return err
}

// DiscoverAllFoldersInOrganization recursively discovers all folders in an organization.
func DiscoverAllFoldersInOrganization(
	ctx context.Context,
	gcpClients *api.GCPClients,
	organizationID string,
) ([]string, error) {
	var allFolders []string
	parent := fmt.Sprintf("organizations/%s", organizationID)

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Listing top-level folders in organization: %s", parent))

	// List all top-level folders directly under the organization
	err := gcpClients.CRMClientV2.Folders.List().Parent(parent).Pages(ctx,
		func(resp *cloudresourcemanagerv2.ListFoldersResponse) error {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Found %d top-level folders in organization", len(resp.Folders)))

			for _, folder := range resp.Folders {
				// Only process active folders
				if folder.LifecycleState != config.LIFECYCLE_STATE_ACTIVE {
					tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Skipping inactive folder: %s (state: %s)", folder.Name, folder.LifecycleState))
					continue
				}

				// Extract folder ID from folder name (format: folders/{folder_id})
				folderID := folder.Name[len("folders/"):]
				allFolders = append(allFolders, folderID)
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Added top-level folder: %s", folderID))

				// Recursively discover sub-folders
				if err := DiscoverSubFolders(ctx, gcpClients, folderID, &allFolders); err != nil {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key] Failed to discover sub-folders under %s: %s", folderID, err.Error()))
					return err
				}
			}
			return nil
		})
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[Service Account Key] Error listing folders in organization %s: %s", parent, err.Error()))
		return nil, err
	}

	return allFolders, nil
}

// ===== IAM Utilities =====

func getIAMPolicyV3(ctx context.Context, gcpClients *api.GCPClients, resource string) (*cloudresourcemanager.Policy, error) {
	policy, err := gcpClients.CRMClient.Projects.GetIamPolicy(resource, &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{RequestedPolicyVersion: 3},
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get IAM policy: %w", err)
	}
	if policy == nil {
		return nil, fmt.Errorf("IAM policy response was empty")
	}
	return policy, nil
}

func missingUnconditionalRoleBindings(policy *cloudresourcemanager.Policy, roleNames []string, member string) []string {
	var missing []string
	seen := make(map[string]bool, len(roleNames))
	for _, roleName := range roleNames {
		if seen[roleName] {
			continue
		}
		seen[roleName] = true

		bound := false
		for _, binding := range policy.Bindings {
			if binding == nil || binding.Role != roleName || binding.Condition != nil {
				continue
			}
			for _, existingMember := range binding.Members {
				if existingMember == member {
					bound = true
					break
				}
			}
			if bound {
				break
			}
		}
		if !bound {
			missing = append(missing, roleName)
		}
	}
	return missing
}

// AddIAMBinding adds IAM binding for service account to a project.
func AddIAMBinding(
	ctx context.Context,
	gcpClients *api.GCPClients,
	projectID string,
	member string,
	roleName string,
) error {
	return RetryIAMPolicyUpdate(ctx, gcpClients, projectID, func(policy *cloudresourcemanager.Policy) (bool, error) {
		// Conditional bindings with the same role are distinct grants; only modify an unconditional binding.
		var binding *cloudresourcemanager.Binding
		for _, b := range policy.Bindings {
			if b != nil && b.Role == roleName && b.Condition == nil {
				binding = b
				break
			}
		}

		// If binding exists, check if member already present
		if binding != nil {
			for _, m := range binding.Members {
				if m == member {
					// Already bound, no change needed
					return false, nil
				}
			}
			// Add member to existing binding
			binding.Members = append(binding.Members, member)
		} else {
			// Create new binding
			policy.Bindings = append(policy.Bindings, &cloudresourcemanager.Binding{
				Role:    roleName,
				Members: []string{member},
			})
		}

		return true, nil
	})
}

// RemoveIAMBinding removes IAM binding for service account from a project.
func RemoveIAMBinding(
	ctx context.Context,
	gcpClients *api.GCPClients,
	projectID string,
	member string,
	roleName string,
) error {
	return RetryIAMPolicyUpdate(ctx, gcpClients, projectID, func(policy *cloudresourcemanager.Policy) (bool, error) {
		// Find binding for this role
		for i, binding := range policy.Bindings {
			if binding == nil || binding.Role != roleName || binding.Condition != nil {
				continue
			}
			// Remove member from the unconditional binding.
			var newMembers []string
			removed := false
			for _, m := range binding.Members {
				if m != member {
					newMembers = append(newMembers, m)
				} else {
					removed = true
				}
			}
			if !removed {
				return false, nil
			}

			// If no members left, remove entire binding.
			if len(newMembers) == 0 {
				policy.Bindings = append(policy.Bindings[:i], policy.Bindings[i+1:]...)
			} else {
				binding.Members = newMembers
			}
			return true, nil
		}
		return false, nil
	})
}

// RetryIAMPolicyUpdate handles get-modify-set with etag-based retries.
func RetryIAMPolicyUpdate(
	ctx context.Context,
	gcpClients *api.GCPClients,
	projectID string,
	modifyFunc func(*cloudresourcemanager.Policy) (bool, error),
) error {
	resource := projectID
	principalDeadline := time.Now().Add(gcpPrincipalPropagationWaitTimeout)

	for attempt := 0; attempt < config.IAM_POLICY_MAX_RETRIES; attempt++ {
		policy, err := getIAMPolicyV3(ctx, gcpClients, resource)
		if err != nil {
			return err
		}

		// Modify policy
		changed, modifyErr := modifyFunc(policy)
		if modifyErr != nil {
			return modifyErr
		}
		if !changed {
			return nil
		}

		// Set updated policy
		setReq := &cloudresourcemanager.SetIamPolicyRequest{
			Policy: policy,
		}
		_, err = gcpClients.CRMClient.Projects.SetIamPolicy(resource, setReq).Context(ctx).Do()
		if err != nil {
			// Check for etag conflict (concurrent modification)
			if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "412") ||
				strings.Contains(err.Error(), "etag") {
				// Exponential backoff
				waitTime := time.Duration(config.IAM_POLICY_RETRY_INITIAL_WAIT*(1<<attempt)) * time.Second
				if waitTime > config.IAM_POLICY_RETRY_MAX_WAIT*time.Second {
					waitTime = config.IAM_POLICY_RETRY_MAX_WAIT * time.Second
				}
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] IAM policy conflict, retrying in %v (attempt %d/%d)",
					waitTime, attempt+1, config.IAM_POLICY_MAX_RETRIES))
				time.Sleep(waitTime)
				continue
			}

			if isGCPPrincipalPropagationError(err) && time.Now().Before(principalDeadline) {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Principal not visible on %s yet, retrying in %v: %s",
					resource, gcpPrincipalPropagationPollInterval, err))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(gcpPrincipalPropagationPollInterval):
				}
				attempt--
				continue
			}
			return fmt.Errorf("failed to set IAM policy: %w", err)
		}

		// Success
		return nil
	}

	return fmt.Errorf("failed to update IAM policy after %d retries due to concurrent modifications",
		config.IAM_POLICY_MAX_RETRIES)
}

// ===== Service Account Utilities =====

// CreateServiceAccount creates a service account with error handling for existing accounts.
func CreateServiceAccount(
	ctx context.Context,
	gcpClients *api.GCPClients,
	projectID string,
	accountID string,
	displayName string,
	description string,
	createIgnoreAlreadyExists bool,
) (*iam.ServiceAccount, error) {
	parent := fmt.Sprintf("projects/%s", projectID)

	request := &iam.CreateServiceAccountRequest{
		AccountId: accountID,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: displayName,
			Description: description,
		},
	}

	sa, err := gcpClients.IAMClient.Projects.ServiceAccounts.Create(parent, request).Context(ctx).Do()
	if err != nil {
		// Handle "already exists" case
		if createIgnoreAlreadyExists && strings.Contains(err.Error(), "already exists") {
			email := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", accountID, projectID)
			saName := fmt.Sprintf("projects/%s/serviceAccounts/%s", projectID, email)

			// Try to get existing service account
			existingSA, getErr := gcpClients.IAMClient.Projects.ServiceAccounts.Get(saName).Context(ctx).Do()
			if getErr != nil {
				// Check if it's a soft-deleted account that can't be retrieved
				if strings.Contains(getErr.Error(), "404") || strings.Contains(getErr.Error(), "not found") {
					return nil, fmt.Errorf("service account with email %s is in 30-day soft-delete period and cannot be used; please wait for deletion to complete or use a different account_id", email)
				}
				return nil, fmt.Errorf("service account with email %s already exists but cannot be retrieved: %w", email, getErr)
			}

			// Check if the service account is disabled
			if existingSA.Disabled {
				return nil, fmt.Errorf("service account %s exists but is disabled; please enable it manually or use a different account_id", email)
			}

			// Verify the service account is usable by attempting to list its keys
			// This helps detect accounts in unusual states (like soft-deleted but still retrievable)
			_, listErr := gcpClients.IAMClient.Projects.ServiceAccounts.Keys.List(saName).Context(ctx).Do()
			if listErr != nil {
				if strings.Contains(listErr.Error(), "404") || strings.Contains(listErr.Error(), "not found") {
					return nil, fmt.Errorf("service account %s exists but is not fully operational (possibly in soft-delete period); please wait for deletion to complete or use a different account_id", email)
				}
				tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Warning: Could not list keys for existing service account %s: %s", email, listErr.Error()))
			}

			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Adopting existing service account: %s", email))
			return existingSA, nil
		}

		// Handle soft-deleted account
		if strings.Contains(err.Error(), "deleted") || strings.Contains(err.Error(), "PENDING_DELETE") {
			return nil, fmt.Errorf("service account with email %s@%s.iam.gserviceaccount.com is in 30-day deletion period; please wait or use a different account_id",
				accountID, projectID)
		}

		return nil, fmt.Errorf("failed to create service account: %w", err)
	}

	return sa, nil
}

// CreateServiceAccountKey creates a new service account key.
func CreateServiceAccountKey(
	ctx context.Context,
	gcpClients *api.GCPClients,
	serviceAccountName string,
) (*iam.ServiceAccountKey, error) {
	request := &iam.CreateServiceAccountKeyRequest{
		PrivateKeyType: config.PRIVATE_KEY_TYPE_GOOGLE_CREDENTIALS,
		KeyAlgorithm:   config.KEY_ALGORITHM_RSA_2048,
	}

	key, err := gcpClients.IAMClient.Projects.ServiceAccounts.Keys.Create(
		serviceAccountName, request).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create service account key: %w", err)
	}

	return key, nil
}

// DeleteServiceAccountKey deletes a service account key with error handling.
func DeleteServiceAccountKey(
	ctx context.Context,
	gcpClients *api.GCPClients,
	keyName string,
) error {
	_, err := gcpClients.IAMClient.Projects.ServiceAccounts.Keys.Delete(keyName).Context(ctx).Do()
	if err != nil {
		// Ignore 404 - key already deleted
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key] Key already deleted: %s", keyName))
			return nil
		}
		return fmt.Errorf("failed to delete service account key: %w", err)
	}
	return nil
}

// AddFolderIAMBinding adds an IAM binding for a member on a GCP folder.
func AddFolderIAMBinding(ctx context.Context, gcpClients *api.GCPClients, folderID, member, role string) error {
	resource := fmt.Sprintf("folders/%s", folderID)
	for attempt := 0; attempt < config.IAM_POLICY_MAX_RETRIES; attempt++ {
		policy, err := gcpClients.CRMClientV2.Folders.GetIamPolicy(resource, &cloudresourcemanagerv2.GetIamPolicyRequest{
			Options: &cloudresourcemanagerv2.GetPolicyOptions{RequestedPolicyVersion: 3},
		}).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get folder IAM policy: %w", err)
		}

		alreadyBound := false
		var binding *cloudresourcemanagerv2.Binding
		for _, b := range policy.Bindings {
			if b != nil && b.Role == role && b.Condition == nil {
				binding = b
				break
			}
		}
		if binding != nil {
			for _, m := range binding.Members {
				if m == member {
					alreadyBound = true
					break
				}
			}
			if !alreadyBound {
				binding.Members = append(binding.Members, member)
			}
		} else {
			policy.Bindings = append(policy.Bindings, &cloudresourcemanagerv2.Binding{
				Role:    role,
				Members: []string{member},
			})
		}

		if alreadyBound {
			return nil
		}

		_, err = gcpClients.CRMClientV2.Folders.SetIamPolicy(resource, &cloudresourcemanagerv2.SetIamPolicyRequest{Policy: policy}).Context(ctx).Do()
		if err != nil {
			if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "412") || strings.Contains(err.Error(), "etag") {
				waitTime := time.Duration(config.IAM_POLICY_RETRY_INITIAL_WAIT*(1<<attempt)) * time.Second
				if waitTime > config.IAM_POLICY_RETRY_MAX_WAIT*time.Second {
					waitTime = config.IAM_POLICY_RETRY_MAX_WAIT * time.Second
				}
				time.Sleep(waitTime)
				continue
			}
			return fmt.Errorf("failed to set folder IAM policy: %w", err)
		}
		return nil
	}
	return fmt.Errorf("failed to update folder IAM policy after %d retries", config.IAM_POLICY_MAX_RETRIES)
}

// RemoveFolderIAMBinding removes an IAM binding for a member on a GCP folder.
func RemoveFolderIAMBinding(ctx context.Context, gcpClients *api.GCPClients, folderID, member, role string) error {
	resource := fmt.Sprintf("folders/%s", folderID)
	for attempt := 0; attempt < config.IAM_POLICY_MAX_RETRIES; attempt++ {
		policy, err := gcpClients.CRMClientV2.Folders.GetIamPolicy(resource, &cloudresourcemanagerv2.GetIamPolicyRequest{
			Options: &cloudresourcemanagerv2.GetPolicyOptions{RequestedPolicyVersion: 3},
		}).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get folder IAM policy: %w", err)
		}

		changed := false
		for i, binding := range policy.Bindings {
			if binding == nil || binding.Role != role || binding.Condition != nil {
				continue
			}
			var newMembers []string
			removed := false
			for _, m := range binding.Members {
				if m != member {
					newMembers = append(newMembers, m)
				} else {
					removed = true
				}
			}
			if !removed {
				return nil
			}
			if len(newMembers) == 0 {
				policy.Bindings = append(policy.Bindings[:i], policy.Bindings[i+1:]...)
			} else {
				binding.Members = newMembers
			}
			changed = true
			break
		}
		if !changed {
			return nil
		}

		_, err = gcpClients.CRMClientV2.Folders.SetIamPolicy(resource, &cloudresourcemanagerv2.SetIamPolicyRequest{Policy: policy}).Context(ctx).Do()
		if err != nil {
			if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "412") || strings.Contains(err.Error(), "etag") {
				waitTime := time.Duration(config.IAM_POLICY_RETRY_INITIAL_WAIT*(1<<attempt)) * time.Second
				if waitTime > config.IAM_POLICY_RETRY_MAX_WAIT*time.Second {
					waitTime = config.IAM_POLICY_RETRY_MAX_WAIT * time.Second
				}
				time.Sleep(waitTime)
				continue
			}
			return fmt.Errorf("failed to set folder IAM policy: %w", err)
		}
		return nil
	}
	return fmt.Errorf("failed to update folder IAM policy after %d retries", config.IAM_POLICY_MAX_RETRIES)
}

// HasFolderRoleBinding checks if a member has a specific unconditional role binding on a GCP folder.
func HasFolderRoleBinding(ctx context.Context, gcpClients *api.GCPClients, folderID, member, role string) (bool, error) {
	resource := fmt.Sprintf("folders/%s", folderID)
	policy, err := gcpClients.CRMClientV2.Folders.GetIamPolicy(resource, &cloudresourcemanagerv2.GetIamPolicyRequest{
		Options: &cloudresourcemanagerv2.GetPolicyOptions{RequestedPolicyVersion: 3},
	}).Context(ctx).Do()
	if err != nil {
		return false, fmt.Errorf("failed to get folder IAM policy: %w", err)
	}
	for _, binding := range policy.Bindings {
		if binding == nil || binding.Role != role || binding.Condition != nil {
			continue
		}
		for _, m := range binding.Members {
			if m == member {
				return true, nil
			}
		}
	}
	return false, nil
}

// AddOrgIAMBinding adds an IAM binding for a member on a GCP organization.
func AddOrgIAMBinding(ctx context.Context, gcpClients *api.GCPClients, orgID, member, role string) error {
	resource := fmt.Sprintf("organizations/%s", orgID)
	for attempt := 0; attempt < config.IAM_POLICY_MAX_RETRIES; attempt++ {
		policy, err := gcpClients.CRMClient.Organizations.GetIamPolicy(resource, &cloudresourcemanager.GetIamPolicyRequest{
			Options: &cloudresourcemanager.GetPolicyOptions{RequestedPolicyVersion: 3},
		}).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get organization IAM policy: %w", err)
		}

		alreadyBound := false
		var binding *cloudresourcemanager.Binding
		for _, b := range policy.Bindings {
			if b != nil && b.Role == role && b.Condition == nil {
				binding = b
				break
			}
		}
		if binding != nil {
			for _, m := range binding.Members {
				if m == member {
					alreadyBound = true
					break
				}
			}
			if !alreadyBound {
				binding.Members = append(binding.Members, member)
			}
		} else {
			policy.Bindings = append(policy.Bindings, &cloudresourcemanager.Binding{
				Role:    role,
				Members: []string{member},
			})
		}

		if alreadyBound {
			return nil
		}

		_, err = gcpClients.CRMClient.Organizations.SetIamPolicy(resource, &cloudresourcemanager.SetIamPolicyRequest{Policy: policy}).Context(ctx).Do()
		if err != nil {
			if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "412") || strings.Contains(err.Error(), "etag") {
				waitTime := time.Duration(config.IAM_POLICY_RETRY_INITIAL_WAIT*(1<<attempt)) * time.Second
				if waitTime > config.IAM_POLICY_RETRY_MAX_WAIT*time.Second {
					waitTime = config.IAM_POLICY_RETRY_MAX_WAIT * time.Second
				}
				time.Sleep(waitTime)
				continue
			}
			return fmt.Errorf("failed to set organization IAM policy: %w", err)
		}
		return nil
	}
	return fmt.Errorf("failed to update organization IAM policy after %d retries", config.IAM_POLICY_MAX_RETRIES)
}

// RemoveOrgIAMBinding removes an IAM binding for a member on a GCP organization.
func RemoveOrgIAMBinding(ctx context.Context, gcpClients *api.GCPClients, orgID, member, role string) error {
	resource := fmt.Sprintf("organizations/%s", orgID)
	for attempt := 0; attempt < config.IAM_POLICY_MAX_RETRIES; attempt++ {
		policy, err := gcpClients.CRMClient.Organizations.GetIamPolicy(resource, &cloudresourcemanager.GetIamPolicyRequest{
			Options: &cloudresourcemanager.GetPolicyOptions{RequestedPolicyVersion: 3},
		}).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get organization IAM policy: %w", err)
		}

		changed := false
		for i, binding := range policy.Bindings {
			if binding == nil || binding.Role != role || binding.Condition != nil {
				continue
			}
			var newMembers []string
			removed := false
			for _, m := range binding.Members {
				if m != member {
					newMembers = append(newMembers, m)
				} else {
					removed = true
				}
			}
			if !removed {
				return nil
			}
			if len(newMembers) == 0 {
				policy.Bindings = append(policy.Bindings[:i], policy.Bindings[i+1:]...)
			} else {
				binding.Members = newMembers
			}
			changed = true
			break
		}
		if !changed {
			return nil
		}

		_, err = gcpClients.CRMClient.Organizations.SetIamPolicy(resource, &cloudresourcemanager.SetIamPolicyRequest{Policy: policy}).Context(ctx).Do()
		if err != nil {
			if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "412") || strings.Contains(err.Error(), "etag") {
				waitTime := time.Duration(config.IAM_POLICY_RETRY_INITIAL_WAIT*(1<<attempt)) * time.Second
				if waitTime > config.IAM_POLICY_RETRY_MAX_WAIT*time.Second {
					waitTime = config.IAM_POLICY_RETRY_MAX_WAIT * time.Second
				}
				time.Sleep(waitTime)
				continue
			}
			return fmt.Errorf("failed to set organization IAM policy: %w", err)
		}
		return nil
	}
	return fmt.Errorf("failed to update organization IAM policy after %d retries", config.IAM_POLICY_MAX_RETRIES)
}

// HasOrgRoleBinding checks if a member has a specific unconditional role binding on a GCP organization.
func HasOrgRoleBinding(ctx context.Context, gcpClients *api.GCPClients, orgID, member, role string) (bool, error) {
	resource := fmt.Sprintf("organizations/%s", orgID)
	policy, err := gcpClients.CRMClient.Organizations.GetIamPolicy(resource, &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{RequestedPolicyVersion: 3},
	}).Context(ctx).Do()
	if err != nil {
		return false, fmt.Errorf("failed to get organization IAM policy: %w", err)
	}
	for _, binding := range policy.Bindings {
		if binding == nil || binding.Role != role || binding.Condition != nil {
			continue
		}
		for _, m := range binding.Members {
			if m == member {
				return true, nil
			}
		}
	}
	return false, nil
}

// ===== Tag Operation Utilities =====
// WaitForTagOperation waits for a long-running tag operation to complete.
func WaitForTagOperation(ctx context.Context, service *cloudresourcemanagerv3.Service, operationName string) (*cloudresourcemanagerv3.Operation, error) {
	if operationName == "" {
		return nil, fmt.Errorf("operation name is empty, cannot poll operation status")
	}
	maxRetries := 60 // Maximum 2 minutes (60 * 2s)
	retryInterval := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		op, err := service.Operations.Get(operationName).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get operation status: %w", err)
		}

		if op.Done {
			return op, nil
		}

		// Wait before polling again
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
			// Continue polling
		}
	}

	return nil, fmt.Errorf("operation timed out after %v", time.Duration(maxRetries)*retryInterval)
}
