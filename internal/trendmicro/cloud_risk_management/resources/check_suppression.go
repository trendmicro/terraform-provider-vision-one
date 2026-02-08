package resources

import (
	"context"
	"fmt"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	crm_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &checkSuppressionResource{}
	_ resource.ResourceWithConfigure   = &checkSuppressionResource{}
	_ resource.ResourceWithImportState = &checkSuppressionResource{}
)

type checkSuppressionResource struct {
	client *api.CrmClient
}
type CheckSuppressionResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	AccountID               types.String `tfsdk:"account_id"`
	Service                 types.String `tfsdk:"service"`
	RuleID                  types.String `tfsdk:"rule_id"`
	Region                  types.String `tfsdk:"region"`
	ResourceID              types.String `tfsdk:"resource_id"`
	Note                    types.String `tfsdk:"note"`
	SuppressedUntilDateTime types.String `tfsdk:"suppressed_until_date_time"`
}

func NewCheckSuppressionResource() resource.Resource {
	return &checkSuppressionResource{
		client: &api.CrmClient{},
	}
}

func (r *checkSuppressionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_check_suppression"
}

func (r *checkSuppressionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a check suppression in Vision One Cloud Risk Management. " +
			"A check suppression configures the suppressed attribute on a check. " +
			"When the Terraform resource is created, the flag is set. " +
			"When the Terraform resource is destroyed, the flag is removed. ",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the check suppression. This is automatically generated.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"account_id": schema.StringAttribute{
				Description: "The Vision One Cloud Risk Management internal account ID for which the check should be suppressed. " +
					"This can be retrieved using the visionone_crm_account_id data source.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service": schema.StringAttribute{
				Description: "The service name for the check. " +
					"Example: 'AutoScaling', 'EC2', 'CloudFormation'",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"rule_id": schema.StringAttribute{
				Description: "ID of the rule to be suppressed. Example: 'EC2-074'",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				Description: "The region to which the check applies. Either a specific region (e.g., 'ap-south-1') or 'global'",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_id": schema.StringAttribute{
				Description: "The ID of the resource if the check should be suppressed only for a specific resource. " +
					"Example: 'sg-061c4319bdc0646a3' or '/subscriptions/8dfbsdfe-we13-46we-9963-188868997f40/resourceGroups/myDevResources/providers/Microsoft.KeyVault/vaults/myDevKeyVault-eastus'",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"note": schema.StringAttribute{
				Description: "Explains why the given check has been suppressed",
				Required:    true,
			},
			"suppressed_until_date_time": schema.StringAttribute{
				Description: "The date and time until which the check will be suppressed. " +
					"Must be in ISO 8601 format with UTC timezone (e.g., '2026-12-31T23:59:59Z'). " +
					"If not specified, the check will be suppressed indefinitely.",
				Optional: true,
			},
		},
	}
}

func (r *checkSuppressionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = &api.CrmClient{Client: client}
}

func (r *checkSuppressionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CheckSuppressionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Construct check ID
	// Format: ccc:{accountId}:{ruleId}:{service}:{region}:{resourceId}
	checkID := fmt.Sprintf("ccc:%s:%s:%s:%s:%s",
		plan.AccountID.ValueString(),
		plan.RuleID.ValueString(),
		plan.Service.ValueString(),
		plan.Region.ValueString(),
		plan.ResourceID.ValueString(),
	)

	tflog.Debug(ctx, "Constructed check ID", map[string]interface{}{
		"check_id": checkID,
	})

	// Update check to suppress it
	updateReq := &crm_dto.UpdateCheckRequest{
		Suppressed: true,
		Note:       plan.Note.ValueString(),
	}

	// Add suppressedUntilDateTime if specified
	if !plan.SuppressedUntilDateTime.IsNull() && !plan.SuppressedUntilDateTime.IsUnknown() {
		updateReq.SuppressedUntilDateTime = plan.SuppressedUntilDateTime.ValueString()
	}

	// API returns 204 No Content on success
	err := r.client.UpdateCheck(checkID, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error suppressing check",
			fmt.Sprintf("Could not suppress check %s: %s", checkID, err.Error()),
		)
		return
	}

	// Set the ID in state from the constructed checkID
	plan.ID = types.StringValue(checkID)

	tflog.Debug(ctx, "Check suppressed successfully", map[string]interface{}{
		"check_id": checkID,
	})

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *checkSuppressionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CheckSuppressionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current check details to verify suppression status
	checkResp, err := r.client.GetCheck(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading check",
			fmt.Sprintf("Could not read check %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If the check is no longer suppressed, remove from state
	if !checkResp.Suppressed {
		tflog.Debug(ctx, "Check is no longer suppressed, removing from state", map[string]interface{}{
			"check_id": state.ID.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Populate state from API response
	state.AccountID = types.StringValue(checkResp.AccountID)
	state.Service = types.StringValue(checkResp.Service)
	state.RuleID = types.StringValue(checkResp.RuleID)
	state.Region = types.StringValue(checkResp.Region)
	state.ResourceID = types.StringValue(checkResp.ResourceID)
	state.Note = types.StringValue(checkResp.Note)

	if checkResp.SuppressedUntilDateTime != "" {
		state.SuppressedUntilDateTime = types.StringValue(checkResp.SuppressedUntilDateTime)
	} else {
		state.SuppressedUntilDateTime = types.StringNull()
	}

	tflog.Debug(ctx, "Check suppression verified", map[string]interface{}{
		"check_id":                   state.ID.ValueString(),
		"suppressed":                 checkResp.Suppressed,
		"note":                       checkResp.Note,
		"suppressed_until_date_time": checkResp.SuppressedUntilDateTime,
	})

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update handles updates to note and suppressed_until_date_time fields
func (r *checkSuppressionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state CheckSuppressionResourceModel

	// Read Terraform plan and current state
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Construct update request
	updateReq := &crm_dto.UpdateCheckRequest{
		Suppressed: true, // Resource existence means it's suppressed
		Note:       plan.Note.ValueString(),
	}

	// Add suppressedUntilDateTime if specified
	if !plan.SuppressedUntilDateTime.IsNull() && !plan.SuppressedUntilDateTime.IsUnknown() {
		updateReq.SuppressedUntilDateTime = plan.SuppressedUntilDateTime.ValueString()
	}

	// Update the check via API
	err := r.client.UpdateCheck(state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating check suppression",
			fmt.Sprintf("Could not update check suppression %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Check suppression updated successfully", map[string]interface{}{
		"check_id":                   state.ID.ValueString(),
		"note":                       plan.Note.ValueString(),
		"suppressed_until_date_time": plan.SuppressedUntilDateTime.ValueString(),
	})

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *checkSuppressionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CheckSuppressionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Unsuppress the check by setting suppressed to false
	updateReq := &crm_dto.UpdateCheckRequest{
		Suppressed: false,
		Note:       "Re-enabled as suppression has been deleted in Terraform",
	}

	// API returns 204 No Content on success
	err := r.client.UpdateCheck(state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error unsuppressing check",
			fmt.Sprintf("Could not unsuppress check %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Check unsuppressed successfully", map[string]interface{}{
		"check_id": state.ID.ValueString(),
	})
}

func (r *checkSuppressionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
