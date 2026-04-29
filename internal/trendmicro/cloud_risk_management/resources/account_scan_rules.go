package resources

import (
	"context"
	"errors"
	"fmt"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/utils"
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

var (
	_ resource.Resource                = &accountScanRulesResource{}
	_ resource.ResourceWithConfigure   = &accountScanRulesResource{}
	_ resource.ResourceWithImportState = &accountScanRulesResource{}
)

type accountScanRulesResource struct {
	client *api.CrmClient
}

type AccountScanRulesResourceModel struct {
	AccountID types.String          `tfsdk:"account_id"`
	ScanRules []utils.ScanRuleModel `tfsdk:"scan_rule"`
}

func NewAccountScanRulesResource() resource.Resource {
	return &accountScanRulesResource{
		client: &api.CrmClient{},
	}
}

func (r *accountScanRulesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_account_scan_rules"
}

func (r *accountScanRulesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages scan rule settings for a Vision One Cloud Risk Management account.\n\n" +
			"Scan rules are provisioned automatically when an account is onboarded. Use this resource to customize their configurations.",
		Attributes: map[string]schema.Attribute{
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The Vision One Cloud Risk Management internal account ID to manage scan rule settings for.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"scan_rule": schema.SetNestedBlock{
				MarkdownDescription: "List of scan rule settings.",
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

func (r *accountScanRulesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accountScanRulesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AccountScanRulesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := plan.AccountID.ValueString()
	tflog.Debug(ctx, fmt.Sprintf("Create account rule setting for account: %s", accountID))

	var partialFailure *api.PartialFailureError
	if len(plan.ScanRules) > 0 {
		ruleSettings, err := convertAccountScanRulesToDTO(plan.ScanRules)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Update Account Scan Rule Settings",
				"An error occurred converting scan rules: "+err.Error(),
			)
			return
		}

		err = r.client.UpdateAccountRuleSettings(accountID, ruleSettings)
		if err != nil {
			tflog.Debug(ctx, err.Error())

			if !errors.As(err, &partialFailure) {
				resp.Diagnostics.AddError(
					"Unable to Update Account Scan Rule Settings",
					"An unexpected error occurred while updating scan rule settings: "+err.Error(),
				)
				return
			}

			if partialFailure.FailCount == partialFailure.TotalCount {
				resp.Diagnostics.AddError(
					"Unable to Update Account Scan Rule Settings",
					partialFailure.Error(),
				)
				return
			}

			// Partial failure — some rules were applied. Continue to read back
			// and save state reflecting only what the API actually accepted.
		}
	}

	// Read back to get the current state from API
	r.readAndUpdatePlan(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)

	// Report partial failure after state is saved.
	if partialFailure != nil {
		resp.Diagnostics.AddError(
			"Unable to Update Account Scan Rule Settings",
			partialFailure.Error(),
		)
	}
}

func (r *accountScanRulesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AccountScanRulesResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := state.AccountID.ValueString()

	apiRuleSettings, err := r.client.GetAccountRuleSettings(accountID)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Read Account Scan Rule Settings",
			"An unexpected error occurred while reading scan rule settings: "+err.Error(),
		)
		return
	}

	updatePlanFromAccountRuleSettings(&state, apiRuleSettings)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *accountScanRulesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AccountScanRulesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state AccountScanRulesResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := plan.AccountID.ValueString()

	// Detect rules removed from the template and reset them
	planRuleIDs := make(map[string]struct{}, len(plan.ScanRules))
	for _, rule := range plan.ScanRules {
		planRuleIDs[rule.ID.ValueString()] = struct{}{}
	}

	var removedRuleIDs []string
	for _, rule := range state.ScanRules {
		ruleID := rule.ID.ValueString()
		if _, exists := planRuleIDs[ruleID]; !exists {
			removedRuleIDs = append(removedRuleIDs, ruleID)
		}
	}

	if len(removedRuleIDs) > 0 {
		tflog.Debug(ctx, fmt.Sprintf("Resetting %d removed rule setting(s) for account %s", len(removedRuleIDs), accountID))

		if err := r.deleteAndReportError(ctx, accountID, removedRuleIDs, &resp.Diagnostics); err != nil {
			return
		}
	}

	var partialFailure *api.PartialFailureError
	// updateTotalFailure tracks when the update call completely failed but
	// rules were already deleted — we must still save state to reflect those
	// deletions rather than returning early with stale state.
	var updateTotalFailure bool
	var updateErr error
	if len(plan.ScanRules) > 0 {
		ruleSettings, err := convertAccountScanRulesToDTO(plan.ScanRules)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Update Account Scan Rule Settings",
				"An error occurred converting scan rules: "+err.Error(),
			)
			return
		}

		err = r.client.UpdateAccountRuleSettings(accountID, ruleSettings)
		if err != nil {
			tflog.Debug(ctx, err.Error())

			if !errors.As(err, &partialFailure) {
				if len(removedRuleIDs) == 0 {
					resp.Diagnostics.AddError(
						"Unable to Update Account Scan Rule Settings",
						"An unexpected error occurred while updating scan rule settings: "+err.Error(),
					)
					return
				}
				// Rules were successfully deleted but update failed.
				// Fall through to read back and save state reflecting the deletions.
				updateTotalFailure = true
				updateErr = err
			} else if partialFailure.FailCount == partialFailure.TotalCount {
				if len(removedRuleIDs) == 0 {
					resp.Diagnostics.AddError(
						"Unable to Update Account Scan Rule Settings",
						partialFailure.Error(),
					)
					return
				}
				// Rules were successfully deleted but all updates failed.
				// Fall through to read back and save state reflecting the deletions.
				updateTotalFailure = true
				updateErr = err
			}
			// else: Partial failure — some rules were applied. Continue to read back
			// and save state reflecting only what the API actually accepted.
		}
	}

	// Read back to get updated state
	r.readAndUpdatePlan(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)

	// Report update errors after state is saved.
	if updateTotalFailure && partialFailure == nil {
		resp.Diagnostics.AddError(
			"Unable to Update Account Scan Rule Settings",
			"An unexpected error occurred while updating scan rule settings: "+updateErr.Error(),
		)
	} else if partialFailure != nil {
		resp.Diagnostics.AddError(
			"Unable to Update Account Scan Rule Settings",
			partialFailure.Error(),
		)
	}
}

func (r *accountScanRulesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AccountScanRulesResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := state.AccountID.ValueString()

	if len(state.ScanRules) > 0 {
		ruleIDs := make([]string, len(state.ScanRules))
		for i, rule := range state.ScanRules {
			ruleIDs[i] = rule.ID.ValueString()
		}

		tflog.Debug(ctx, fmt.Sprintf("Resetting %d rule setting(s) for account %s", len(ruleIDs), accountID))

		if err := r.deleteAndReportError(ctx, accountID, ruleIDs, &resp.Diagnostics); err != nil {
			return
		}
	}

	tflog.Debug(ctx, fmt.Sprintf("Successfully reset rule settings for account %s", accountID))
}

// ImportState imports the resource state using the account ID.
func (r *accountScanRulesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("account_id"), req.ID)...)
}

func convertAccountScanRulesToDTO(rules []utils.ScanRuleModel) ([]cloud_risk_management_dto.AccountRuleSettingUpdate, error) {
	result := make([]cloud_risk_management_dto.AccountRuleSettingUpdate, len(rules))

	for i, rule := range rules {
		ruleSetting, err := utils.ConvertScanRuleToDTO(rule)
		if err != nil {
			return nil, err
		}

		// "note" is excluded from the Terraform schema and it sets static default value because:
		// 1. Notes are append-only — each API call adds a new entry; existing notes cannot be updated or deleted.
		//    Not manageable by users.
		// 2. The GET API does not return notes, so Terraform cannot read back the value, causing
		//    "inconsistent result after apply" errors.
		result[i] = cloud_risk_management_dto.AccountRuleSettingUpdate{
			ScanRule: ruleSetting,
			Note:     "Updated by Trend Micro Vision One Terraform Provider",
		}
	}

	return result, nil
}

// readAndUpdatePlan reads the account rule settings from the API and updates the plan/state model.
func (r *accountScanRulesResource) readAndUpdatePlan(ctx context.Context, plan *AccountScanRulesResourceModel, diagnostics *diag.Diagnostics) {
	accountID := plan.AccountID.ValueString()

	apiRuleSettings, err := r.client.GetAccountRuleSettings(accountID)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		diagnostics.AddError(
			"Unable to Read Account Scan Rule Settings",
			"An unexpected error occurred while reading scan rule settings: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Read %d rule settings from API for account %s", len(apiRuleSettings), accountID))

	updatePlanFromAccountRuleSettings(plan, apiRuleSettings)
}

// updatePlanFromAccountRuleSettings rebuilds the Terraform plan/state model from the GET API response.
// Only rules returned by the GET API (customized rules) are included in state.
// Rules not returned are dropped, so Terraform detects drift and plans a re-deploy.
func updatePlanFromAccountRuleSettings(plan *AccountScanRulesResourceModel, apiRuleSettings []cloud_risk_management_dto.AccountRuleSetting) {
	// Build maps from original plan for preserving original extra_settings
	originalExtraSettings := make(map[string]map[string]*utils.ExtraSettingModel)
	for _, rule := range plan.ScanRules {
		ruleID := rule.ID.ValueString()
		if len(rule.ExtraSettings) > 0 {
			originalExtraSettings[ruleID] = make(map[string]*utils.ExtraSettingModel)
			for i := range rule.ExtraSettings {
				settingName := rule.ExtraSettings[i].Name.ValueString()
				originalExtraSettings[ruleID][settingName] = &rule.ExtraSettings[i]
			}
		}
	}

	// Build lookup map from API response
	apiRuleSettingMap := make(map[string]*cloud_risk_management_dto.AccountRuleSetting)
	for i := range apiRuleSettings {
		apiRuleSettingMap[apiRuleSettings[i].ID] = &apiRuleSettings[i]
	}

	// Full rebuild: only include rules that exist in the API response (customized).
	// Rules not returned by GET are dropped from state, so Terraform detects drift
	// and plans a re-deploy on the next apply.
	var newScanRules []utils.ScanRuleModel

	if len(plan.ScanRules) == 0 {
		// Import case: state has no rules yet, populate from all API rules.
		for i := range apiRuleSettings {
			rebuilt := rebuildScanRuleFromAPI(&apiRuleSettings[i], nil)
			newScanRules = append(newScanRules, rebuilt)
		}
	} else {
		for _, planRule := range plan.ScanRules {
			ruleID := planRule.ID.ValueString()
			apiRuleSetting, found := apiRuleSettingMap[ruleID]

			if !found {
				// Rule not in API response — not customized, skip it.
				// Terraform will detect the missing rule and plan re-creation.
				continue
			}

			// Rebuild from API data, preserving plan-only fields
			rebuilt := rebuildScanRuleFromAPI(apiRuleSetting, originalExtraSettings[ruleID])
			newScanRules = append(newScanRules, rebuilt)
		}
	}

	plan.ScanRules = newScanRules
}

// rebuildScanRuleFromAPI constructs a ScanRuleModel from an API response rule.
// origExtraSettings maps setting names to original plan values for preserving user-specified data.
func rebuildScanRuleFromAPI(apiRuleSetting *cloud_risk_management_dto.AccountRuleSetting, origExtraSettings map[string]*utils.ExtraSettingModel) utils.ScanRuleModel {
	rebuilt := utils.ScanRuleModel{
		ID:        types.StringValue(apiRuleSetting.ID),
		Provider:  types.StringValue(apiRuleSetting.Provider),
		Enabled:   types.BoolValue(apiRuleSetting.Enabled),
		RiskLevel: types.StringValue(apiRuleSetting.RiskLevel),
	}

	rebuilt.Exceptions = utils.ConvertExceptionsFromDTO(apiRuleSetting.Exceptions)

	if len(apiRuleSetting.ExtraSettings) > 0 {
		rebuilt.ExtraSettings = utils.ConvertExtraSettingsFromDTO(apiRuleSetting.ExtraSettings, origExtraSettings)
	} else {
		rebuilt.ExtraSettings = []utils.ExtraSettingModel{}
	}

	return rebuilt
}

// deleteAndReportError calls DeleteAccountRuleSettings and adds appropriate diagnostics on error.
// Returns the error (nil on success) so callers can decide whether to return early.
func (r *accountScanRulesResource) deleteAndReportError(ctx context.Context, accountID string, ruleIDs []string, diagnostics *diag.Diagnostics) error {
	err := r.client.DeleteAccountRuleSettings(accountID, ruleIDs)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		var partialFailure *api.PartialFailureError
		if errors.As(err, &partialFailure) {
			diagnostics.AddError(
				"Unable to Reset Account Scan Rule Settings",
				partialFailure.Error(),
			)
		} else {
			diagnostics.AddError(
				"Unable to Reset Account Scan Rule Settings",
				"An unexpected error occurred while resetting scan rule settings: "+err.Error(),
			)
		}
	}
	return err
}
