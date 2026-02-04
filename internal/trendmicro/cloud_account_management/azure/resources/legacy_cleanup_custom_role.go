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

type legacyCleanupCustomRole struct {
	client *trendmicro.Client
}

type legacyCleanupCustomRoleModel struct {
	ID             types.String `tfsdk:"id"`
	SubscriptionID types.String `tfsdk:"subscription_id"`

	// Optional: For validation
	CustomRoleID types.String `tfsdk:"custom_role_id"`

	// Computed outputs
	Deleted              types.Bool   `tfsdk:"deleted"`
	RoleAssignmentsCount types.Int64  `tfsdk:"role_assignments_count"`
	DeletionTimestamp    types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus        types.String `tfsdk:"cleanup_status"`
	CleanupError         types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupCustomRole() resource.Resource {
	return &legacyCleanupCustomRole{}
}

func (r *legacyCleanupCustomRole) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_CUSTOM_ROLE
}

func (r *legacyCleanupCustomRole) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deletes legacy Custom Role Definition created by CAM Ver1 deployments. The resource automatically detects whether a legacy Custom Role exists before attempting cleanup. Also deletes all associated role assignments. Returns `cleanup_status = \"not_found\"` if no legacy resources exist.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this cleanup resource (subscription ID)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "Azure subscription ID containing the legacy Custom Role to delete",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"custom_role_id": schema.StringAttribute{
				MarkdownDescription: "Full Azure resource ID of the Custom Role for validation (optional). If provided, cleanup will fail if the detected Custom Role doesn't match this ID.",
				Optional:            true,
			},
			"deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether the Custom Role was successfully deleted",
				Computed:            true,
			},
			"role_assignments_count": schema.Int64Attribute{
				MarkdownDescription: "Number of role assignments that were deleted",
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

func (r *legacyCleanupCustomRole) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupCustomRoleModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subscriptionID := plan.SubscriptionID.ValueString()
	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup Custom Role] Deleting legacy custom role for subscription: %s", subscriptionID))

	// Optional validation: If user provided an expected role ID, verify it matches
	if !plan.CustomRoleID.IsNull() && !plan.CustomRoleID.IsUnknown() {
		expectedRoleID := plan.CustomRoleID.ValueString()

		// Detect current custom role
		detected, err := legacy.DetectCustomRole(ctx, subscriptionID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Detection Failed",
				fmt.Sprintf("Failed to detect custom role before cleanup: %s", err.Error()),
			)
			return
		}

		if detected.Exists && detected.ID != expectedRoleID {
			resp.Diagnostics.AddError(
				"Validation Failed",
				fmt.Sprintf("Custom Role ID mismatch. Expected: %s, Found: %s", expectedRoleID, detected.ID),
			)
			return
		}
	}

	// Execute cleanup
	result, err := legacy.CleanupCustomRole(ctx, subscriptionID, legacy.CustomRoleCleanupOptions{})

	// Populate state
	plan.ID = types.StringValue(subscriptionID)
	plan.Deleted = types.BoolValue(result.Deleted)
	plan.RoleAssignmentsCount = types.Int64Value(int64(result.RoleAssignmentsCount))
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
		resp.Diagnostics.AddError("[Legacy Cleanup Custom Role] Cleanup failed", errMsg)
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

func (r *legacyCleanupCustomRole) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupCustomRoleModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()

	// Re-detect the custom role to check if it was recreated
	role, err := legacy.DetectCustomRole(ctx, subscriptionID)
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Detection Warning",
			fmt.Sprintf("Failed to re-detect custom role during refresh: %s", err.Error()),
		)
		// Keep existing state
		return
	}

	// If resource exists after being deleted, update status to indicate recreation
	if role.Exists && state.Deleted.ValueBool() {
		tflog.Warn(ctx, fmt.Sprintf("[Legacy Cleanup Custom Role] Custom role was recreated after deletion: %s", subscriptionID))
		state.CleanupStatus = types.StringValue("recreated")
	}

	if diags := resp.State.Set(ctx, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *legacyCleanupCustomRole) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Cleanup resources don't support updates - trigger recreation
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Legacy cleanup resources cannot be updated. Any changes will trigger resource replacement.",
	)
}

func (r *legacyCleanupCustomRole) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Cleanup resources don't actually delete anything on terraform destroy
	// They only delete Azure resources during terraform apply
	tflog.Info(ctx, "[Legacy Cleanup Custom Role] Resource removed from Terraform state (Azure resources unchanged)")
}

func (r *legacyCleanupCustomRole) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	tflog.Debug(ctx, "[Legacy Cleanup Custom Role] Resource configured successfully")
}
