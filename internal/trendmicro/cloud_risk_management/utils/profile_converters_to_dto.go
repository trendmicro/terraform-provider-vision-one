package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// =============================================================================
// Type Helper Functions
// =============================================================================

// IsNumericType returns true if the setting type should use numeric values.
func IsNumericType(settingType string) bool {
	return settingType == "ttl" ||
		settingType == "single-number-value" ||
		settingType == "multiple-number-values"
}

// IsValueSetType returns true if the setting type should use value_set (simple string array).
// These types support the simplified value_set syntax instead of the values block.
func IsValueSetType(settingType string) bool {
	return settingType == "multiple-string-values" ||
		settingType == "multiple-ip-values" ||
		settingType == "multiple-aws-account-values" ||
		settingType == "multiple-number-values" ||
		settingType == "regions" ||
		settingType == "ignored-regions" ||
		settingType == "tags" ||
		settingType == "countries"
}

// ConvertStringToNumber converts a string to a number (int64 or float64).
// Returns the numeric value and true if successful, or the original string and false if not.
func ConvertStringToNumber(s string) (any, bool) {
	// Try parsing as integer first
	if intVal, err := strconv.ParseInt(s, 10, 64); err == nil {
		return intVal, true
	}
	// Try parsing as float
	if floatVal, err := strconv.ParseFloat(s, 64); err == nil {
		return floatVal, true
	}
	return s, false
}

// ConvertValueToString converts an interface{} value to a string.
// Handles string, float64 (from JSON), int, and int64 types.
// Returns the string value and a boolean indicating if conversion was successful.
func ConvertValueToString(val interface{}) (string, bool) {
	if val == nil {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	case float64:
		// Handle numeric values - convert to string without decimal for integers
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return fmt.Sprintf("%g", v), true
	case int:
		return fmt.Sprintf("%d", v), true
	case int64:
		return fmt.Sprintf("%d", v), true
	default:
		return "", false
	}
}

// =============================================================================
// Model to DTO Converters (Terraform Plan -> API Request)
// =============================================================================

// ConvertScanRulesToDTO converts a slice of ScanRuleModel from the Terraform plan
// to a slice of ScanRule DTOs for API requests.
func ConvertScanRulesToDTO(_ context.Context, rules []ScanRuleModel) ([]cloud_risk_management_dto.ScanRule, error) {
	result := make([]cloud_risk_management_dto.ScanRule, len(rules))

	for i, rule := range rules {
		result[i] = cloud_risk_management_dto.ScanRule{
			ID:        rule.ID.ValueString(),
			Provider:  rule.Provider.ValueString(),
			Enabled:   rule.Enabled.ValueBool(),
			RiskLevel: rule.RiskLevel.ValueString(),
		}

		// Convert exceptions - only create if there are actual values
		if rule.Exceptions != nil && (len(rule.Exceptions.FilterTags) > 0 || len(rule.Exceptions.ResourceIds) > 0) {
			result[i].Exceptions = &cloud_risk_management_dto.RuleExceptions{}

			if len(rule.Exceptions.FilterTags) > 0 {
				filterTags := make([]string, len(rule.Exceptions.FilterTags))
				for j, ft := range rule.Exceptions.FilterTags {
					filterTags[j] = ft.ValueString()
				}
				result[i].Exceptions.FilterTags = filterTags
			}

			if len(rule.Exceptions.ResourceIds) > 0 {
				resourceIds := make([]string, len(rule.Exceptions.ResourceIds))
				for j, rid := range rule.Exceptions.ResourceIds {
					resourceIds[j] = rid.ValueString()
				}
				result[i].Exceptions.ResourceIds = resourceIds
			}
		}

		// Convert extra settings
		if len(rule.ExtraSettings) > 0 {
			extraSettings, err := ConvertExtraSettingsToDTO(rule.ExtraSettings)
			if err != nil {
				return nil, err
			}
			result[i].ExtraSettings = extraSettings
		}
	}

	return result, nil
}

// ConvertExtraSettingsToDTO converts a slice of ExtraSettingModel from the Terraform plan
// to a slice of RuleExtraSetting DTOs for API requests. It handles all setting types.
func ConvertExtraSettingsToDTO(settings []ExtraSettingModel) ([]cloud_risk_management_dto.RuleExtraSetting, error) {
	extraSettings := make([]cloud_risk_management_dto.RuleExtraSetting, len(settings))
	var err error

	for i := range settings {
		es := settings[i]
		extraSettings[i], err = ConvertExtraSettingToDTO(&es)
		if err != nil {
			return nil, err
		}
	}

	return extraSettings, nil
}

// ConvertExtraSettingToDTO converts a single ExtraSettingModel from the Terraform plan
// to a RuleExtraSetting DTO for API requests. It handles different setting types:
// - multiple-object-values: JSON objects with nested settings
// - choice-multiple-value: options with enabled/customised flags
// - value_set types: simple string arrays (regions, tags, IPs, etc.)
// - numeric types: converts string values to numbers
func ConvertExtraSettingToDTO(setting *ExtraSettingModel) (cloud_risk_management_dto.RuleExtraSetting, error) {
	result := cloud_risk_management_dto.RuleExtraSetting{
		Name: setting.Name.ValueString(),
		Type: setting.Type.ValueString(),
	}

	if !setting.Value.IsNull() {
		valueStr := setting.Value.ValueString()
		// For numeric types, convert to number
		if IsNumericType(setting.Type.ValueString()) {
			if numVal, ok := ConvertStringToNumber(valueStr); ok {
				result.Value = numVal
			} else {
				result.Value = valueStr
			}
		} else {
			result.Value = valueStr
		}
	}

	settingType := setting.Type.ValueString()
	switch settingType {
	case multipleObjectValuesType:
		convertMultipleObjectValuesToDTO(setting, &result)
	case choiceMultipleValueType:
		convertChoiceMultipleValueToDTO(setting, &result)
	default:
		convertDefaultTypeToDTO(setting, settingType, &result)
	}
	return result, nil
}

// convertMultipleObjectValuesToDTO handles the multiple-object-values type conversion.
// It handles both JSON parsing and nested settings.
func convertMultipleObjectValuesToDTO(setting *ExtraSettingModel, result *cloud_risk_management_dto.RuleExtraSetting) {
	if setting.Values == nil {
		return
	}

	result.Values = []any{}

	for i := range setting.Values {
		v := setting.Values[i]
		valuesMap := map[string]any{}

		// Handle value field
		if !v.Value.IsNull() && v.Value.ValueString() != "" {
			// Try parsing as JSON first
			var jsonValue interface{}
			if err := json.Unmarshal([]byte(v.Value.ValueString()), &jsonValue); err == nil {
				// If it's a JSON object (map), convert to pairs format for API
				if objMap, ok := jsonValue.(map[string]interface{}); ok {
					// Sort keys to ensure consistent ordering
					keys := make([]string, 0, len(objMap))
					for key := range objMap {
						keys = append(keys, key)
					}
					sort.Strings(keys)

					pairs := []map[string]string{}
					for _, key := range keys {
						val := objMap[key]
						// Convert value to string
						valStr := ""
						if strVal, ok := ConvertValueToString(val); ok {
							valStr = strVal
						}
						pairs = append(pairs, map[string]string{
							"key":   key,
							"value": valStr,
						})
					}
					valuesMap["pairs"] = pairs
				} else {
					// For non-object JSON values, use as-is
					valuesMap["value"] = jsonValue
				}
			} else {
				// If not valid JSON, use as plain string
				valuesMap["value"] = v.Value.ValueString()
			}
		}

		result.Values = append(result.Values, valuesMap)
	}
}

// convertSettingValuesToDTO is a generic helper that converts values for both ExtraSettingModel and NestedSettingsModel.
// It handles choice-multiple-value, value_set types, and default types.
// The hasNestedSettings parameter indicates if the values can contain nested settings (only for ExtraSettingsValuesObjectModel).
func convertSettingValuesToDTO(
	settingType string,
	valueSetInput []types.String,
	valuesInput interface{}, // []ExtraSettingsValuesObjectModel or []NestedSettingsValuesObjectModel
	hasNestedSettings bool,
) []any {
	var result []any

	// Handle value_set for simple types
	if IsValueSetType(settingType) && len(valueSetInput) > 0 {
		return convertValueSetToArray(valueSetInput, settingType)
	}

	// Handle values block
	switch v := valuesInput.(type) {
	case []ExtraSettingsValuesObjectModel:
		if v == nil {
			return nil
		}
		result = []any{}
		for i := range v {
			val := v[i]
			valuesMap := convertValuesObjectToMap(val.Value, val.Enabled, val.Customised, val.Severity, val.VpcId, val.GatewayIds, settingType)
			if hasNestedSettings && len(val.Settings) > 0 {
				valuesMap["settings"] = convertNestedSettingsToDTO(val.Settings)
			}
			result = append(result, valuesMap)
		}
	case []NestedSettingsValuesObjectModel:
		if v == nil {
			return nil
		}
		result = []any{}
		for _, val := range v {
			valuesMap := convertValuesObjectToMap(val.Value, val.Enabled, val.Customised, val.Severity, val.VpcId, val.GatewayIds, settingType)
			result = append(result, valuesMap)
		}
	}

	return result
}

// convertValuesObjectToMap converts common fields from a values object to a map.
// Handles value, enabled, customised, severity, vpcId, and gatewayIds fields.
func convertValuesObjectToMap(
	value types.String,
	enabled types.Bool,
	customised types.Bool,
	severity types.String,
	vpcId types.String,
	gatewayIds []types.String,
	settingType string,
) map[string]any {
	valuesMap := map[string]any{}

	// Handle value field - only include if not null
	if !value.IsNull() {
		valueStr := value.ValueString()
		var valueField any = valueStr

		// For numeric types, convert value to number
		if IsNumericType(settingType) {
			if numVal, ok := ConvertStringToNumber(valueStr); ok {
				valueField = numVal
			}
		}

		valuesMap["value"] = valueField
	}

	// Only include enabled if explicitly specified
	if !enabled.IsNull() {
		valuesMap["enabled"] = enabled.ValueBool()
	}

	// Only include customised if explicitly specified
	if !customised.IsNull() {
		valuesMap["customised"] = customised.ValueBool()
	}

	// Only include severity if explicitly specified
	if !severity.IsNull() {
		valuesMap["severity"] = severity.ValueString()
	}

	// Handle vpc_id field
	if !vpcId.IsNull() {
		valuesMap["vpcId"] = vpcId.ValueString()
	}

	// Handle gateway_ids field
	if len(gatewayIds) > 0 {
		gwIds := make([]string, len(gatewayIds))
		for k, gid := range gatewayIds {
			gwIds[k] = gid.ValueString()
		}
		valuesMap["gatewayIds"] = gwIds
	}

	return valuesMap
}

// convertChoiceMultipleValueToDTO handles the choice-multiple-value type conversion.
// Only includes enabled/customised if explicitly specified (not null).
func convertChoiceMultipleValueToDTO(setting *ExtraSettingModel, result *cloud_risk_management_dto.RuleExtraSetting) {
	result.Values = convertSettingValuesToDTO(setting.Type.ValueString(), setting.ValueSet, setting.Values, true)
}

// convertDefaultTypeToDTO handles the default type conversion including value_set types.
func convertDefaultTypeToDTO(setting *ExtraSettingModel, settingType string, result *cloud_risk_management_dto.RuleExtraSetting) {
	result.Values = convertSettingValuesToDTO(settingType, setting.ValueSet, setting.Values, false)
}

// convertNestedSettingsToDTO converts nested settings to a slice of maps for the DTO.
func convertNestedSettingsToDTO(settings []NestedSettingsModel) []any {
	settingsArray := []any{}
	for _, nestedSetting := range settings {
		settingMap := map[string]any{
			"name": nestedSetting.Name.ValueString(),
			"type": nestedSetting.Type.ValueString(),
		}

		// Handle value field
		if !nestedSetting.Value.IsNull() {
			valueStr := nestedSetting.Value.ValueString()
			// For numeric types, convert to number
			if IsNumericType(nestedSetting.Type.ValueString()) {
				if numVal, ok := ConvertStringToNumber(valueStr); ok {
					settingMap["value"] = numVal
				} else {
					settingMap["value"] = valueStr
				}
			} else {
				settingMap["value"] = valueStr
			}
		}

		// Handle different setting types (similar to ConvertExtraSettingToDTO)
		settingType := nestedSetting.Type.ValueString()
		switch settingType {
		case choiceMultipleValueType:
			convertNestedChoiceMultipleValueToDTO(&nestedSetting, settingMap)
		default:
			convertNestedDefaultTypeToDTO(&nestedSetting, settingType, settingMap)
		}

		settingsArray = append(settingsArray, settingMap)
	}
	return settingsArray
}

// convertNestedChoiceMultipleValueToDTO handles choice-multiple-value type for nested settings
func convertNestedChoiceMultipleValueToDTO(setting *NestedSettingsModel, settingMap map[string]any) {
	if values := convertSettingValuesToDTO(setting.Type.ValueString(), setting.ValueSet, setting.Values, false); values != nil {
		settingMap["values"] = values
	}
}

// convertNestedDefaultTypeToDTO handles default types for nested settings (including value_set types)
func convertNestedDefaultTypeToDTO(setting *NestedSettingsModel, settingType string, settingMap map[string]any) {
	if values := convertSettingValuesToDTO(settingType, setting.ValueSet, setting.Values, false); values != nil {
		settingMap["values"] = values
	}
}

// convertValueSetToArray converts a value_set to an array of value objects for the API
func convertValueSetToArray(valueSet []types.String, settingType string) []any {
	valuesArr := []any{}
	for _, vs := range valueSet {
		valueStr := vs.ValueString()
		var valueField any = valueStr

		// For numeric types, convert string to number
		if IsNumericType(settingType) {
			if numVal, ok := ConvertStringToNumber(valueStr); ok {
				valueField = numVal
			}
		}

		valuesArr = append(valuesArr, map[string]any{
			"value": valueField,
		})
	}
	return valuesArr
}
