package utils

import (
	"context"
	"fmt"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	ReportTypeComplianceStandard = "COMPLIANCE-STANDARD"
)

// The resource data model.
type ReportConfigResourceModel struct {
	ID                          types.String    `tfsdk:"id"`
	AccountID                   types.String    `tfsdk:"account_id"`
	GroupID                     types.String    `tfsdk:"group_id"`
	Level                       types.String    `tfsdk:"level"`
	ReportTitle                 types.String    `tfsdk:"report_title"`
	ReportType                  types.String    `tfsdk:"report_type"`
	IncludeChecks               types.Bool      `tfsdk:"include_checks"`
	IncludeAccountNames         types.Bool      `tfsdk:"include_account_names"`
	EmailRecipients             types.Set       `tfsdk:"email_recipients"`
	ReportFormatsInEmail        types.Set       `tfsdk:"report_formats_in_email"`
	Language                    types.String    `tfsdk:"language"`
	AppliedComplianceStandardID types.String    `tfsdk:"applied_compliance_standard_id"`
	ControlsType                types.String    `tfsdk:"controls_type"`
	Schedule                    []ScheduleModel `tfsdk:"schedule"`
	ChecksFilter                []FilterModel   `tfsdk:"checks_filter"`
}

type ScheduleModel struct {
	Enabled   types.Bool   `tfsdk:"enabled"`
	Frequency types.String `tfsdk:"frequency"`
	Timezone  types.String `tfsdk:"timezone"`
}

type FilterModel struct {
	Categories            types.Set    `tfsdk:"categories"`
	ComplianceStandardIds types.Set    `tfsdk:"compliance_standard_ids"`
	Tags                  types.Set    `tfsdk:"tags"`
	Description           types.String `tfsdk:"description"`
	NewerThanDays         types.Int64  `tfsdk:"newer_than_days"`
	OlderThanDays         types.Int64  `tfsdk:"older_than_days"`
	Providers             types.Set    `tfsdk:"providers"`
	Regions               types.Set    `tfsdk:"regions"`
	ResourceID            types.String `tfsdk:"resource_id"`
	ResourceSearchMode    types.String `tfsdk:"resource_search_mode"`
	ResourceTypes         types.Set    `tfsdk:"resource_types"`
	RiskLevels            types.Set    `tfsdk:"risk_levels"`
	RuleIds               types.Set    `tfsdk:"rule_ids"`
	Services              types.Set    `tfsdk:"services"`
	Statuses              types.Set    `tfsdk:"statuses"`
	Suppressed            types.Bool   `tfsdk:"suppressed"`
}

// Converts Terraform model to DTO create request
func ConvertToCreateReportConfigRequest(ctx context.Context, plan *ReportConfigResourceModel) (cloud_risk_management_dto.CreateReportConfigRequest, error) {
	req := cloud_risk_management_dto.CreateReportConfigRequest{}

	// Set account or group ID
	if !plan.AccountID.IsNull() {
		req.AccountID = plan.AccountID.ValueString()
	}
	if !plan.GroupID.IsNull() {
		req.GroupID = plan.GroupID.ValueString()
	}

	// Top-level fields
	req.ReportTitle = plan.ReportTitle.ValueString()

	if !plan.ReportType.IsNull() {
		req.ReportType = plan.ReportType.ValueString()
	}

	if !plan.IncludeChecks.IsNull() {
		val := plan.IncludeChecks.ValueBool()
		req.IncludeChecks = &val
	}

	// Include account names only for group/company level (not for account level)
	// Only send if user explicitly specified it in config (not null in plan)
	if plan.AccountID.IsNull() && !plan.IncludeAccountNames.IsNull() {
		val := plan.IncludeAccountNames.ValueBool()
		req.IncludeAccountNames = &val
	}

	// Email recipients
	if !plan.EmailRecipients.IsNull() && !plan.EmailRecipients.IsUnknown() {
		var emails []string
		diags := plan.EmailRecipients.ElementsAs(ctx, &emails, false)
		if diags.HasError() {
			return req, fmt.Errorf("failed to convert email recipients: %v", diags)
		}
		req.EmailRecipients = emails
	}

	// Report formats
	if !plan.ReportFormatsInEmail.IsNull() && !plan.ReportFormatsInEmail.IsUnknown() {
		var formats []string
		diags := plan.ReportFormatsInEmail.ElementsAs(ctx, &formats, false)
		if diags.HasError() {
			return req, fmt.Errorf("failed to convert report formats: %v", diags)
		}
		req.ReportFormatsInEmail = formats
	}

	// Language
	if !plan.Language.IsNull() {
		req.Language = plan.Language.ValueString()
	}

	// Schedule
	if len(plan.Schedule) > 0 {
		schedule := plan.Schedule[0]

		hasFrequency := !schedule.Frequency.IsNull() && schedule.Frequency.ValueString() != ""
		hasTimezone := !schedule.Timezone.IsNull() && schedule.Timezone.ValueString() != ""

		if hasFrequency || hasTimezone {
			req.Schedule = &cloud_risk_management_dto.ReportSchedule{}

			if hasFrequency {
				req.Schedule.Frequency = schedule.Frequency.ValueString()
			}
			if hasTimezone {
				req.Schedule.Timezone = schedule.Timezone.ValueString()
			}
		}
	}

	// Compliance Standard specific
	if !plan.ReportType.IsNull() && plan.ReportType.ValueString() == ReportTypeComplianceStandard {
		if !plan.AppliedComplianceStandardID.IsNull() {
			req.AppliedComplianceStandardID = plan.AppliedComplianceStandardID.ValueString()
		}
		if !plan.ControlsType.IsNull() {
			req.ControlsType = plan.ControlsType.ValueString()
		}
	}

	// Filter
	if len(plan.ChecksFilter) > 0 {
		filter, err := convertFilterToDTO(ctx, &plan.ChecksFilter[0])
		if err != nil {
			return req, fmt.Errorf("failed to convert filter: %w", err)
		}
		req.ChecksFilter = filter
	}

	return req, nil
}

// Check if any set-type filter field has a value
func hasAnySetFilterValue(filter *FilterModel) bool {
	return (!filter.Categories.IsNull() && !filter.Categories.IsUnknown()) ||
		(!filter.ComplianceStandardIds.IsNull() && !filter.ComplianceStandardIds.IsUnknown()) ||
		(!filter.Tags.IsNull() && !filter.Tags.IsUnknown()) ||
		(!filter.Providers.IsNull() && !filter.Providers.IsUnknown()) ||
		(!filter.Regions.IsNull() && !filter.Regions.IsUnknown()) ||
		(!filter.ResourceTypes.IsNull() && !filter.ResourceTypes.IsUnknown()) ||
		(!filter.RiskLevels.IsNull() && !filter.RiskLevels.IsUnknown()) ||
		(!filter.RuleIds.IsNull() && !filter.RuleIds.IsUnknown()) ||
		(!filter.Services.IsNull() && !filter.Services.IsUnknown()) ||
		(!filter.Statuses.IsNull() && !filter.Statuses.IsUnknown())
}

// Check if any scalar-type filter field has a value
func hasAnyScalarFilterValue(filter *FilterModel) bool {
	return (!filter.Description.IsNull() && !filter.Description.IsUnknown()) ||
		(!filter.NewerThanDays.IsNull() && !filter.NewerThanDays.IsUnknown()) ||
		(!filter.OlderThanDays.IsNull() && !filter.OlderThanDays.IsUnknown()) ||
		(!filter.ResourceID.IsNull() && !filter.ResourceID.IsUnknown()) ||
		(!filter.ResourceSearchMode.IsNull() && !filter.ResourceSearchMode.IsUnknown()) ||
		(!filter.Suppressed.IsNull() && !filter.Suppressed.IsUnknown())
}

// Check if any filter field has a non-null/non-unknown value
func hasAnyFilterValue(filter *FilterModel) bool {
	return hasAnySetFilterValue(filter) || hasAnyScalarFilterValue(filter)
}

// Convert schedule from plan to DTO for update requests
func convertScheduleForUpdate(plan *ReportConfigResourceModel) *cloud_risk_management_dto.ReportSchedule {
	if len(plan.Schedule) == 0 {
		return nil
	}

	schedule := plan.Schedule[0]
	hasFrequency := !schedule.Frequency.IsNull() && schedule.Frequency.ValueString() != ""
	hasTimezone := !schedule.Timezone.IsNull() && schedule.Timezone.ValueString() != ""
	hasEnabled := !schedule.Enabled.IsNull()

	if !hasFrequency && !hasTimezone && !hasEnabled {
		return nil
	}

	dto := &cloud_risk_management_dto.ReportSchedule{}
	if hasFrequency {
		dto.Frequency = schedule.Frequency.ValueString()
	}
	if hasTimezone {
		dto.Timezone = schedule.Timezone.ValueString()
	}
	if hasEnabled {
		enabled := schedule.Enabled.ValueBool()
		dto.Enabled = &enabled
	}
	return dto
}

// Set the basic top-level fields for update request
func setBasicUpdateFields(req *cloud_risk_management_dto.UpdateReportConfigRequest, plan *ReportConfigResourceModel) {
	req.ReportTitle = plan.ReportTitle.ValueString()

	if !plan.ReportType.IsNull() {
		req.ReportType = plan.ReportType.ValueString()
	}

	if !plan.IncludeChecks.IsNull() {
		val := plan.IncludeChecks.ValueBool()
		req.IncludeChecks = &val
	}

	if plan.AccountID.IsNull() && !plan.IncludeAccountNames.IsNull() {
		val := plan.IncludeAccountNames.ValueBool()
		req.IncludeAccountNames = &val
	}

	if !plan.Language.IsNull() {
		req.Language = plan.Language.ValueString()
	}
}

// Convert email-related fields for update request
func setEmailFieldsForUpdate(ctx context.Context, req *cloud_risk_management_dto.UpdateReportConfigRequest, plan *ReportConfigResourceModel) error {
	if !plan.EmailRecipients.IsNull() && !plan.EmailRecipients.IsUnknown() {
		var emails []string
		diags := plan.EmailRecipients.ElementsAs(ctx, &emails, false)
		if diags.HasError() {
			return fmt.Errorf("failed to convert email recipients: %v", diags)
		}
		req.EmailRecipients = emails
	}

	if !plan.ReportFormatsInEmail.IsNull() && !plan.ReportFormatsInEmail.IsUnknown() {
		var formats []string
		diags := plan.ReportFormatsInEmail.ElementsAs(ctx, &formats, false)
		if diags.HasError() {
			return fmt.Errorf("failed to convert report formats: %v", diags)
		}
		req.ReportFormatsInEmail = formats
	}
	return nil
}

// Set compliance standard specific fields
func setComplianceFieldsForUpdate(req *cloud_risk_management_dto.UpdateReportConfigRequest, plan *ReportConfigResourceModel) {
	if plan.ReportType.IsNull() || plan.ReportType.ValueString() != ReportTypeComplianceStandard {
		return
	}

	if !plan.AppliedComplianceStandardID.IsNull() {
		req.AppliedComplianceStandardID = plan.AppliedComplianceStandardID.ValueString()
	}
	if !plan.ControlsType.IsNull() {
		req.ControlsType = plan.ControlsType.ValueString()
	}
}

// Converts Terraform model to DTO update request
// The function uses pointer types in the DTO to send explicit null values for cleared fields
func ConvertToUpdateReportConfigRequest(ctx context.Context, plan, state *ReportConfigResourceModel) (cloud_risk_management_dto.UpdateReportConfigRequest, error) {
	req := cloud_risk_management_dto.UpdateReportConfigRequest{}

	setBasicUpdateFields(&req, plan)

	if err := setEmailFieldsForUpdate(ctx, &req, plan); err != nil {
		return req, err
	}

	req.Schedule = convertScheduleForUpdate(plan)
	setComplianceFieldsForUpdate(&req, plan)

	// Filter handling (follows same pattern as schedule):
	// 1. Block omitted from config → preserves existing
	// 2. Block present with at least one value → updates it
	// 3. Block present but all fields empty → preserves existing
	if len(plan.ChecksFilter) > 0 && hasAnyFilterValue(&plan.ChecksFilter[0]) {
		filterDTO, err := convertFilterToDTO(ctx, &plan.ChecksFilter[0])
		if err != nil {
			return req, fmt.Errorf("failed to convert filter: %w", err)
		}
		req.ChecksFilter = filterDTO
	}

	return req, nil
}

// Helper functions for updating state from report config
func updateReportFormatsInEmail(ctx context.Context, state *ReportConfigResourceModel, reportConfig *cloud_risk_management_dto.ReportConfig) {
	// If API returns no formats, set to null
	if len(reportConfig.ReportFormatsInEmail) == 0 {
		state.ReportFormatsInEmail = types.SetNull(types.StringType)
		return
	}

	apiFormats := reportConfig.ReportFormatsInEmail

	// Handle API normalization: ["PDF", "CSV"] is equivalent to ["all"]
	// Only do this if state already has a value (not during import)
	if !state.ReportFormatsInEmail.IsNull() {
		if len(apiFormats) == 1 && apiFormats[0] == "all" {
			var plannedFormats []string
			diags := state.ReportFormatsInEmail.ElementsAs(ctx, &plannedFormats, false)
			if !diags.HasError() {
				if len(plannedFormats) == 2 && containsAll(plannedFormats, []string{"PDF", "CSV"}) {
					apiFormats = plannedFormats
				}
			}
		}

		// Also handle reverse: if API returned ["PDF", "CSV"] but state has ["all"]
		if len(apiFormats) == 2 && containsAll(apiFormats, []string{"PDF", "CSV"}) {
			var plannedFormats []string
			diags := state.ReportFormatsInEmail.ElementsAs(ctx, &plannedFormats, false)
			if !diags.HasError() {
				if len(plannedFormats) == 1 && plannedFormats[0] == "all" {
					apiFormats = plannedFormats
				}
			}
		}
	}

	formats, diags := types.SetValueFrom(ctx, types.StringType, apiFormats)
	if diags.HasError() {
		fmt.Printf("Warning: failed to convert report formats from API: %v\n", diags)
	} else {
		state.ReportFormatsInEmail = formats
	}
}

func updateScheduleFromReportConfig(state *ReportConfigResourceModel, reportConfig *cloud_risk_management_dto.ReportConfig) {
	// Schedule is an Optional block (not Computed), so it should only be in state if user configured it
	// Check if user has schedule in their config
	hasScheduleInConfig := len(state.Schedule) > 0

	if !hasScheduleInConfig {
		// User didn't configure schedule - don't populate it even if API returns one
		// (API may preserve old schedule values from previous configurations)
		state.Schedule = nil
		return
	}

	// If API returns no schedule but user had it in config, clear it
	if reportConfig.Schedule == nil {
		state.Schedule = nil
		return
	}

	// Populate schedule from API
	scheduleModel := ScheduleModel{}

	// Populate enabled if API returns it
	if reportConfig.Schedule.Enabled != nil {
		scheduleModel.Enabled = types.BoolValue(*reportConfig.Schedule.Enabled)
	} else {
		scheduleModel.Enabled = types.BoolNull()
	}

	// Populate frequency and timezone if present
	if reportConfig.Schedule.Frequency != "" {
		scheduleModel.Frequency = types.StringValue(reportConfig.Schedule.Frequency)
	} else {
		scheduleModel.Frequency = types.StringNull()
	}

	if reportConfig.Schedule.Timezone != "" {
		scheduleModel.Timezone = types.StringValue(reportConfig.Schedule.Timezone)
	} else {
		scheduleModel.Timezone = types.StringNull()
	}

	state.Schedule = []ScheduleModel{scheduleModel}
}

func updateComplianceStandardFields(state *ReportConfigResourceModel, reportConfig *cloud_risk_management_dto.ReportConfig) {
	// Always populate applied compliance standard ID
	if reportConfig.AppliedComplianceStandardID != "" {
		state.AppliedComplianceStandardID = types.StringValue(reportConfig.AppliedComplianceStandardID)
	} else if reportConfig.AppliedComplianceStandard != nil {
		state.AppliedComplianceStandardID = types.StringValue(reportConfig.AppliedComplianceStandard.ID)
	} else {
		state.AppliedComplianceStandardID = types.StringNull()
	}

	// Always populate controls_type
	if reportConfig.ControlsType != "" {
		state.ControlsType = types.StringValue(reportConfig.ControlsType)
	} else {
		// API default when not specified
		state.ControlsType = types.StringValue("all")
	}
}

// isFilterBlockEmpty checks if all filter fields are null
func isFilterBlockEmpty(filter *FilterModel) bool {
	return filter.Categories.IsNull() &&
		filter.ComplianceStandardIds.IsNull() &&
		filter.Tags.IsNull() &&
		filter.Description.IsNull() &&
		filter.NewerThanDays.IsNull() &&
		filter.OlderThanDays.IsNull() &&
		filter.Providers.IsNull() &&
		filter.Regions.IsNull() &&
		filter.ResourceID.IsNull() &&
		filter.ResourceSearchMode.IsNull() &&
		filter.ResourceTypes.IsNull() &&
		filter.RiskLevels.IsNull() &&
		filter.RuleIds.IsNull() &&
		filter.Services.IsNull() &&
		filter.Statuses.IsNull() &&
		filter.Suppressed.IsNull()
}

// updateBasicStateFields sets the basic ID and top-level fields
func updateBasicStateFields(state *ReportConfigResourceModel, reportConfig *cloud_risk_management_dto.ReportConfig) {
	state.ID = types.StringValue(reportConfig.ID)

	if reportConfig.AccountID != "" {
		state.AccountID = types.StringValue(reportConfig.AccountID)
	}
	if reportConfig.GroupID != "" {
		state.GroupID = types.StringValue(reportConfig.GroupID)
	}

	if reportConfig.Level != "" {
		state.Level = types.StringValue(reportConfig.Level)
	}

	state.ReportTitle = types.StringValue(reportConfig.ReportTitle)
	state.ReportType = types.StringValue(reportConfig.ReportType)
	state.IncludeChecks = types.BoolValue(reportConfig.IncludeChecks)

	if reportConfig.Language != "" {
		state.Language = types.StringValue(reportConfig.Language)
	} else {
		state.Language = types.StringNull()
	}
}

// Handle include_account_names field logic
func updateIncludeAccountNamesField(state *ReportConfigResourceModel, reportConfig *cloud_risk_management_dto.ReportConfig, isCreateOrUpdate bool) {
	if reportConfig.Level != "group" && reportConfig.Level != "company" {
		state.IncludeAccountNames = types.BoolNull()
		return
	}

	shouldPopulate := !isCreateOrUpdate || !state.IncludeAccountNames.IsNull()
	if !shouldPopulate {
		return
	}

	if reportConfig.IncludeAccountNames != nil {
		state.IncludeAccountNames = types.BoolValue(*reportConfig.IncludeAccountNames)
	} else {
		state.IncludeAccountNames = types.BoolValue(true)
	}
}

// Handle email_recipients field
func updateEmailRecipientsField(ctx context.Context, state *ReportConfigResourceModel, reportConfig *cloud_risk_management_dto.ReportConfig) {
	if reportConfig.EmailRecipients != nil {
		emails, diags := types.SetValueFrom(ctx, types.StringType, reportConfig.EmailRecipients)
		if diags.HasError() {
			fmt.Printf("Warning: failed to convert email recipients from API: %v\n", diags)
		} else {
			state.EmailRecipients = emails
		}
	} else {
		state.EmailRecipients = types.SetNull(types.StringType)
	}
}

// Handle checks_filter field logic
func updateFilterField(ctx context.Context, state *ReportConfigResourceModel, reportConfig *cloud_risk_management_dto.ReportConfig, isCreateOrUpdate bool) {
	if reportConfig.ChecksFilter == nil {
		state.ChecksFilter = nil
		return
	}

	shouldPopulateFilter := !isCreateOrUpdate || len(state.ChecksFilter) > 0

	// If user specified an empty checks_filter {} block, don't populate from API
	if shouldPopulateFilter && len(state.ChecksFilter) > 0 && isFilterBlockEmpty(&state.ChecksFilter[0]) {
		shouldPopulateFilter = false
	}

	if shouldPopulateFilter {
		if len(state.ChecksFilter) == 0 {
			state.ChecksFilter = []FilterModel{{}}
		}
		updateFilterFromDTO(ctx, &state.ChecksFilter[0], reportConfig.ChecksFilter, reportConfig.ReportType)
	}
}

// Updates the Terraform state from API response
func UpdateStateFromReportConfig(ctx context.Context, state *ReportConfigResourceModel, reportConfig *cloud_risk_management_dto.ReportConfig) {
	isCreateOrUpdate := state.Level.IsNull() || state.Level.ValueString() == ""

	updateBasicStateFields(state, reportConfig)
	updateIncludeAccountNamesField(state, reportConfig, isCreateOrUpdate)
	updateEmailRecipientsField(ctx, state, reportConfig)
	updateReportFormatsInEmail(ctx, state, reportConfig)
	updateScheduleFromReportConfig(state, reportConfig)

	// Compliance Standard specific
	if reportConfig.ReportType == ReportTypeComplianceStandard {
		updateComplianceStandardFields(state, reportConfig)
	} else {
		state.ControlsType = types.StringNull()
		state.AppliedComplianceStandardID = types.StringNull()
	}

	updateFilterField(ctx, state, reportConfig, isCreateOrUpdate)
}

// Helper functions for converting filter fields
func convertSetToStringPointer(ctx context.Context, set types.Set, fieldName string) (*[]string, error) {
	if !set.IsNull() && !set.IsUnknown() {
		var items []string
		diags := set.ElementsAs(ctx, &items, false)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert %s: %v", fieldName, diags)
		}
		return &items, nil
	} else if !set.IsUnknown() {
		return nil, nil
	}
	return nil, nil
}

func convertStringToPointer(value types.String) *string {
	if !value.IsNull() && !value.IsUnknown() {
		str := value.ValueString()
		return &str
	} else if !value.IsUnknown() {
		return nil
	}
	return nil
}

func convertInt64ToIntPointer(value types.Int64) *int {
	if !value.IsNull() && !value.IsUnknown() {
		val := int(value.ValueInt64())
		return &val
	} else if !value.IsUnknown() {
		return nil
	}
	return nil
}

func convertBoolToPointer(value types.Bool) *bool {
	if !value.IsNull() && !value.IsUnknown() {
		val := value.ValueBool()
		return &val
	}
	return nil
}

// Helper functions
func convertFilterToDTO(ctx context.Context, filter *FilterModel) (*cloud_risk_management_dto.ReportConfigFilter, error) {
	dto := &cloud_risk_management_dto.ReportConfigFilter{}
	var err error

	// Convert set fields
	dto.Categories, err = convertSetToStringPointer(ctx, filter.Categories, "categories")
	if err != nil {
		return nil, err
	}

	dto.ComplianceStandardIds, err = convertSetToStringPointer(ctx, filter.ComplianceStandardIds, "compliance standard IDs")
	if err != nil {
		return nil, err
	}

	dto.Tags, err = convertSetToStringPointer(ctx, filter.Tags, "tags")
	if err != nil {
		return nil, err
	}

	dto.Providers, err = convertSetToStringPointer(ctx, filter.Providers, "providers")
	if err != nil {
		return nil, err
	}

	dto.Regions, err = convertSetToStringPointer(ctx, filter.Regions, "regions")
	if err != nil {
		return nil, err
	}

	dto.ResourceTypes, err = convertSetToStringPointer(ctx, filter.ResourceTypes, "resource types")
	if err != nil {
		return nil, err
	}

	dto.RiskLevels, err = convertSetToStringPointer(ctx, filter.RiskLevels, "risk levels")
	if err != nil {
		return nil, err
	}

	dto.RuleIds, err = convertSetToStringPointer(ctx, filter.RuleIds, "rule IDs")
	if err != nil {
		return nil, err
	}

	dto.Services, err = convertSetToStringPointer(ctx, filter.Services, "services")
	if err != nil {
		return nil, err
	}

	dto.Statuses, err = convertSetToStringPointer(ctx, filter.Statuses, "statuses")
	if err != nil {
		return nil, err
	}

	// Convert string fields
	dto.Description = convertStringToPointer(filter.Description)
	dto.ResourceID = convertStringToPointer(filter.ResourceID)
	dto.ResourceSearchMode = convertStringToPointer(filter.ResourceSearchMode)

	// Convert int fields
	dto.NewerThanDays = convertInt64ToIntPointer(filter.NewerThanDays)
	dto.OlderThanDays = convertInt64ToIntPointer(filter.OlderThanDays)

	// Convert bool field
	dto.Suppressed = convertBoolToPointer(filter.Suppressed)

	return dto, nil
}

// Helper functions for updating filter fields from DTO
func updateSetFromStringPointer(ctx context.Context, target *types.Set, source *[]string) {
	if source != nil {
		// API returned a value - update the target
		if len(*source) > 0 {
			value, diags := types.SetValueFrom(ctx, types.StringType, *source)
			if diags.HasError() {
				fmt.Printf("Warning: failed to convert set from API: %v\n", diags)
			} else {
				*target = value
			}
		} else {
			// API returned empty array explicitly
			*target = types.SetNull(types.StringType)
		}
	} else {
		// API didn't return the field - set to properly typed null
		// This ensures all Sets have correct element type
		*target = types.SetNull(types.StringType)
	}
}

func updateStringFromPointer(target *types.String, source *string) {
	if source != nil {
		// API returned a value
		if *source != "" {
			*target = types.StringValue(*source)
		} else {
			// API returned empty string explicitly
			*target = types.StringNull()
		}
	} else {
		// API didn't return the field - set to null
		*target = types.StringNull()
	}
}

func updateInt64FromIntPointer(target *types.Int64, source *int) {
	if source != nil {
		// API returned a value
		*target = types.Int64Value(int64(*source))
	} else {
		// API didn't return the field - set to null
		*target = types.Int64Null()
	}
}

func updateBoolFromPointer(target *types.Bool, source *bool) {
	if source != nil {
		// API returned a value
		*target = types.BoolValue(*source)
	} else {
		// API didn't return the field - set to null
		*target = types.BoolNull()
	}
}

func updateFilterFromDTO(ctx context.Context, filter *FilterModel, dto *cloud_risk_management_dto.ReportConfigFilter, reportType string) {
	// Update set fields
	updateSetFromStringPointer(ctx, &filter.Categories, dto.Categories)

	// Handle compliance_standard_ids: Only for GENERIC reports
	// For COMPLIANCE-STANDARD reports, compliance standard is specified at top-level, not in filter
	if reportType == "GENERIC" {
		// API returns complianceStandards (array of objects), not complianceStandardIds (array of strings)
		// Extract IDs from the objects
		if dto.ComplianceStandards != nil && len(*dto.ComplianceStandards) > 0 {
			ids := make([]string, 0, len(*dto.ComplianceStandards))
			for _, std := range *dto.ComplianceStandards {
				ids = append(ids, std.ID)
			}
			updateSetFromStringPointer(ctx, &filter.ComplianceStandardIds, &ids)
		} else {
			// Fallback to direct ComplianceStandardIds if present (shouldn't happen with current API)
			updateSetFromStringPointer(ctx, &filter.ComplianceStandardIds, dto.ComplianceStandardIds)
		}
	} else {
		// For COMPLIANCE-STANDARD reports, always set compliance_standard_ids to null
		filter.ComplianceStandardIds = types.SetNull(types.StringType)
	}

	updateSetFromStringPointer(ctx, &filter.Tags, dto.Tags)
	updateSetFromStringPointer(ctx, &filter.Providers, dto.Providers)
	updateSetFromStringPointer(ctx, &filter.Regions, dto.Regions)
	updateSetFromStringPointer(ctx, &filter.ResourceTypes, dto.ResourceTypes)
	updateSetFromStringPointer(ctx, &filter.RiskLevels, dto.RiskLevels)
	updateSetFromStringPointer(ctx, &filter.RuleIds, dto.RuleIds)
	updateSetFromStringPointer(ctx, &filter.Services, dto.Services)
	updateSetFromStringPointer(ctx, &filter.Statuses, dto.Statuses)

	// Update string fields
	updateStringFromPointer(&filter.Description, dto.Description)
	updateStringFromPointer(&filter.ResourceID, dto.ResourceID)
	updateStringFromPointer(&filter.ResourceSearchMode, dto.ResourceSearchMode)

	// Update int fields
	updateInt64FromIntPointer(&filter.NewerThanDays, dto.NewerThanDays)
	updateInt64FromIntPointer(&filter.OlderThanDays, dto.OlderThanDays)

	// Update bool field
	updateBoolFromPointer(&filter.Suppressed, dto.Suppressed)
}

// Checks if slice 'slice' contains all elements from 'items'
func containsAll(slice, items []string) bool {
	if len(items) == 0 {
		return true
	}
	if len(slice) < len(items) {
		return false
	}

	found := make(map[string]bool)
	for _, item := range slice {
		found[item] = true
	}

	for _, item := range items {
		if !found[item] {
			return false
		}
	}
	return true
}
