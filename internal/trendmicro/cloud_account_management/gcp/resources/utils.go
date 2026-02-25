package resources

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/cloudresourcemanager/v1"
	cloudresourcemanagerv2 "google.golang.org/api/cloudresourcemanager/v2"
	cloudresourcemanagerv3 "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/iam/v1"
)

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

// HasRoleBinding checks if a specific role binding exists for a member in the IAM policy.
func HasRoleBinding(policy *cloudresourcemanager.Policy, roleName, member string) bool {
	for _, binding := range policy.Bindings {
		if binding.Role != roleName {
			continue
		}
		for _, m := range binding.Members {
			if m == member {
				return true
			}
		}
	}
	return false
}

// AddIAMBinding adds IAM binding for service account to a project.
func AddIAMBinding(
	ctx context.Context,
	gcpClients *api.GCPClients,
	projectID string,
	member string,
	roleName string,
) error {
	return RetryIAMPolicyUpdate(ctx, gcpClients, projectID, func(policy *cloudresourcemanager.Policy) error {
		// Find existing binding for this role
		var binding *cloudresourcemanager.Binding
		for _, b := range policy.Bindings {
			if b.Role == roleName {
				binding = b
				break
			}
		}

		// If binding exists, check if member already present
		if binding != nil {
			for _, m := range binding.Members {
				if m == member {
					// Already bound, no change needed
					return nil
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

		return nil
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
	return RetryIAMPolicyUpdate(ctx, gcpClients, projectID, func(policy *cloudresourcemanager.Policy) error {
		// Find binding for this role
		for i, binding := range policy.Bindings {
			if binding.Role == roleName {
				// Remove member from binding
				var newMembers []string
				for _, m := range binding.Members {
					if m != member {
						newMembers = append(newMembers, m)
					}
				}

				// If no members left, remove entire binding
				if len(newMembers) == 0 {
					policy.Bindings = append(policy.Bindings[:i], policy.Bindings[i+1:]...)
				} else {
					binding.Members = newMembers
				}
				break
			}
		}
		return nil
	})
}

// RetryIAMPolicyUpdate handles get-modify-set with etag-based retries.
func RetryIAMPolicyUpdate(
	ctx context.Context,
	gcpClients *api.GCPClients,
	projectID string,
	modifyFunc func(*cloudresourcemanager.Policy) error,
) error {
	resource := projectID

	for attempt := 0; attempt < config.IAM_POLICY_MAX_RETRIES; attempt++ {
		// Get current policy
		getReq := &cloudresourcemanager.GetIamPolicyRequest{}
		policy, err := gcpClients.CRMClient.Projects.GetIamPolicy(resource, getReq).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get IAM policy: %w", err)
		}

		// Modify policy
		if modifyErr := modifyFunc(policy); modifyErr != nil {
			return modifyErr
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

// ===== Tag Operation Utilities =====
// WaitForTagOperation waits for a long-running tag operation to complete.
func WaitForTagOperation(ctx context.Context, service *cloudresourcemanagerv3.Service, operationName string) (*cloudresourcemanagerv3.Operation, error) {
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
