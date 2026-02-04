package legacy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// AppRegistrationCleanupOptions defines options for app registration cleanup
type AppRegistrationCleanupOptions struct{}

// AppRegistrationCleanupResult contains the result of app registration cleanup
type AppRegistrationCleanupResult struct {
	SubscriptionID           string
	AppRegistrationName      string
	AppRegistrationObjectID  string
	AppRegistrationClientID  string
	Exists                   bool
	Deleted                  bool
	ServicePrincipalDeleted  bool
	FederatedIdentityDeleted bool
	Error                    error
	Timestamp                time.Time
}

// CleanupAppRegistration deletes a V1 App Registration (cascades to SP and Federated Identity)
func CleanupAppRegistration(
	ctx context.Context,
	subscriptionID string,
	options AppRegistrationCleanupOptions,
) (*AppRegistrationCleanupResult, error) {
	result := &AppRegistrationCleanupResult{
		SubscriptionID: subscriptionID,
		Timestamp:      time.Now(),
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Starting app registration cleanup for subscription: %s", subscriptionID))

	// Detect app registration
	appReg, err := DetectAppRegistration(ctx, subscriptionID)
	if err != nil {
		result.Error = fmt.Errorf("failed to detect app registration: %w", err)
		return result, result.Error
	}

	result.AppRegistrationName = appReg.DisplayName
	result.AppRegistrationObjectID = appReg.ObjectID
	result.AppRegistrationClientID = appReg.ClientID
	result.Exists = appReg.Exists

	if !appReg.Exists {
		tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] App registration does not exist: %s", appReg.DisplayName))
		return result, nil
	}

	// Get Graph client for Microsoft Graph API operations
	graphClient, _, diags := api.GetGraphClient(ctx)
	if diags.HasError() {
		result.Error = fmt.Errorf("failed to get Graph client: %v", diags)
		return result, result.Error
	}

	// Delete App Registration (this cascades to Service Principal and Federated Identity)
	tflog.Debug(ctx, fmt.Sprintf("[Legacy Cleanup] Deleting app registration: %s", appReg.ObjectID))
	delErr := graphClient.Applications().ByApplicationId(appReg.ObjectID).Delete(ctx, nil)
	if delErr != nil && !strings.Contains(delErr.Error(), "not found") && !strings.Contains(delErr.Error(), "NotFound") {
		result.Error = fmt.Errorf("failed to delete app registration: %w", delErr)
		return result, result.Error
	}

	result.Deleted = true
	result.ServicePrincipalDeleted = true  // Cascaded by Azure
	result.FederatedIdentityDeleted = true // Cascaded by Azure

	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup] Successfully deleted app registration: %s (Service Principal and Federated Identity cascaded)", appReg.DisplayName))

	return result, nil
}
