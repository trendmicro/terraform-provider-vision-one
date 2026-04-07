package resources

import (
	"context"
	"fmt"
	"time"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                   = &reportConfigResource{}
	_ resource.ResourceWithConfigure      = &reportConfigResource{}
	_ resource.ResourceWithImportState    = &reportConfigResource{}
	_ resource.ResourceWithValidateConfig = &reportConfigResource{}
)

func NewReportConfigResource() resource.Resource {
	return &reportConfigResource{
		client: &api.CrmClient{},
	}
}

type reportConfigResource struct {
	client *api.CrmClient
}

// ReportConfigResourceModel uses the model from utils package
type ReportConfigResourceModel = utils.ReportConfigResourceModel
type ScheduleModel = utils.ScheduleModel
type FilterModel = utils.FilterModel

// Metadata returns the resource type name.
func (r *reportConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_report_config"
}

// Schema defines the schema for the resource.
func (r *reportConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloud Risk Management report configuration for scheduled or on-demand compliance reports.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique ID of the report configuration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The Cloud Risk Management account ID to generate reports for. Omit both account_id and group_id for company-level reports. Cannot specify both account_id and group_id together.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"group_id": schema.StringAttribute{
				MarkdownDescription: "The Cloud Risk Management group ID to generate reports for. Omit both account_id and group_id for company-level reports. Cannot specify both account_id and group_id together.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"level": schema.StringAttribute{
				MarkdownDescription: "The level of the report (account, group, or company). This is computed based on whether account_id or group_id is specified.",
				Computed:            true,
			},
			"report_title": schema.StringAttribute{
				MarkdownDescription: "The title of the report.",
				Required:            true,
			},
			"report_type": schema.StringAttribute{
				MarkdownDescription: "Type of report to generate. Allowed values: GENERIC, COMPLIANCE-STANDARD.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("GENERIC", "COMPLIANCE-STANDARD"),
				},
			},
			"include_checks": schema.BoolAttribute{
				MarkdownDescription: "Whether to include individual checks in PDF reports. Default: false. Note: If the total number of checks exceeds 10,000, not all checks are included.",
				Optional:            true,
				Computed:            true,
			},
			"include_account_names": schema.BoolAttribute{
				MarkdownDescription: "Whether to include cloud account names in PDF reports. Only available for group-level and company-level reports. Cannot be used when account_id is provided.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					nullIfAccountLevelPlanModifier{},
					nullIfNotInConfigPlanModifier{},
				},
			},
			"email_recipients": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of email addresses to send the report to. Defaults to empty list if not specified.",
				Optional:            true,
				Computed:            true,
			},
			"report_formats_in_email": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "The format of emailed reports. Allowed values: PDF, CSV, all. Default: [\"all\"].",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf("PDF", "CSV", "all")),
				},
			},
			"language": schema.StringAttribute{
				MarkdownDescription: "The language for the report. Allowed values: en (English), ja (Japanese). Defaults to 'en' if not specified.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("en", "ja"),
				},
			},
			"applied_compliance_standard_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the compliance standard to apply (e.g., 'NIST4', 'AWAF-2025'). Required when report_type is COMPLIANCE-STANDARD.",
				Optional:            true,
			},
			"controls_type": schema.StringAttribute{
				MarkdownDescription: "The type of controls to display in PDF reports. Only available for COMPLIANCE-STANDARD reports, not for GENERIC reports. Allowed values: withChecksOnly (controls with checks), noChecksOnly (controls without checks), all (all controls). Default: all",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("withChecksOnly", "noChecksOnly", "all"),
				},
				PlanModifiers: []planmodifier.String{
					nullIfGenericReportPlanModifier{},
				},
			},
		},
		Blocks: map[string]schema.Block{
			"schedule": schema.ListNestedBlock{
				MarkdownDescription: "Schedule configuration for automated report generation.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether the report is scheduled to run automatically. Defaults to false if not specified.",
							Optional:            true,
							Computed:            true,
						},
						"frequency": schema.StringAttribute{
							MarkdownDescription: "Cron expression for schedule frequency. Format: '(day of month) (month) (day of week)'. Examples: '* * 2' (every Tuesday), '1 * *' (1st of every month). Required when enabled is true.",
							Optional:            true,
							Computed:            true,
						},
						"timezone": schema.StringAttribute{
							MarkdownDescription: "Required when enabled is true. It's used as which timezone the report schedule is based on, when the attribute scheduled is true. If this attribute was provided, it must be string that is a valid value of timezone database name such as Australia/Sydney. Available timzezones https://en.wikipedia.org/wiki/List_of_tz_database_time_zones.",
							Optional:            true,
							Computed:            true,
							Validators: []validator.String{
								timezoneValidator{},
							},
						},
					},
				},
			},
			"checks_filter": schema.ListNestedBlock{
				MarkdownDescription: "Filters to determine which checks appear in the report. Multiple conditions within a field use OR logic. Different fields use AND logic.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"categories": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by compliance categories. Allowed values: security, cost-optimisation, reliability, performance-efficiency, operational-excellence, sustainability.",
							Optional:            true,
							Validators: []validator.Set{
								setvalidator.ValueStringsAre(stringvalidator.OneOf("security", "cost-optimisation", "reliability", "performance-efficiency", "operational-excellence", "sustainability")),
							},
						},
						"compliance_standard_ids": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by compliance standard IDs (for GENERIC reports only).",
							Optional:            true,
						},
						"tags": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by tags.",
							Optional:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "The filter for including checks in the report based on the description of a check.",
							Optional:            true,
						},
						"newer_than_days": schema.Int64Attribute{
							MarkdownDescription: "Include checks from the last N days (max 365). Example: 5 includes checks from the last 5 days.",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
								int64validator.AtMost(365),
							},
						},
						"older_than_days": schema.Int64Attribute{
							MarkdownDescription: "Include checks older than N days (max 365). Example: 5 includes checks older than 5 days.",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
								int64validator.AtMost(365),
							},
						},
						"providers": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by cloud providers.",
							Optional:            true,
						},
						"regions": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by cloud regions.",
							Optional:            true,
						},
						"resource_id": schema.StringAttribute{
							MarkdownDescription: "Filter by resource ID.",
							Optional:            true,
						},
						"resource_search_mode": schema.StringAttribute{
							MarkdownDescription: "Resource search mode. Allowed values: text, regex.",
							Optional:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("text", "regex"),
							},
						},
						"resource_types": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by resource types (e.g., 'kms-key', 's3-bucket').",
							Optional:            true,
						},
						"risk_levels": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by risk levels.",
							Optional:            true,
							Validators: []validator.Set{
								setvalidator.ValueStringsAre(stringvalidator.OneOf("LOW", "MEDIUM", "HIGH", "VERY_HIGH", "EXTREME")),
							},
						},
						"rule_ids": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by specific rule IDs (e.g., 'S3-001', 'IAM-045').",
							Optional:            true,
						},
						"services": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by cloud services.",
							Optional:            true,
						},
						"statuses": schema.SetAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Filter by check statuses. Allowed values: SUCCESS, FAILURE.",
							Optional:            true,
							Validators: []validator.Set{
								setvalidator.ValueStringsAre(stringvalidator.OneOf("SUCCESS", "FAILURE")),
							},
						},
						"suppressed": schema.BoolAttribute{
							MarkdownDescription: "Whether to include suppressed or regular checks only. If not provided, both suppressed and unsuppressed checks are included.",
							Optional:            true,
						},
					},
				},
			},
		},
	}
}

// Validates the resource configuration.
func (r *reportConfigResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ReportConfigResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that both account_id and group_id are not specified together
	// Note: Both can be null for company-level reports
	if !data.AccountID.IsNull() && !data.GroupID.IsNull() {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Cannot specify both 'account_id' and 'group_id' at the same time.",
		)
	}

	// Validate compliance standard requirement for COMPLIANCE-STANDARD report type
	if !data.ReportType.IsNull() && data.ReportType.ValueString() == utils.ReportTypeComplianceStandard {
		if data.AppliedComplianceStandardID.IsNull() || data.AppliedComplianceStandardID.ValueString() == "" {
			resp.Diagnostics.AddError(
				"Invalid Configuration",
				"'applied_compliance_standard_id' is required when 'report_type' is COMPLIANCE-STANDARD.",
			)
		}
	}

	// Validate include_account_names is only used with group_id (group/company level)
	if !data.IncludeAccountNames.IsNull() && !data.AccountID.IsNull() {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"'include_account_names' is only available for group-level and company-level reports. Remove 'include_account_names' when using 'account_id'.",
		)
	}

	// Validate controls_type is only used with COMPLIANCE-STANDARD report type
	if !data.ControlsType.IsNull() && data.ControlsType.ValueString() != "" {
		if !data.ReportType.IsNull() && data.ReportType.ValueString() != utils.ReportTypeComplianceStandard {
			resp.Diagnostics.AddError(
				"Invalid Configuration",
				"'controls_type' is only available for COMPLIANCE-STANDARD reports. Remove 'controls_type' when using GENERIC report type.",
			)
		}
	}
}

// Configure adds the provider configured client to the resource.
func (r *reportConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Creates the resource and sets the initial Terraform state.
func (r *reportConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ReportConfigResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Create new Report Config plan: %+v", plan))

	// Convert plan to create request
	createReq, err := utils.ConvertToCreateReportConfigRequest(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Report Config",
			"An error occurred converting plan to request: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Create new Report Config request: %+v", createReq))

	apiResponse, err := r.client.CreateReportConfig(&createReq)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Create Report Config",
			"An unexpected error occurred when creating the report config. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Vision One Client: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(apiResponse.ID)

	// Read back to get full state
	if !plan.ID.IsNull() && plan.ID.ValueString() != "" {
		reportConfig, err := r.client.GetReportConfig(plan.ID.ValueString())
		if err != nil {
			tflog.Debug(ctx, err.Error())
			resp.Diagnostics.AddError(
				"Unable to Read Report Config",
				"An unexpected error occurred when reading the report config after creation. "+
					"Vision One Client: "+err.Error(),
			)
			return
		}
		utils.UpdateStateFromReportConfig(ctx, &plan, reportConfig)
	}

	// Set state
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Refreshes the Terraform state with the latest data.
func (r *reportConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ReportConfigResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reportConfig, err := r.client.GetReportConfig(state.ID.ValueString())
	if api.IsNotFoundError(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Read Report Config",
			"An unexpected error occurred when reading the report config. "+
				"Vision One Client: "+err.Error(),
		)
		return
	}

	// Update state from API response
	utils.UpdateStateFromReportConfig(ctx, &state, reportConfig)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update the resource and sets the updated Terraform state on success.
func (r *reportConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ReportConfigResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ReportConfigResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert plan to update request
	updateReq, err := utils.ConvertToUpdateReportConfigRequest(ctx, &plan, &state)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update Report Config",
			"An error occurred converting plan to request: "+err.Error(),
		)
		return
	}

	err = r.client.UpdateReportConfig(plan.ID.ValueString(), &updateReq)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Update Report Config",
			"An unexpected error occurred when updating the report config. "+
				"Vision One Client: "+err.Error(),
		)
		return
	}

	// Read back to get updated state
	reportConfig, err := r.client.GetReportConfig(plan.ID.ValueString())
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Read Report Config",
			"An unexpected error occurred when reading the report config after update. "+
				"Vision One Client: "+err.Error(),
		)
		return
	}
	utils.UpdateStateFromReportConfig(ctx, &plan, reportConfig)

	// Set updated state
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Deletes the resource and removes the Terraform state on success.
func (r *reportConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ReportConfigResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteReportConfig(state.ID.ValueString())
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Delete Report Config",
			"An unexpected error occurred when deleting the report config. "+
				"Vision One Client: "+err.Error(),
		)
	}
}

// ImportState imports the resource state.
func (r *reportConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// =============================================================================
// Helper Functions
// =============================================================================

// Validates if a string is a valid IANA timezone
type timezoneValidator struct{}

func (v timezoneValidator) Description(_ context.Context) string {
	return "value must be a valid IANA timezone (e.g., America/New_York, Europe/Paris, UTC)"
}

func (v timezoneValidator) MarkdownDescription(_ context.Context) string {
	return "value must be a valid IANA timezone (e.g., `America/New_York`, `Europe/Paris`, `UTC`)"
}

func (v timezoneValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	timezone := req.ConfigValue.ValueString()
	if timezone == "" {
		return
	}

	// Use Go's built-in timezone validation
	_, err := time.LoadLocation(timezone)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Timezone",
			fmt.Sprintf("The value %q is not a valid IANA timezone. "+
				"Please use a valid timezone from the IANA timezone database (e.g., 'America/New_York', 'Europe/Paris', 'UTC'). "+
				"See https://en.wikipedia.org/wiki/List_of_tz_database_time_zones for a complete list. "+
				"Error: %s", timezone, err.Error()),
		)
	}
}

// Set the value to null if this is an account-level report
type nullIfAccountLevelPlanModifier struct{}

func (m nullIfAccountLevelPlanModifier) Description(_ context.Context) string {
	return "Sets value to null for account-level reports"
}

func (m nullIfAccountLevelPlanModifier) MarkdownDescription(_ context.Context) string {
	return "Sets value to null for account-level reports"
}

func (m nullIfAccountLevelPlanModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	// Don't run during destroy
	if req.Plan.Raw.IsNull() {
		return
	}

	var accountID types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("account_id"), &accountID)...)
	if resp.Diagnostics.HasError() {
		tflog.Debug(ctx, fmt.Sprintf("Failed to get account_id attribute in plan modifier: %v", resp.Diagnostics))
		return
	}

	// If account_id is set (account-level report), set this field to null
	if !accountID.IsNull() {
		resp.PlanValue = types.BoolNull()
	}
}

// nullIfGenericReportPlanModifier sets the value to null if this is a GENERIC report
type nullIfGenericReportPlanModifier struct{}

func (m nullIfGenericReportPlanModifier) Description(_ context.Context) string {
	return "Sets value to null for GENERIC reports"
}

func (m nullIfGenericReportPlanModifier) MarkdownDescription(_ context.Context) string {
	return "Sets value to null for GENERIC reports"
}

func (m nullIfGenericReportPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Don't run during destroy
	if req.Plan.Raw.IsNull() {
		return
	}

	var reportType types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("report_type"), &reportType)...)
	if resp.Diagnostics.HasError() {
		tflog.Debug(ctx, fmt.Sprintf("Failed to get report_type attribute in plan modifier: %v", resp.Diagnostics))
		return
	}

	// If report_type is GENERIC, set controls_type to null
	if !reportType.IsNull() && reportType.ValueString() != utils.ReportTypeComplianceStandard {
		resp.PlanValue = types.StringNull()
	}
}

// nullIfNotInConfigPlanModifier sets the value to null if it's not specified in the user's config
// This ensures we don't send the field to the API when the user hasn't explicitly set it,
// allowing the API to use its default value instead of propagating a value from state
type nullIfNotInConfigPlanModifier struct{}

func (m nullIfNotInConfigPlanModifier) Description(_ context.Context) string {
	return "Sets value to null if not specified in config"
}

func (m nullIfNotInConfigPlanModifier) MarkdownDescription(_ context.Context) string {
	return "Sets value to null if not specified in config"
}

func (m nullIfNotInConfigPlanModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	// Don't run during destroy
	if req.Plan.Raw.IsNull() {
		return
	}

	// If the value is null in the config, keep it null in the plan
	// This prevents state values from being used when the user hasn't specified the field
	if req.ConfigValue.IsNull() {
		resp.PlanValue = types.BoolNull()
	}
}
