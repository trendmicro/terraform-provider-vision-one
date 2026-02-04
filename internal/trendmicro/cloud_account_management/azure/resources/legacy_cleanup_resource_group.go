package azure

import (
	"context"
	"fmt"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/legacy"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type legacyCleanupResourceGroup struct {
	client *trendmicro.Client
}

type legacyCleanupResourceGroupModel struct {
	ID                   types.String `tfsdk:"id"`
	SubscriptionID       types.String `tfsdk:"subscription_id"`
	PreserveStateStorage types.Bool   `tfsdk:"preserve_state_storage"`
	ForceDelete          types.Bool   `tfsdk:"force_delete"`

	// Computed outputs
	Deleted           types.Bool   `tfsdk:"deleted"`
	Archived          types.Bool   `tfsdk:"archived"`
	DeletionTimestamp types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus     types.String `tfsdk:"cleanup_status"`
	CleanupError      types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupResourceGroup() resource.Resource {
	return &legacyCleanupResourceGroup{}
}

func (r *legacyCleanupResourceGroup) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_RESOURCE_GROUP
}

func (r *legacyCleanupResourceGroup) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deletes or archives legacy Resource Group created by CAM Ver1 deployments. The resource automatically detects whether a legacy Resource Group exists before attempting cleanup. Supports archive mode (tagging instead of deletion) to preserve Terraform state storage. Returns `cleanup_status = \"not_found\"` if no legacy resources exist.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this cleanup resource (subscription ID)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "Azure subscription ID containing the legacy Resource Group to delete",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"preserve_state_storage": schema.BoolAttribute{
				MarkdownDescription: "If true, archive the resource group with tags instead of deleting (preserves Terraform state storage). Default: true",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"force_delete": schema.BoolAttribute{
				MarkdownDescription: "If true, delete resource group even if state files exist (ignored if preserve_state_storage is true). Default: false",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether the Resource Group was successfully deleted",
				Computed:            true,
			},
			"archived": schema.BoolAttribute{
				MarkdownDescription: "Whether the Resource Group was archived (tagged) instead of deleted",
				Computed:            true,
			},
			"deletion_timestamp": schema.StringAttribute{
				MarkdownDescription: "Timestamp when deletion/archiving was performed (RFC3339 format)",
				Computed:            true,
			},
			"cleanup_status": schema.StringAttribute{
				MarkdownDescription: "Status of cleanup operation: deleted, archived, not_found, or failed",
				Computed:            true,
			},
			"cleanup_error": schema.StringAttribute{
				MarkdownDescription: "Error message if cleanup failed",
				Computed:            true,
			},
		},
	}
}

func (r *legacyCleanupResourceGroup) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupResourceGroupModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subscriptionID := plan.SubscriptionID.ValueString()
	preserveStateStorage := plan.PreserveStateStorage.ValueBool()
	forceDelete := plan.ForceDelete.ValueBool()

	tflog.Info(ctx, fmt.Sprintf("[Legacy Cleanup Resource Group] Starting cleanup for subscription: %s, preserve_state_storage: %v, force_delete: %v",
		subscriptionID, preserveStateStorage, forceDelete))

	// Execute cleanup
	result, err := legacy.CleanupResourceGroup(ctx, subscriptionID, legacy.ResourceGroupCleanupOptions{
		PreserveStateStorage: preserveStateStorage,
		ForceDelete:          forceDelete,
	})

	// Populate state
	plan.ID = types.StringValue(subscriptionID)
	plan.Deleted = types.BoolValue(result.Deleted)
	plan.Archived = types.BoolValue(result.Archived)
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
		resp.Diagnostics.AddError("[Legacy Cleanup Resource Group] Cleanup failed", errMsg)
	} else {
		plan.CleanupError = types.StringNull()
		if result.Deleted {
			plan.CleanupStatus = types.StringValue("deleted")
		} else if result.Archived {
			plan.CleanupStatus = types.StringValue("archived")
		} else {
			plan.CleanupStatus = types.StringValue("not_found")
		}
	}

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *legacyCleanupResourceGroup) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupResourceGroupModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()

	// Re-detect the resource group to check if it was recreated
	rg, err := legacy.DetectResourceGroup(ctx, subscriptionID)
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Detection Warning",
			fmt.Sprintf("Failed to re-detect resource group during refresh: %s", err.Error()),
		)
		// Keep existing state
		return
	}

	// If resource exists after being deleted/archived, update status to indicate recreation
	if rg.Exists && (state.Deleted.ValueBool() || state.Archived.ValueBool()) {
		tflog.Warn(ctx, fmt.Sprintf("[Legacy Cleanup Resource Group] Resource group was recreated after cleanup: %s", subscriptionID))
		state.CleanupStatus = types.StringValue("recreated")
	}

	if diags := resp.State.Set(ctx, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *legacyCleanupResourceGroup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Cleanup resources don't support updates - trigger recreation
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"V1 cleanup resources cannot be updated. Any changes will trigger resource replacement.",
	)
}

func (r *legacyCleanupResourceGroup) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Cleanup resources don't actually delete anything on terraform destroy
	tflog.Info(ctx, "[Legacy Cleanup Resource Group] Resource removed from Terraform state (Azure resources unchanged)")
}

func (r *legacyCleanupResourceGroup) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	tflog.Debug(ctx, "[Legacy Cleanup Resource Group] Resource configured successfully")
}
