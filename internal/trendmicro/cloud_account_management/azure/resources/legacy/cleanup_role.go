package legacy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// CustomRoleCleanupOptions defines options for custom role cleanup
type CustomRoleCleanupOptions struct{}

// CustomRoleCleanupResult contains the result of custom role cleanup
type CustomRoleCleanupResult struct {
	SubscriptionID       string
	CustomRoleName       string
	CustomRoleID         string
	Exists               bool
	Deleted              bool
	RoleAssignmentsCount int
	Error                error
	Timestamp            time.Time
}

// CleanupCustomRole deletes a V1 custom role and its assignments
func CleanupCustomRole(
	ctx context.Context,
	subscriptionID string,
	options CustomRoleCleanupOptions,
) (*CustomRoleCleanupResult, error) {
	result := &CustomRoleCleanupResult{
		SubscriptionID: subscriptionID,
		Timestamp:      time.Now(),
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Starting custom role cleanup for subscription: %s", subscriptionID))

	// Detect custom role
	role, err := DetectCustomRole(ctx, subscriptionID)
	if err != nil {
		result.Error = fmt.Errorf("failed to detect custom role: %w", err)
		return result, result.Error
	}

	result.CustomRoleName = role.Name
	result.CustomRoleID = role.ID
	result.Exists = role.Exists
	result.RoleAssignmentsCount = len(role.AssignmentIDs)

	if !role.Exists {
		tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Custom role does not exist: %s", role.Name))
		return result, nil
	}

	// Get Azure clients
	azureClient, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		result.Error = fmt.Errorf("failed to get Azure clients: %v", diags)
		return result, result.Error
	}

	// Delete role assignments first
	for _, assignmentID := range role.AssignmentIDs {
		tflog.Debug(ctx, fmt.Sprintf("[Legacy Cleanup] Deleting role assignment: %s", assignmentID))
		err = deleteRoleAssignment(ctx, azureClient, assignmentID)
		if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
			result.Error = fmt.Errorf("failed to delete role assignment %s: %w", assignmentID, err)
			return result, result.Error
		}
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Deleted %d role assignments", len(role.AssignmentIDs)))

	// Delete role definition
	tflog.Debug(ctx, fmt.Sprintf("[Legacy Cleanup] Deleting role definition: %s", role.ID))
	// Extract role definition name (GUID) from full ID
	// Format: /subscriptions/{sub}/providers/Microsoft.Authorization/roleDefinitions/{guid}
	roleDefParts := strings.Split(role.ID, "/roleDefinitions/")
	var roleDefName string
	if len(roleDefParts) == 2 {
		roleDefName = roleDefParts[1]
	} else {
		roleDefName = role.ID // Fallback to full ID if parsing fails
	}

	err = deleteRoleDefinition(ctx, azureClient, roleDefName, role.Scope)
	if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
		result.Error = fmt.Errorf("failed to delete role definition: %w", err)
		return result, result.Error
	}

	result.Deleted = true
	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Successfully deleted custom role: %s", role.Name))

	return result, nil
}

// deleteRoleAssignment deletes a role assignment by ID using REST API with 2022-04-01 version
func deleteRoleAssignment(ctx context.Context, azureClient *api.AzureClients, assignmentID string) error {
	// Extract scope and assignment name from full assignment ID
	// Format: /subscriptions/{sub}/providers/Microsoft.Authorization/roleAssignments/{guid}
	parts := strings.Split(assignmentID, "/providers/Microsoft.Authorization/roleAssignments/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid role assignment ID format: %s", assignmentID)
	}
	scope := parts[0]          // e.g., /subscriptions/{sub-id}
	assignmentName := parts[1] // e.g., guid

	// Build REST API URL with 2022-04-01 API version (supports DataActions)
	apiVersion := "2022-04-01"
	url := fmt.Sprintf("https://management.azure.com%s/providers/Microsoft.Authorization/roleAssignments/%s?api-version=%s",
		scope, assignmentName, apiVersion)

	// Get access token
	token, err := azureClient.Credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete role assignment (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// deleteRoleDefinition deletes a role definition using REST API with 2022-04-01 version
func deleteRoleDefinition(ctx context.Context, azureClient *api.AzureClients, roleDefinitionID, scope string) error {
	// Build REST API URL with 2022-04-01 API version (supports DataActions)
	apiVersion := "2022-04-01"
	url := fmt.Sprintf("https://management.azure.com%s/providers/Microsoft.Authorization/roleDefinitions/%s?api-version=%s",
		scope, roleDefinitionID, apiVersion)

	// Get access token
	token, err := azureClient.Credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete role definition (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
