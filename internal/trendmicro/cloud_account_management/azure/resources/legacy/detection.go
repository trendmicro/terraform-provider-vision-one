package legacy

import (
	"context"
	"crypto/md5" //nolint:gosec // MD5 used for Azure storage account naming only, not cryptographic security
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/microsoftgraph/msgraph-sdk-go/applications"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"
)

// LegacyResourceSet represents all V1 resources for a subscription
type LegacyResourceSet struct {
	SubscriptionID     string
	CustomRole         *LegacyCustomRole
	ResourceGroup      *LegacyResourceGroup
	StorageAccount     *LegacyStorageAccount
	AppRegistration    *LegacyAppRegistration  // Only for primary subscription
	ServicePrincipal   *LegacyServicePrincipal // Only for primary subscription
	FederatedIdentity  *LegacyFedIdentity      // Only for primary subscription
	DetectionTimestamp time.Time
}

// LegacyCustomRole represents a V1 custom role definition
type LegacyCustomRole struct {
	Name          string // v1-custom-role-{sub-id}
	ID            string // Full Azure resource ID
	Scope         string // /subscriptions/{sub-id}
	Exists        bool
	AssignmentIDs []string // Associated role assignments
}

// LegacyResourceGroup represents a V1 resource group
type LegacyResourceGroup struct {
	Name     string // trendmicro-v1-{sub-id}
	Location string
	Exists   bool
	Tags     map[string]string
}

// LegacyStorageAccount represents a V1 storage account for Terraform state
type LegacyStorageAccount struct {
	Name              string // camtfstatestorage{8-char-hash}
	ResourceGroupName string
	Location          string
	Exists            bool
	ContainerName     string // camtfstate{sub-id}
	StateFileExists   bool   // terraform.tfstate in container
}

// LegacyAppRegistration represents a V1 App Registration
type LegacyAppRegistration struct {
	DisplayName string // v1-app-registration-{sub-id}
	ObjectID    string
	ClientID    string
	Exists      bool
}

// LegacyServicePrincipal represents a V1 Service Principal
type LegacyServicePrincipal struct {
	ObjectID string
	ClientID string
	Tags     []string
	Exists   bool
}

// LegacyFedIdentity represents a V1 Federated Identity Credential
type LegacyFedIdentity struct {
	DisplayName string // v1-fed-cred
	Issuer      string
	Subject     string
	Exists      bool
}

// Naming helper functions
func GenerateLegacyCustomRoleName(subscriptionID string) string {
	return fmt.Sprintf("%s%s", config.LEGACY_AZURE_CUSTOM_ROLE_PREFIX, subscriptionID)
}

func GenerateLegacyResourceGroupName(subscriptionID string) string {
	return fmt.Sprintf("%s%s", config.LEGACY_AZURE_RESOURCE_GROUP_PREFIX, subscriptionID)
}

func GenerateLegacyStorageAccountName(subscriptionID string) string {
	// V1 uses MD5 hash of subscription ID (first 8 chars)
	hash := md5.Sum([]byte(subscriptionID)) //nolint:gosec // MD5 used for Azure storage account naming only, not cryptographic security
	hashStr := hex.EncodeToString(hash[:])[:8]
	return fmt.Sprintf("%s%s", config.LEGACY_AZURE_STORAGE_ACCOUNT_PREFIX, hashStr)
}

func GenerateLegacyAppRegistrationName(subscriptionID string) string {
	return fmt.Sprintf("%s%s", config.LEGACY_AZURE_APP_REGISTRATION_PREFIX, subscriptionID)
}

func GenerateLegacyContainerName(subscriptionID string) string {
	return fmt.Sprintf("%s%s", config.LEGACY_AZURE_CONTAINER_PREFIX, subscriptionID)
}

func GenerateLegacyFederatedIdentityName() string {
	return config.LEGACY_AZURE_FEDERATED_IDENTITY_NAME
}

// DetectV1Resources detects all V1 resources for a subscription
func DetectV1Resources(ctx context.Context, subscriptionID string, includeAppRegistration bool) (*LegacyResourceSet, error) {
	tflog.Info(ctx, fmt.Sprintf("[Legacy Detection] Starting detection for subscription: %s", subscriptionID))

	result := &LegacyResourceSet{
		SubscriptionID:     subscriptionID,
		DetectionTimestamp: time.Now(),
	}

	// Detect Custom Role
	customRole, err := DetectCustomRole(ctx, subscriptionID)
	if err != nil {
		tflog.Error(ctx, fmt.Sprintf("[Legacy Detection] Failed to detect custom role: %s", err))
		return nil, fmt.Errorf("failed to detect custom role: %w", err)
	}
	result.CustomRole = customRole

	// Detect Resource Group
	resourceGroup, err := DetectResourceGroup(ctx, subscriptionID)
	if err != nil {
		tflog.Error(ctx, fmt.Sprintf("[Legacy Detection] Failed to detect resource group: %s", err))
		return nil, fmt.Errorf("failed to detect resource group: %w", err)
	}
	result.ResourceGroup = resourceGroup

	// Detect Storage Account (if resource group exists)
	if resourceGroup.Exists {
		storageAccount, err := DetectStorageAccount(ctx, subscriptionID, resourceGroup.Name)
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Legacy Detection] Failed to detect storage account: %s", err))
			// Non-critical, continue
		} else {
			result.StorageAccount = storageAccount
		}
	}

	// Detect App Registration (only if requested)
	if includeAppRegistration {
		appReg, err := DetectAppRegistration(ctx, subscriptionID)
		if err != nil {
			tflog.Error(ctx, fmt.Sprintf("[Legacy Detection] Failed to detect app registration: %s", err))
			return nil, fmt.Errorf("failed to detect app registration: %w", err)
		}
		result.AppRegistration = appReg

		// Detect Service Principal (if app registration exists)
		if appReg.Exists {
			sp, err := DetectServicePrincipal(ctx, appReg.ClientID)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("[Legacy Detection] Failed to detect service principal: %s", err))
			} else {
				result.ServicePrincipal = sp
			}

			// Detect Federated Identity
			fedIdentity, err := DetectFederatedIdentity(ctx, appReg.ObjectID)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("[Legacy Detection] Failed to detect federated identity: %s", err))
			} else {
				result.FederatedIdentity = fedIdentity
			}
		}
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Detection] Completed detection for subscription: %s", subscriptionID))
	return result, nil
}

// DetectCustomRole detects V1 custom role definition
func DetectCustomRole(ctx context.Context, subscriptionID string) (*LegacyCustomRole, error) {
	roleName := GenerateLegacyCustomRoleName(subscriptionID)
	scope := fmt.Sprintf("/subscriptions/%s", subscriptionID)

	tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Detecting custom role: %s in scope: %s", roleName, scope))

	// Get Azure clients
	azureClient, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to get Azure clients: %v", diags)
	}

	// List role definitions with filter
	pager := azureClient.RoleClient.NewListPager(scope, &armauthorization.RoleDefinitionsClientListOptions{
		Filter: &roleName,
	})

	var foundRole *armauthorization.RoleDefinition
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list role definitions: %w", err)
		}

		for _, role := range page.Value {
			if role.Properties != nil && role.Properties.RoleName != nil && *role.Properties.RoleName == roleName {
				foundRole = role
				break
			}
		}
		if foundRole != nil {
			break
		}
	}

	if foundRole == nil {
		tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Custom role not found: %s", roleName))
		return &LegacyCustomRole{
			Name:   roleName,
			Scope:  scope,
			Exists: false,
		}, nil
	}

	// Get role assignments for this role
	assignmentIDs, err := getRoleAssignments(ctx, azureClient, *foundRole.ID, scope)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[Legacy Detection] Failed to get role assignments: %s", err))
		assignmentIDs = []string{}
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Detection] Found custom role: %s with %d assignments", roleName, len(assignmentIDs)))

	return &LegacyCustomRole{
		Name:          roleName,
		ID:            *foundRole.ID,
		Scope:         scope,
		Exists:        true,
		AssignmentIDs: assignmentIDs,
	}, nil
}

// getRoleAssignments gets all role assignments for a role definition
func getRoleAssignments(ctx context.Context, azureClient *api.AzureClients, roleDefinitionID, scope string) ([]string, error) {
	assignmentClient, err := armauthorization.NewRoleAssignmentsClient(azureClient.SubscriptionID, azureClient.Credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create role assignments client: %w", err)
	}

	var assignmentIDs []string
	pager := assignmentClient.NewListForScopePager(scope, &armauthorization.RoleAssignmentsClientListForScopeOptions{
		Filter: nil,
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list role assignments: %w", err)
		}

		for _, assignment := range page.Value {
			if assignment.Properties == nil || assignment.Properties.RoleDefinitionID == nil {
				continue
			}
			if *assignment.Properties.RoleDefinitionID == roleDefinitionID {
				assignmentIDs = append(assignmentIDs, *assignment.ID)
			}
		}
	}

	return assignmentIDs, nil
}

// DetectResourceGroup detects V1 resource group
func DetectResourceGroup(ctx context.Context, subscriptionID string) (*LegacyResourceGroup, error) {
	rgName := GenerateLegacyResourceGroupName(subscriptionID)

	tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Detecting resource group: %s", rgName))

	// Get Azure clients
	azureClient, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to get Azure clients: %v", diags)
	}

	// Check if resource group exists
	rg, err := azureClient.RGClient.Get(ctx, rgName, nil)
	if err != nil {
		// Check if error is "not found"
		if strings.Contains(err.Error(), "ResourceGroupNotFound") || strings.Contains(err.Error(), "NotFound") {
			tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Resource group not found: %s", rgName))
			return &LegacyResourceGroup{
				Name:   rgName,
				Exists: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get resource group: %w", err)
	}

	tags := make(map[string]string)
	if rg.Tags != nil {
		for k, v := range rg.Tags {
			if v != nil {
				tags[k] = *v
			}
		}
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Detection] Found resource group: %s in location: %s", rgName, *rg.Location))

	return &LegacyResourceGroup{
		Name:     rgName,
		Location: *rg.Location,
		Exists:   true,
		Tags:     tags,
	}, nil
}

// DetectStorageAccount detects V1 storage account
func DetectStorageAccount(ctx context.Context, subscriptionID, rgName string) (*LegacyStorageAccount, error) {
	saName := GenerateLegacyStorageAccountName(subscriptionID)
	containerName := GenerateLegacyContainerName(subscriptionID)

	tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Detecting storage account: %s in RG: %s", saName, rgName))

	// TODO: Implement storage account detection using Azure Storage SDK
	// For now, return a placeholder
	tflog.Warn(ctx, "[Legacy Detection] Storage account detection not yet implemented")

	return &LegacyStorageAccount{
		Name:              saName,
		ResourceGroupName: rgName,
		ContainerName:     containerName,
		Exists:            false,
		StateFileExists:   false,
	}, nil
}

// DetectAppRegistration detects V1 App Registration
func DetectAppRegistration(ctx context.Context, subscriptionID string) (*LegacyAppRegistration, error) {
	displayName := GenerateLegacyAppRegistrationName(subscriptionID)

	tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Detecting app registration: %s", displayName))

	// Get Graph client for Microsoft Graph API operations
	graphClient, _, diags := api.GetGraphClient(ctx)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to get Graph client: %v", diags)
	}

	// Query App Registrations with filter
	filter := fmt.Sprintf("displayName eq '%s'", displayName)
	apps, err := graphClient.Applications().Get(ctx, &applications.ApplicationsRequestBuilderGetRequestConfiguration{
		QueryParameters: &applications.ApplicationsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list app registrations: %w", err)
	}

	if apps.GetValue() == nil || len(apps.GetValue()) == 0 {
		tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] App registration not found: %s", displayName))
		return &LegacyAppRegistration{
			DisplayName: displayName,
			Exists:      false,
		}, nil
	}

	app := apps.GetValue()[0]
	appID := app.GetId()
	clientID := app.GetAppId()

	if appID == nil || clientID == nil {
		return nil, fmt.Errorf("app registration missing required fields")
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Detection] Found app registration: %s", displayName))

	return &LegacyAppRegistration{
		DisplayName: displayName,
		ObjectID:    *appID,
		ClientID:    *clientID,
		Exists:      true,
	}, nil
}

// DetectServicePrincipal detects V1 Service Principal by client ID
func DetectServicePrincipal(ctx context.Context, clientID string) (*LegacyServicePrincipal, error) {
	tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Detecting service principal with client ID: %s", clientID))

	// Get Graph client for Microsoft Graph API operations
	graphClient, _, diags := api.GetGraphClient(ctx)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to get Graph client: %v", diags)
	}

	// Query Service Principals with filter
	filter := fmt.Sprintf("appId eq '%s'", clientID)
	sps, err := graphClient.ServicePrincipals().Get(ctx, &serviceprincipals.ServicePrincipalsRequestBuilderGetRequestConfiguration{
		QueryParameters: &serviceprincipals.ServicePrincipalsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list service principals: %w", err)
	}

	if sps.GetValue() == nil || len(sps.GetValue()) == 0 {
		tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Service principal not found for client ID: %s", clientID))
		return &LegacyServicePrincipal{
			ClientID: clientID,
			Exists:   false,
		}, nil
	}

	sp := sps.GetValue()[0]
	spID := sp.GetId()

	if spID == nil {
		return nil, fmt.Errorf("service principal missing object ID")
	}

	var tags []string
	if sp.GetTags() != nil {
		tags = sp.GetTags()
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Detection] Found service principal: %s", *spID))

	return &LegacyServicePrincipal{
		ObjectID: *spID,
		ClientID: clientID,
		Tags:     tags,
		Exists:   true,
	}, nil
}

// DetectFederatedIdentity detects V1 Federated Identity Credential
func DetectFederatedIdentity(ctx context.Context, appObjectID string) (*LegacyFedIdentity, error) {
	fedName := GenerateLegacyFederatedIdentityName()

	tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Detecting federated identity: %s for app: %s", fedName, appObjectID))

	// Get Graph client for Microsoft Graph API operations
	graphClient, _, diags := api.GetGraphClient(ctx)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to get Graph client: %v", diags)
	}

	// List federated identity credentials
	fedCreds, err := graphClient.Applications().ByApplicationId(appObjectID).FederatedIdentityCredentials().Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list federated identity credentials: %w", err)
	}

	if fedCreds.GetValue() == nil || len(fedCreds.GetValue()) == 0 {
		tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Federated identity not found: %s", fedName))
		return &LegacyFedIdentity{
			DisplayName: fedName,
			Exists:      false,
		}, nil
	}

	// Find the V1 federated identity by name
	for _, fedCred := range fedCreds.GetValue() {
		if fedCred.GetName() == nil || *fedCred.GetName() != fedName {
			continue
		}
		issuer := ""
		subject := ""
		if fedCred.GetIssuer() != nil {
			issuer = *fedCred.GetIssuer()
		}
		if fedCred.GetSubject() != nil {
			subject = *fedCred.GetSubject()
		}

		tflog.Info(ctx, fmt.Sprintf("[Legacy Detection] Found federated identity: %s", fedName))

		return &LegacyFedIdentity{
			DisplayName: fedName,
			Issuer:      issuer,
			Subject:     subject,
			Exists:      true,
		}, nil
	}

	tflog.Debug(ctx, fmt.Sprintf("[Legacy Detection] Federated identity not found: %s", fedName))
	return &LegacyFedIdentity{
		DisplayName: fedName,
		Exists:      false,
	}, nil
}

// DetectionTimestamp returns the current timestamp in RFC3339 format
func DetectionTimestamp() string {
	return time.Now().Format(time.RFC3339)
}
