package resources

import (
	"context"
	"errors"
	"fmt"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/utils"
	"terraform-provider-vision-one/pkg/dto"
	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &profileResource{}
	_ resource.ResourceWithConfigure   = &profileResource{}
	_ resource.ResourceWithImportState = &profileResource{}
)

// NewProfileResource is a helper function to simplify the provider implementation.
func NewProfileResource() resource.Resource {
	return &profileResource{
		client: &api.CrmClient{},
	}
}

// profileResource is the resource implementation.
type profileResource struct {
	client *api.CrmClient
}

// ProfileResourceModel represents the Terraform resource model for a CRM profile.
type ProfileResourceModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	Description types.String          `tfsdk:"description"`
	ScanRules   []utils.ScanRuleModel `tfsdk:"scan_rule"`
}

// Metadata returns the resource type name.
func (r *profileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_profile"
}

// Schema defines the schema for the resource.
func (r *profileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloud Risk Management profile with rule settings.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique ID of the profile. If provided, the resource will update the existing profile instead of creating a new one.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the profile.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the profile.",
				Optional:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"scan_rule": schema.SetNestedBlock{
				MarkdownDescription: "List of scan rule configurations.",
				NestedObject: schema.NestedBlockObject{
					Attributes: utils.ScanRuleBaseAttributes(),
					Blocks: map[string]schema.Block{
						"exceptions":     utils.ExceptionsSchemaBlock(),
						"extra_settings": utils.ExtraSettingsSchemaBlock(),
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *profileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = api.NewCrmClient(client.HostURL, client.BearerToken, client.ProviderVersion)
}

// Create creates the resource and sets the initial Terraform state.
func (r *profileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ProfileResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Create new Profile plan: %+v", plan))
	createReq := cloud_risk_management_dto.CreateProfileRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}

	if len(plan.ScanRules) > 0 {
		scanRules, err := utils.ConvertScanRulesToDTO(plan.ScanRules)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Create Profile",
				"An error occurred converting scan rules: "+err.Error(),
			)
			return
		}
		createReq.ScanRules = scanRules
	}

	tflog.Debug(ctx, fmt.Sprintf("Create new Profile request: %+v", createReq))

	apiResponse, err := r.client.CreateProfile(createReq)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Create Profile",
			"An unexpected error occurred when creating the profile. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	// The API returns 201 with empty body, we need to get the profile to get the ID
	// For now, we'll use the name to find it or wait for API to return ID
	if apiResponse.ID != "" {
		plan.ID = types.StringValue(apiResponse.ID)
	}

	// Read back to get full state
	if !plan.ID.IsNull() && plan.ID.ValueString() != "" {
		r.readProfileAndUpdatePlan(ctx, &plan, &resp.Diagnostics, &resp.State)
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *profileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ProfileResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	profile, err := r.client.GetProfile(state.ID.ValueString())
	if errors.Is(err, dto.ErrorNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Read Profile",
			"An unexpected error occurred when reading the profile. "+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	updatePlanFromProfile(&state, profile)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update the resource and sets the updated Terraform state on success.
func (r *profileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ProfileResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := cloud_risk_management_dto.UpdateProfileRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}

	if len(plan.ScanRules) > 0 {
		scanRules, err := utils.ConvertScanRulesToDTO(plan.ScanRules)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Update Profile",
				"An error occurred converting scan rules: "+err.Error(),
			)
			return
		}
		updateReq.ScanRules = scanRules
	}

	err := r.client.UpdateProfile(plan.ID.ValueString(), updateReq)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Update Profile",
			"An unexpected error occurred when updating the profile. "+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	// Read back to get updated state
	r.readProfileAndUpdatePlan(ctx, &plan, &resp.Diagnostics, &resp.State)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *profileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ProfileResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteProfile(state.ID.ValueString())
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Delete Profile",
			"An unexpected error occurred when deleting the profile. "+
				"TrendMicro Client: "+err.Error(),
		)
	}
}

// ImportState imports the resource state.
func (r *profileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// =============================================================================
// Helper Functions
// =============================================================================

// stateSetter is an interface for setting state in resource responses.
type stateSetter interface {
	Set(context.Context, any) diag.Diagnostics
}

// readProfileAndUpdatePlan reads the profile from the API, updates the plan/state model, and sets the state.
func (r *profileResource) readProfileAndUpdatePlan(ctx context.Context, plan *ProfileResourceModel, diagnostics *diag.Diagnostics, state stateSetter) {
	profile, err := r.client.GetProfile(plan.ID.ValueString())
	if err != nil {
		tflog.Debug(ctx, err.Error())
		diagnostics.AddError(
			"Unable to Read Profile",
			"An unexpected error occurred when reading the profile. "+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Plan BEFORE updatePlanFromProfile: %+v", plan))
	tflog.Debug(ctx, fmt.Sprintf("Profile from API: %+v", profile))

	updatePlanFromProfile(plan, profile)

	tflog.Debug(ctx, fmt.Sprintf("Plan AFTER updatePlanFromProfile: %+v", plan))

	diags := state.Set(ctx, plan)
	diagnostics.Append(diags...)
}

// =============================================================================
// Model Converters
// =============================================================================

// updatePlanFromProfile updates the Terraform plan/state model with data from the API response.
func updatePlanFromProfile(plan *ProfileResourceModel, profile *cloud_risk_management_dto.Profile) {
	// Create a map of original extra_settings by rule ID -> setting name
	originalExtraSettings := make(map[string]map[string]*utils.ExtraSettingModel)
	for _, rule := range plan.ScanRules {
		if len(rule.ExtraSettings) > 0 {
			ruleID := rule.ID.ValueString()
			originalExtraSettings[ruleID] = make(map[string]*utils.ExtraSettingModel)
			for i := range rule.ExtraSettings {
				settingName := rule.ExtraSettings[i].Name.ValueString()
				originalExtraSettings[ruleID][settingName] = &rule.ExtraSettings[i]
			}
		}
	}

	plan.ID = types.StringValue(profile.ID)
	plan.Name = types.StringValue(profile.Name)
	plan.Description = types.StringValue(profile.Description)

	// Convert scan rules back
	if len(profile.ScanRules) > 0 {
		plan.ScanRules = make([]utils.ScanRuleModel, len(profile.ScanRules))
		for i, rule := range profile.ScanRules {
			plan.ScanRules[i] = utils.ScanRuleModel{
				ID:        types.StringValue(rule.ID),
				Provider:  types.StringValue(rule.Provider),
				Enabled:   types.BoolValue(rule.Enabled),
				RiskLevel: types.StringValue(rule.RiskLevel),
			}

			// Convert exceptions from API response
			plan.ScanRules[i].Exceptions = utils.ConvertExceptionsFromDTO(rule.Exceptions)

			// Convert extra settings - always convert to match plan structure
			if len(rule.ExtraSettings) > 0 {
				plan.ScanRules[i].ExtraSettings = utils.ConvertExtraSettingsFromDTO(rule.ExtraSettings, originalExtraSettings[rule.ID])
			} else {
				// Ensure ExtraSettings is an empty slice (not nil) to match schema
				plan.ScanRules[i].ExtraSettings = []utils.ExtraSettingModel{}
			}
		}
	}
}
