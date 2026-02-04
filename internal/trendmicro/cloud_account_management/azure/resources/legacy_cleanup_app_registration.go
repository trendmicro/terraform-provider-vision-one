package azure

import (
	"context"
	"fmt"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/legacy"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type legacyCleanupAppRegistration struct {
	client *trendmicro.Client
}

type legacyCleanupAppRegistrationModel struct {
	ID             types.String `tfsdk:"id"`
	SubscriptionID types.String `tfsdk:"subscription_id"`

	// Optional: For validation
	AppRegistrationObjectID types.String `tfsdk:"app_registration_object_id"`

	// Computed outputs
	Deleted                  types.Bool   `tfsdk:"deleted"`
	ServicePrincipalDeleted  types.Bool   `tfsdk:"service_principal_deleted"`
	FederatedIdentityDeleted types.Bool   `tfsdk:"federated_identity_deleted"`
	DeletionTimestamp        types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus            types.String `tfsdk:"cleanup_status"`
	CleanupError             types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupAppRegistration() resource.Resource {
	return &legacyCleanupAppRegistration{}
}

func (r *legacyCleanupAppRegistration) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_APP_REGISTRATION
}

func (r *legacyCleanupAppRegistration) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deletes legacy App Registration created by CAM Ver1 deployments. The resource automatically detects whether a legacy App Registration exists before attempting cleanup. Deleting the App Registration automatically cascades to delete the associated Service Principal and Federated Identity Credentials. Returns `cleanup_status = \"not_found\"` if no legacy resources exist.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this cleanup resource (subscription ID)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "Azure subscription ID containing the legacy App Registration to delete",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"app_registration_object_id": schema.StringAttribute{
				MarkdownDescription: "Object ID of the App Registration for validation (optional). If provided, cleanup will fail if the detected App Registration doesn't match this ID.",
				Optional:            true,
			},
			"deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether the App Registration was successfully deleted",
				Computed:            true,
			},
			"service_principal_deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether the Service Principal was deleted (cascaded from App Registration deletion)",
				Computed:            true,
			},
			"federated_identity_deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether the Federated Identity was deleted (cascaded from App Registration deletion)",
				Computed:            true,
			},
			"deletion_timestamp": schema.StringAttribute{
				MarkdownDescription: "Timestamp when deletion was performed (RFC3339 format)",
				Computed:            true,
			},
			"cleanup_status": schema.StringAttribute{
				MarkdownDescription: "Status of cleanup operation: deleted, not_found, or failed",
				Computed:            true,
			},
			"cleanup_error": schema.StringAttribute{
				MarkdownDescription: "Error message if cleanup failed",
				Computed:            true,
			},
		},
	}
}

func (r *legacyCleanupAppRegistration) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupAppRegistrationModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subscriptionID := plan.SubscriptionID.ValueString()
	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup App Registration] Deleting legacy app registration for subscription: %s", subscriptionID))

	// Optional validation: If user provided an expected object ID, verify it matches
	if !plan.AppRegistrationObjectID.IsNull() && !plan.AppRegistrationObjectID.IsUnknown() {
		expectedObjectID := plan.AppRegistrationObjectID.ValueString()

		// Detect current app registration
		detected, err := legacy.DetectAppRegistration(ctx, subscriptionID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Detection Failed",
				fmt.Sprintf("Failed to detect app registration before cleanup: %s", err.Error()),
			)
			return
		}

		if detected.Exists && detected.ObjectID != expectedObjectID {
			resp.Diagnostics.AddError(
				"Validation Failed",
				fmt.Sprintf("App Registration Object ID mismatch. Expected: %s, Found: %s", expectedObjectID, detected.ObjectID),
			)
			return
		}
	}

	// Execute cleanup
	result, err := legacy.CleanupAppRegistration(ctx, subscriptionID, legacy.AppRegistrationCleanupOptions{})

	// Populate state
	plan.ID = types.StringValue(subscriptionID)
	plan.Deleted = types.BoolValue(result.Deleted)
	plan.ServicePrincipalDeleted = types.BoolValue(result.ServicePrincipalDeleted)
	plan.FederatedIdentityDeleted = types.BoolValue(result.FederatedIdentityDeleted)
	plan.DeletionTimestamp = types.StringValue(legacy.DetectionTimestamp())

	// Handle errors
	if err != nil || result.Error != nil {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else if result.Error != nil {
			errMsg = result.Error.Error()
		}
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(errMsg)
		resp.Diagnostics.AddError("[Legacy Cleanup App Registration] Cleanup failed", errMsg)
	} else {
		plan.CleanupError = types.StringNull()
		if result.Deleted {
			plan.CleanupStatus = types.StringValue("deleted")
		} else {
			plan.CleanupStatus = types.StringValue("not_found")
		}
	}

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *legacyCleanupAppRegistration) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupAppRegistrationModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()

	// Re-detect the app registration to check if it was recreated
	appReg, err := legacy.DetectAppRegistration(ctx, subscriptionID)
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Detection Warning",
			fmt.Sprintf("Failed to re-detect app registration during refresh: %s", err.Error()),
		)
		// Keep existing state
		return
	}

	// If resource exists after being deleted, update status to indicate recreation
	if appReg.Exists && state.Deleted.ValueBool() {
		tflog.Warn(ctx, fmt.Sprintf("[Legacy Cleanup App Registration] App registration was recreated after deletion: %s", subscriptionID))
		state.CleanupStatus = types.StringValue("recreated")
	}

	if diags := resp.State.Set(ctx, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *legacyCleanupAppRegistration) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Cleanup resources don't support updates - trigger recreation
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Legacy cleanup resources cannot be updated. Any changes will trigger resource replacement.",
	)
}

func (r *legacyCleanupAppRegistration) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Cleanup resources don't actually delete anything on terraform destroy
	tflog.Info(ctx, "[Legacy Cleanup App Registration] Resource removed from Terraform state (Azure resources unchanged)")
}

func (r *legacyCleanupAppRegistration) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *trendmicro.Client",
		)
		return
	}

	r.client = client
	tflog.Debug(ctx, "[Legacy Cleanup App Registration] Resource configured successfully")
}
