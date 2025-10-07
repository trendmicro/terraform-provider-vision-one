package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	msgraph "github.com/microsoftgraph/msgraph-sdk-go"
)

type AzureClients struct {
	SubscriptionID string
	RGClient       *armresources.ResourceGroupsClient
	RoleClient     *armauthorization.RoleDefinitionsClient
	GraphClient    *msgraph.GraphServiceClient
	Credential     azidentity.AzureCLICredential
}

func GetAzureClients(ctx context.Context, subscriptionID string) (*AzureClients, diag.Diagnostics) {
	var diags diag.Diagnostics
	var subID string

	if subscriptionID == "" {
		subID = os.Getenv("ARM_SUBSCRIPTION_ID")
	} else {
		subID = subscriptionID
	}

	if subID == "" {
		diags.AddError("Missing Subscription ID", "Environment variable ARM_SUBSCRIPTION_ID is not set and no subscription_id attribute was provided.")
		return nil, diags
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		diags.AddError("Azure Credential Error", fmt.Sprintf("Failed to get credential: %s", err))
		return nil, diags
	}

	rgClient, err := armresources.NewResourceGroupsClient(subID, cred, nil)
	if err != nil {
		diags.AddError("Azure Client Error", fmt.Sprintf("Failed to create ResourceGroupsClient: %s", err))
		return nil, diags
	}

	roleClient, err := armauthorization.NewRoleDefinitionsClient(cred, nil)
	if err != nil {
		diags.AddError("Role Definition Client Error", err.Error())
		return nil, diags
	}

	graphClient, err := msgraph.NewGraphServiceClientWithCredentials(cred, []string{"https://graph.microsoft.com/.default"})
	if err != nil {
		diags.AddError("Graph Client Error", fmt.Sprintf("Failed to create GraphServiceClient: %s", err))
		return nil, diags
	}

	return &AzureClients{
		SubscriptionID: subID,
		RGClient:       rgClient,
		RoleClient:     roleClient,
		GraphClient:    graphClient,
	}, diags
}

func GetAzureCredential() (*azidentity.DefaultAzureCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure CLI credential: %w", err)
	}
	return cred, nil
}

func GetDefaultSubscription() (string, error) {
	subscriptionID := os.Getenv("ARM_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
	}
	if subscriptionID == "" {
		cliSubscriptionID, err := GetAzureCLISubscription()
		if err == nil {
			return cliSubscriptionID, nil
		}
		return "", fmt.Errorf("environment variables ARM_SUBSCRIPTION_ID and AZURE_SUBSCRIPTION_ID are not set, and Azure CLI subscription could not be retrieved: %w", err)
	}

	return subscriptionID, nil
}

func GetDefaultTenantID() (string, error) {
	tenantID := os.Getenv("ARM_TENANT_ID")
	if tenantID == "" {
		tenantID = os.Getenv("AZURE_TENANT_ID")
	}
	if tenantID == "" {
		cliTenantID, err := GetAzureCLITenant()
		if err == nil {
			return cliTenantID, nil
		}
		return "", fmt.Errorf("environment variables ARM_TENANT_ID and AZURE_TENANT_ID are not set")
	}
	return tenantID, nil
}

type azureAccount struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TenantID     string `json:"tenantId"`
	IsDefault    bool   `json:"isDefault"`
	State        string `json:"state"`
	User         User   `json:"user"`
	HomeTenantID string `json:"homeTenantId"`
}

type User struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func GetAzureCLISubscription() (string, error) {
	cmd := exec.Command("az", "account", "show", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute 'az account show': %w", err)
	}

	var account azureAccount
	if err := json.Unmarshal(output, &account); err != nil {
		return "", fmt.Errorf("failed to parse Azure CLI output: %w", err)
	}

	if account.ID == "" {
		return "", fmt.Errorf("no subscription ID found in Azure CLI session")
	}

	return account.ID, nil
}

func GetAzureCLITenant() (string, error) {
	cmd := exec.Command("az", "account", "show", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute 'az account show': %w", err)
	}

	var account azureAccount
	if err := json.Unmarshal(output, &account); err != nil {
		return "", fmt.Errorf("failed to parse Azure CLI output: %w", err)
	}

	if account.TenantID == "" {
		return "", fmt.Errorf("no tenant ID found in Azure CLI session")
	}

	return account.TenantID, nil
}
