package resources

import (
	"context"
	"fmt"
	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/utils"
	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                   = &accountScanSettingResource{}
	_ resource.ResourceWithConfigure      = &accountScanSettingResource{}
	_ resource.ResourceWithImportState    = &accountScanSettingResource{}
	_ resource.ResourceWithValidateConfig = &accountScanSettingResource{}
)

type accountScanSettingResource struct {
	client *api.CrmClient
}

type AccountScanSettingResourceModel struct {
	AccountID             types.String `tfsdk:"account_id"`
	DisabledRegions       types.List   `tfsdk:"disabled_regions"`
	DisabledUntilDateTime types.String `tfsdk:"disabled_until_datetime"`
	Enabled               types.Bool   `tfsdk:"enabled"`
	Interval              types.Int64  `tfsdk:"interval"`
}

func NewAccountScanSettingResource() resource.Resource {
	return &accountScanSettingResource{
		client: &api.CrmClient{},
	}
}

func (r *accountScanSettingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_account_scan_setting"
}

func (r *accountScanSettingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accountScanSettingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages scan settings for a Cloud Risk Management account.\n\n" +
			"Account scan settings control how and when cloud posture scans are performed. " +
			"These settings are automatically created when an account is added and can be updated to customize scan behavior.",
		Attributes: map[string]schema.Attribute{
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The CRM account ID for which to manage scan settings.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"disabled_regions": schema.ListAttribute{
				MarkdownDescription: "List of cloud regions where scanning is disabled. Only applicable for AWS accounts. For other providers, please do not use this attribute.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"disabled_until_datetime": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 datetime string indicating when scanning should be disabled until. " +
					"After this time, scanning will automatically resume. Leave empty to not use this feature.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Validators: []validator.String{
					utils.ISO8601Datetime(),
					utils.DatetimeRangeFromNow(1, 72, time.Hour),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether scanning is enabled for this account.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"interval": schema.Int64Attribute{
				MarkdownDescription: "Scan interval in hours. Determines how frequently the account is scanned.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
					int64validator.AtMost(12),
				},
			},
		},
	}
}

func (r *accountScanSettingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AccountScanSettingResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := plan.AccountID.ValueString()
	tflog.Debug(ctx, fmt.Sprintf("Creating account scan setting for account: %s", accountID))

	// Build update request from plan
	updateReq := buildUpdateRequest(&plan)

	// Update the settings
	err := r.client.UpdateAccountScanSetting(accountID, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Account Scan Setting",
			"Could not update account scan settings for account "+accountID+": "+err.Error(),
		)
		return
	}

	// Read the updated settings
	updatedSettings, err := r.client.GetAccountScanSetting(accountID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Account Scan Settings",
			"Updated account scan settings but failed to read them back: "+err.Error(),
		)
		return
	}

	// Map the response to state
	mapAccountScanSettingToState(updatedSettings, &plan, accountID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *accountScanSettingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AccountScanSettingResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := state.AccountID.ValueString()
	tflog.Debug(ctx, fmt.Sprintf("Reading account scan setting for account: %s", accountID))

	settings, err := r.client.GetAccountScanSetting(accountID)
	if api.IsNotFoundError(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Account Scan Settings",
			"Could not read scan settings for account "+accountID+": "+err.Error(),
		)
		return
	}

	mapAccountScanSettingToState(settings, &state, accountID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *accountScanSettingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AccountScanSettingResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := plan.AccountID.ValueString()
	tflog.Debug(ctx, fmt.Sprintf("Updating account scan setting for account: %s", accountID))

	// Build update request from plan
	updateReq := buildUpdateRequest(&plan)

	// Update the settings
	err := r.client.UpdateAccountScanSetting(accountID, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Account Scan Setting",
			"Could not update account scan settings for account "+accountID+": "+err.Error(),
		)
		return
	}

	// Read the updated settings
	updatedSettings, err := r.client.GetAccountScanSetting(accountID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Account Scan Settings",
			"Updated account scan settings but failed to read them back: "+err.Error(),
		)
		return
	}

	// Map the response to state
	mapAccountScanSettingToState(updatedSettings, &plan, accountID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *accountScanSettingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AccountScanSettingResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateModel := &cloud_risk_management_dto.AccountScanSetting{
		DisabledRegions:       []string{},
		Enabled:               true,
		Interval:              1,
		DisabledUntilDateTime: nil,
	}
	err := r.client.UpdateAccountScanSetting(state.AccountID.ValueString(), updateModel)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Resetting Account Scan Setting",
			"Could not reset account scan settings for account "+state.AccountID.ValueString()+": "+err.Error(),
		)
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Deleting(Reset) account scan setting for account: %s", state.AccountID.ValueString()))
}

func (r *accountScanSettingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import using the account ID
	resource.ImportStatePassthroughID(ctx, path.Root("account_id"), req, resp)
}

func (r *accountScanSettingResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config AccountScanSettingResourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only validate when disabled_regions is explicitly set and non-empty
	if config.DisabledRegions.IsNull() || config.DisabledRegions.IsUnknown() || len(config.DisabledRegions.Elements()) == 0 {
		return
	}

	// account_id may be unknown during planning (e.g. computed from another resource)
	if config.AccountID.IsNull() || config.AccountID.IsUnknown() {
		return
	}

	// Client is only available after Configure; skip API validation if not ready
	if r.client == nil || r.client.Client == nil || r.client.HostURL == "" {
		return
	}

	accountID := config.AccountID.ValueString()
	account, err := r.client.GetAccountById(accountID)
	if err != nil {
		// Do not block apply on API errors; the server will surface any issues
		return
	}

	if account == nil || account.AwsAccountID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("disabled_regions"),
			"disabled_regions Is Only Applicable for AWS Accounts",
			fmt.Sprintf(
				"The account_id %q does not correspond to a valid AWS account. "+
					"The disabled_regions field is only applicable for AWS accounts. "+
					"Please remove disabled_regions or use an AWS account ID.",
				accountID,
			),
		)
	}
}

// Helper function to build update request from plan
func buildUpdateRequest(plan *AccountScanSettingResourceModel) *cloud_risk_management_dto.AccountScanSetting {
	updateReq := &cloud_risk_management_dto.AccountScanSetting{
		Enabled:  plan.Enabled.ValueBool(),
		Interval: int(plan.Interval.ValueInt64()),
	}

	if !plan.DisabledRegions.IsNull() && !plan.DisabledRegions.IsUnknown() {
		var disabledRegions []string
		for _, element := range plan.DisabledRegions.Elements() {
			regionValue := element.(types.String)
			if !regionValue.IsNull() && !regionValue.IsUnknown() {
				disabledRegions = append(disabledRegions, regionValue.ValueString())
			}
		}
		updateReq.DisabledRegions = disabledRegions
	} else {
		updateReq.DisabledRegions = []string{}
	}

	// Always set disabledUntilDateTime, even if it's empty
	// This allows clearing the value when it's removed from config
	if !plan.DisabledUntilDateTime.IsNull() && !plan.DisabledUntilDateTime.IsUnknown() {
		disabledUntil := plan.DisabledUntilDateTime.ValueString()
		updateReq.DisabledUntilDateTime = &disabledUntil
	} else {
		updateReq.DisabledUntilDateTime = nil
	}

	return updateReq
}

// Helper function to map API response to Terraform state
func mapAccountScanSettingToState(settings *cloud_risk_management_dto.AccountScanSetting, state *AccountScanSettingResourceModel, accountID string, diags *diag.Diagnostics) {
	state.AccountID = types.StringValue(accountID)
	state.Enabled = types.BoolValue(settings.Enabled)
	state.Interval = types.Int64Value(int64(settings.Interval))
	if settings.DisabledUntilDateTime != nil {
		state.DisabledUntilDateTime = types.StringValue(*settings.DisabledUntilDateTime)
	} else {
		state.DisabledUntilDateTime = types.StringValue("")
	}

	// Convert disabled regions slice to List
	if len(settings.DisabledRegions) > 0 {
		regionValues := make([]attr.Value, len(settings.DisabledRegions))
		for i, region := range settings.DisabledRegions {
			regionValues[i] = types.StringValue(region)
		}
		listValue, listDiags := types.ListValue(types.StringType, regionValues)
		diags.Append(listDiags...)
		state.DisabledRegions = listValue
	} else {
		state.DisabledRegions = types.ListValueMust(types.StringType, []attr.Value{})
	}
}
