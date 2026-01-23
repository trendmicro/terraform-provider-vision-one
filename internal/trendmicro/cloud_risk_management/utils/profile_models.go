package utils

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ProfileResourceModel represents the Terraform resource model for a CRM profile.
type ProfileResourceModel struct {
	ID          types.String    `tfsdk:"id"`
	Name        types.String    `tfsdk:"name"`
	Description types.String    `tfsdk:"description"`
	ScanRules   []ScanRuleModel `tfsdk:"scan_rule"`
}

// ScanRuleModel represents a scan rule configuration within a profile.
type ScanRuleModel struct {
	ID            types.String         `tfsdk:"id"`
	Provider      types.String         `tfsdk:"provider"`
	Enabled       types.Bool           `tfsdk:"enabled"`
	RiskLevel     types.String         `tfsdk:"risk_level"`
	Exceptions    *RuleExceptionsModel `tfsdk:"exceptions"`
	ExtraSettings []ExtraSettingModel  `tfsdk:"extra_settings"`
}

// RuleExceptionsModel represents rule exceptions for filtering resources.
type RuleExceptionsModel struct {
	FilterTags  []types.String `tfsdk:"filter_tags"`
	ResourceIds []types.String `tfsdk:"resource_ids"`
}

// ExtraSettingModel represents additional settings for a scan rule.
// It supports multiple setting types including simple values, value sets,
// choice values, and complex object values
type ExtraSettingModel struct {
	Name     types.String                     `tfsdk:"name"`
	Type     types.String                     `tfsdk:"type"`
	Value    types.String                     `tfsdk:"value"`
	Values   []ExtraSettingsValuesObjectModel `tfsdk:"values"`
	ValueSet []types.String                   `tfsdk:"value_set"`
}

// ExtraSettingsValuesObjectModel represents a value object within extra settings.
type ExtraSettingsValuesObjectModel struct {
	Value      types.String          `tfsdk:"value"`
	Enabled    types.Bool            `tfsdk:"enabled"`
	Customised types.Bool            `tfsdk:"customised"`
	Severity   types.String          `tfsdk:"severity"`
	Settings   []NestedSettingsModel `tfsdk:"settings"`
	VpcId      types.String          `tfsdk:"vpc_id"`
	GatewayIds []types.String        `tfsdk:"gateway_ids"`
}

// NestedSettingsModel represents nested settings within a value object,
// used for choice-multiple-values type.
type NestedSettingsModel struct {
	Name     types.String                      `tfsdk:"name"`
	Type     types.String                      `tfsdk:"type"`
	Value    types.String                      `tfsdk:"value"`
	Values   []NestedSettingsValuesObjectModel `tfsdk:"values"`
	ValueSet []types.String                    `tfsdk:"value_set"`
}

type NestedSettingsValuesObjectModel struct {
	Value      types.String   `tfsdk:"value"`
	Enabled    types.Bool     `tfsdk:"enabled"`
	Customised types.Bool     `tfsdk:"customised"`
	Severity   types.String   `tfsdk:"severity"`
	VpcId      types.String   `tfsdk:"vpc_id"`
	GatewayIds []types.String `tfsdk:"gateway_ids"`
}
