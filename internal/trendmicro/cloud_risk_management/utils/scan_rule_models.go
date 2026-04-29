package utils

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ScanRuleModel represents a scan rule configuration shared across profile and account scan rules.
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
	Value               types.String   `tfsdk:"value"`
	Enabled             types.Bool     `tfsdk:"enabled"`
	VpcId               types.String   `tfsdk:"vpc_id"`
	GatewayIds          []types.String `tfsdk:"gateway_ids"`
	CustomizedTags      types.Set      `tfsdk:"customized_tags"`
	CustomizedRiskLevel types.String   `tfsdk:"customized_risk_level"`
}
