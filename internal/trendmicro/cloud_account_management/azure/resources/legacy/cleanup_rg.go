package legacy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ResourceGroupCleanupOptions defines options for resource group cleanup
type ResourceGroupCleanupOptions struct {
	PreserveStateStorage bool // Archive instead of delete
	ForceDelete          bool // Delete even if state files exist
}

// ResourceGroupCleanupResult contains the result of resource group cleanup
type ResourceGroupCleanupResult struct {
	SubscriptionID     string
	ResourceGroupName  string
	Exists             bool
	Deleted            bool
	Archived           bool
	StorageAccountName string
	StateFileExists    bool
	Error              error
	Timestamp          time.Time
}

// CleanupResourceGroup deletes or archives a V1 resource group
func CleanupResourceGroup(
	ctx context.Context,
	subscriptionID string,
	options ResourceGroupCleanupOptions,
) (*ResourceGroupCleanupResult, error) {
	result := &ResourceGroupCleanupResult{
		SubscriptionID: subscriptionID,
		Timestamp:      time.Now(),
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Starting resource group cleanup for subscription: %s", subscriptionID))

	// Detect resource group
	rg, err := DetectResourceGroup(ctx, subscriptionID)
	if err != nil {
		result.Error = fmt.Errorf("failed to detect resource group: %w", err)
		return result, result.Error
	}

	result.ResourceGroupName = rg.Name
	result.Exists = rg.Exists

	if !rg.Exists {
		tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Resource group does not exist: %s", rg.Name))
		return result, nil
	}

	// Detect storage account
	sa, err := DetectStorageAccount(ctx, subscriptionID, rg.Name)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[Legacy Cleanup] Failed to detect storage account: %s", err))
	} else if sa != nil {
		result.StorageAccountName = sa.Name
		result.StateFileExists = sa.StateFileExists
	}

	// Check if we should skip deletion
	if result.StateFileExists && !options.ForceDelete && !options.PreserveStateStorage {
		result.Error = fmt.Errorf("state file exists in storage account, use force_delete=true or preserve_state_storage=true")
		return result, result.Error
	}

	// Get Azure clients
	azureClient, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		result.Error = fmt.Errorf("failed to get Azure clients: %v", diags)
		return result, result.Error
	}

	if options.PreserveStateStorage {
		// Archive mode: Tag resource group instead of deleting
		tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Archiving resource group: %s", rg.Name))
		err = archiveResourceGroup(ctx, azureClient, rg.Name)
		if err != nil {
			result.Error = fmt.Errorf("failed to archive resource group: %w", err)
			return result, result.Error
		}
		result.Archived = true
		tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Successfully archived resource group: %s", rg.Name))
	} else {
		// Delete mode
		tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Deleting resource group: %s", rg.Name))
		err = deleteResourceGroup(ctx, azureClient, rg.Name)
		if err != nil && !strings.Contains(err.Error(), "NotFound") {
			result.Error = fmt.Errorf("failed to delete resource group: %w", err)
			return result, result.Error
		}
		result.Deleted = true
		tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Successfully deleted resource group: %s", rg.Name))
	}

	return result, nil
}

// archiveResourceGroup archives a resource group by adding tags
func archiveResourceGroup(ctx context.Context, azureClient *api.AzureClients, rgName string) error {
	// Get existing resource group
	rg, err := azureClient.RGClient.Get(ctx, rgName, nil)
	if err != nil {
		return fmt.Errorf("failed to get resource group: %w", err)
	}

	// Add archive tags
	tags := rg.Tags
	if tags == nil {
		tags = make(map[string]*string)
	}

	archivedTag := "true"
	archivedTime := time.Now().Format(time.RFC3339)
	tags["V1Archived"] = &archivedTag
	tags["V1ArchivedAt"] = &archivedTime

	// Update resource group with tags
	rg.Tags = tags
	_, err = azureClient.RGClient.Update(ctx, rgName, armresources.ResourceGroupPatchable{
		Tags: tags,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to update resource group tags: %w", err)
	}

	return nil
}

// deleteResourceGroup deletes a resource group
func deleteResourceGroup(ctx context.Context, azureClient *api.AzureClients, rgName string) error {
	poller, err := azureClient.RGClient.BeginDelete(ctx, rgName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin resource group deletion: %w", err)
	}

	// Wait for deletion to complete
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete resource group deletion: %w", err)
	}

	return nil
}
