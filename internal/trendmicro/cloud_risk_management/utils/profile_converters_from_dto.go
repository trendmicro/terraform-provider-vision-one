package utils

import (
	"encoding/json"
	"sort"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// =============================================================================
// DTO to Model Converters (API Response -> Terraform State)
// =============================================================================
const (
	multipleObjectValuesType        = "multiple-object-values"
	choiceMultipleValueType         = "choice-multiple-value"
	ttlType                         = "ttl"
	numericTypeMultipleNumberValues = "multiple-number-values"
)

// UpdatePlanFromProfile updates the Terraform plan/state model with data from the API response.
// It preserves original plan values where the API doesn't return certain fields (like exceptions)
// and maintains consistency between the user's configuration and the actual state.
func UpdatePlanFromProfile(plan *ProfileResourceModel, profile *cloud_risk_management_dto.Profile) {
	// Create a map of original plan's scan rules by ID for reference
	originalExceptions := make(map[string]*RuleExceptionsModel)
	// Create a map of original extra_settings by rule ID -> setting name
	originalExtraSettings := make(map[string]map[string]*ExtraSettingModel)
	for _, rule := range plan.ScanRules {
		// Preserve exceptions to maintain user's config structure (even if empty)
		if rule.Exceptions != nil {
			originalExceptions[rule.ID.ValueString()] = rule.Exceptions
		}
		if len(rule.ExtraSettings) > 0 {
			ruleID := rule.ID.ValueString()
			originalExtraSettings[ruleID] = make(map[string]*ExtraSettingModel)
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
		plan.ScanRules = make([]ScanRuleModel, len(profile.ScanRules))
		for i, rule := range profile.ScanRules {
			plan.ScanRules[i] = ScanRuleModel{
				ID:        types.StringValue(rule.ID),
				Provider:  types.StringValue(rule.Provider),
				Enabled:   types.BoolValue(rule.Enabled),
				RiskLevel: types.StringValue(rule.RiskLevel),
			}

			// Convert exceptions: start with user's original (preserves nil vs empty), then override with API values
			if originalExc := originalExceptions[rule.ID]; originalExc != nil {
				plan.ScanRules[i].Exceptions = &RuleExceptionsModel{
					FilterTags:  originalExc.FilterTags,
					ResourceIds: originalExc.ResourceIds,
				}
			}
			if rule.Exceptions != nil {
				if plan.ScanRules[i].Exceptions == nil {
					plan.ScanRules[i].Exceptions = &RuleExceptionsModel{}
				}

				if len(rule.Exceptions.FilterTags) > 0 {
					filterTags := make([]types.String, len(rule.Exceptions.FilterTags))
					for j, ft := range rule.Exceptions.FilterTags {
						filterTags[j] = types.StringValue(ft)
					}
					plan.ScanRules[i].Exceptions.FilterTags = filterTags
				}

				if len(rule.Exceptions.ResourceIds) > 0 {
					resourceIds := make([]types.String, len(rule.Exceptions.ResourceIds))
					for j, rid := range rule.Exceptions.ResourceIds {
						resourceIds[j] = types.StringValue(rid)
					}
					plan.ScanRules[i].Exceptions.ResourceIds = resourceIds
				}
			}

			// Convert extra settings - always convert to match plan structure
			if len(rule.ExtraSettings) > 0 {
				plan.ScanRules[i].ExtraSettings = ConvertExtraSettingsFromDTO(rule.ExtraSettings, originalExtraSettings[rule.ID])
			} else {
				// Ensure ExtraSettings is an empty slice (not nil) to match schema
				plan.ScanRules[i].ExtraSettings = []ExtraSettingModel{}
			}
		}
	}
}

// ConvertExtraSettingsFromDTO converts a slice of RuleExtraSetting DTOs from the API response
// to a slice of ExtraSettingModel for the Terraform state. It uses originalSettings to preserve
// user-specified values that the API may not return.
func ConvertExtraSettingsFromDTO(settings []cloud_risk_management_dto.RuleExtraSetting, originalSettings map[string]*ExtraSettingModel) []ExtraSettingModel {
	extraSettings := make([]ExtraSettingModel, len(settings))

	for i, es := range settings {
		// Get original setting if available
		var origSetting *ExtraSettingModel
		if originalSettings != nil {
			origSetting = originalSettings[es.Name]
		}
		extraSettings[i] = ConvertSingleSettingFromDTO(&es, origSetting)
	}

	return extraSettings
}

// ConvertSingleSettingFromDTO converts a single RuleExtraSetting DTO from the API response
// to an ExtraSettingModel for the Terraform state. It handles different setting types:
// - multiple-object-values: JSON objects
// - choice-multiple-value: options with enabled flags
// - value_set types: populates ValueSet if the original plan used it
// - numeric types: converts numbers back to strings
// The origSetting parameter is used to preserve user-specified values and determine
// whether to use value_set or values block in the state.
func ConvertSingleSettingFromDTO(es *cloud_risk_management_dto.RuleExtraSetting, origSetting *ExtraSettingModel) ExtraSettingModel {
	result := ExtraSettingModel{
		Name: types.StringValue(es.Name),
		Type: types.StringValue(es.Type),
	}

	if strVal, ok := ConvertValueToString(es.Value); ok {
		result.Value = types.StringValue(strVal)
	} else {
		result.Value = types.StringNull()
	}

	switch es.Type {
	case multipleObjectValuesType:
		result = convertMultipleObjectValuesFromDTO(es)
	case ttlType:
		// For ttl type, API returns value but no values array. Ensure Values/ValueSet match plan.
		result.Values = []ExtraSettingsValuesObjectModel{}
		result.ValueSet = nil
	case choiceMultipleValueType:
		result = convertChoiceMultipleValueFromDTO(es, origSetting)
	default:
		result = convertDefaultTypeFromDTO(es, origSetting)
	}

	return result
}

// convertMultipleObjectValuesFromDTO handles the multiple-object-values type conversion.
// For this type, only the value field is used in the values array.
func convertMultipleObjectValuesFromDTO(es *cloud_risk_management_dto.RuleExtraSetting) ExtraSettingModel {
	result := ExtraSettingModel{
		Name:     types.StringValue(es.Name),
		Type:     types.StringValue(es.Type),
		Value:    types.StringNull(),
		Values:   []ExtraSettingsValuesObjectModel{},
		ValueSet: nil, // multiple-object-values doesn't use value_set
	}

	if len(es.Values) > 0 {
		result.Values = []ExtraSettingsValuesObjectModel{}
		for _, value := range es.Values {
			valMap, ok := value.(map[string]interface{})
			if !ok {
				continue
			}

			valuesObj := ExtraSettingsValuesObjectModel{
				// For multiple-object-values, all fields except value should be null/nil
				Enabled:    types.BoolNull(),
				VpcId:      types.StringNull(),
				GatewayIds: nil,
			}

			// Convert pairs format from API back to JSON object string
			if pairs, exists := valMap["pairs"]; exists {
				if pairsSlice, ok := pairs.([]interface{}); ok {
					// Build a map from pairs
					pairsMap := make(map[string]interface{})
					for _, pairItem := range pairsSlice {
						if pairMap, ok := pairItem.(map[string]interface{}); ok {
							if key, keyOk := pairMap["key"].(string); keyOk {
								if value, valOk := pairMap["value"]; valOk {
									pairsMap[key] = value
								}
							}
						}
					}
					// Convert map back to JSON string with sorted keys for consistency
					if len(pairsMap) > 0 {
						// Sort keys to ensure consistent JSON output
						keys := make([]string, 0, len(pairsMap))
						for k := range pairsMap {
							keys = append(keys, k)
						}
						sort.Strings(keys)

						// Build JSON manually with sorted keys
						jsonStr := "{"
						for i, k := range keys {
							if i > 0 {
								jsonStr += ","
							}
							// Marshal key and value separately to handle proper escaping
							keyBytes, _ := json.Marshal(k)
							valBytes, _ := json.Marshal(pairsMap[k])
							jsonStr += string(keyBytes) + ":" + string(valBytes)
						}
						jsonStr += "}"
						valuesObj.Value = types.StringValue(jsonStr)
					}
				}
			} else if val, exists := valMap["value"]; exists {
				// Handle value field if not pairs
				if strVal, ok := val.(string); ok {
					valuesObj.Value = types.StringValue(strVal)
				} else if jsonBytes, err := json.Marshal(val); err == nil {
					valuesObj.Value = types.StringValue(string(jsonBytes))
				} else {
					valuesObj.Value = types.StringNull()
				}
			} else {
				valuesObj.Value = types.StringNull()
			}

			result.Values = append(result.Values, valuesObj)
		}
	}

	return result
}

// convertChoiceMultipleValueFromDTO handles the choice-multiple-value type conversion.
func convertChoiceMultipleValueFromDTO(es *cloud_risk_management_dto.RuleExtraSetting, origSetting *ExtraSettingModel) ExtraSettingModel {
	result := ExtraSettingModel{
		Name:     types.StringValue(es.Name),
		Type:     types.StringValue(es.Type),
		Value:    types.StringNull(),
		Values:   []ExtraSettingsValuesObjectModel{},
		ValueSet: nil, // choice-multiple-value doesn't use value_set
	}
	// Build a map of original plan values by their value field for lookup
	origValuesMap := make(map[string]*ExtraSettingsValuesObjectModel)
	if origSetting != nil && len(origSetting.Values) > 0 {
		for i := range origSetting.Values {
			origValuesMap[origSetting.Values[i].Value.ValueString()] = &origSetting.Values[i]
		}
	}

	if len(es.Values) > 0 {
		result.Values = []ExtraSettingsValuesObjectModel{}
		for _, value := range es.Values {
			valMap, ok := value.(map[string]interface{})
			if !ok {
				continue
			}

			// Handle value field
			var valueStr string
			if val, exists := valMap["value"]; exists {
				if strVal, ok := ConvertValueToString(val); ok {
					valueStr = strVal
				}
			}

			// Look up original plan value for this entry
			origValue := origValuesMap[valueStr]

			// Get original field values or defaults
			origEnabled := types.BoolNull()
			if origValue != nil {
				origEnabled = origValue.Enabled
			}

			valuesObj := ExtraSettingsValuesObjectModel{}
			fields := convertValuesObjectFromMap(valMap, valueStr, origEnabled, types.StringNull(), nil)
			valuesObj.Value = fields.Value
			valuesObj.Enabled = fields.Enabled

			// For choice-multiple-value, vpc_id and gateway_ids are not used - set to null
			valuesObj.VpcId = types.StringNull()
			valuesObj.GatewayIds = nil

			result.Values = append(result.Values, valuesObj)
		}
	}

	return result
}

// convertDefaultTypeFromDTO handles the default type conversion (including value_set types).
func convertDefaultTypeFromDTO(es *cloud_risk_management_dto.RuleExtraSetting, origSetting *ExtraSettingModel) ExtraSettingModel {
	// Initialize value from API response
	var valueField types.String
	if strVal, ok := ConvertValueToString(es.Value); ok {
		valueField = types.StringValue(strVal)
	} else {
		valueField = types.StringNull()
	}

	result := ExtraSettingModel{
		Name:     types.StringValue(es.Name),
		Type:     types.StringValue(es.Type),
		Value:    valueField, // Preserve value for single-value types
		Values:   []ExtraSettingsValuesObjectModel{},
		ValueSet: []types.String{},
	}
	// Check if the original plan used value_set for this type
	useValueSet := origSetting != nil && len(origSetting.ValueSet) > 0

	// For value_set types, populate value_set if original plan used it
	if IsValueSetType(es.Type) && useValueSet && len(es.Values) > 0 {
		valueSet := make([]types.String, 0, len(es.Values))
		for _, value := range es.Values {
			valMap, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			if val, exists := valMap["value"]; exists {
				if strVal, ok := ConvertValueToString(val); ok {
					valueSet = append(valueSet, types.StringValue(strVal))
				}
			}
		}
		if len(valueSet) > 0 {
			result.ValueSet = valueSet
			result.Values = []ExtraSettingsValuesObjectModel{} // When using value_set, values should be empty list
			result.Value = types.StringNull()                  // When using value_set, value should be null
		}
	} else if len(es.Values) > 0 {
		// Fallback to values block for complex types or when original didn't use value_set
		result = convertValuesBlockFromDTO(es, origSetting)
	} else {
		// Single value type - ensure values and value_set are nil, not empty
		result.Values = nil
		result.ValueSet = nil
	}

	return result
}

// valuesObjectFields holds the converted fields from a values object map.
type valuesObjectFields struct {
	Value      types.String
	Enabled    types.Bool
	VpcId      types.String
	GatewayIds []types.String
}

// convertValuesObjectFromMap is a generic helper that converts common fields from a map to populate fields.
// It handles value, enabled, vpcId, and gatewayIds.
func convertValuesObjectFromMap(
	valMap map[string]interface{},
	valueStr string,
	origEnabled types.Bool,
	origVpcId types.String,
	origGatewayIds []types.String,
) valuesObjectFields {
	fields := valuesObjectFields{}

	// Value is already extracted - set to null if empty
	if valueStr != "" {
		fields.Value = types.StringValue(valueStr)
	} else {
		fields.Value = types.StringNull()
	}

	// Handle enabled field - use plan value if API doesn't return it
	if enabledVal, exists := valMap["enabled"]; exists {
		if boolVal, ok := enabledVal.(bool); ok {
			fields.Enabled = types.BoolValue(boolVal)
		} else {
			fields.Enabled = origEnabled
		}
	} else {
		fields.Enabled = origEnabled
	}

	// Handle vpcId field
	if vpcIdVal, exists := valMap["vpcId"]; exists {
		if vpcIdStr, ok := vpcIdVal.(string); ok {
			fields.VpcId = types.StringValue(vpcIdStr)
		} else {
			fields.VpcId = origVpcId
		}
	} else {
		fields.VpcId = origVpcId
	}

	// Handle gatewayIds field
	if gatewayIdsVal, exists := valMap["gatewayIds"]; exists {
		if gatewayIdsSlice, ok := gatewayIdsVal.([]interface{}); ok {
			gatewayIdsArr := make([]types.String, 0, len(gatewayIdsSlice))
			for _, gid := range gatewayIdsSlice {
				if gidStr, ok := gid.(string); ok {
					gatewayIdsArr = append(gatewayIdsArr, types.StringValue(gidStr))
				}
			}
			fields.GatewayIds = gatewayIdsArr
		} else {
			fields.GatewayIds = origGatewayIds
		}
	} else {
		fields.GatewayIds = origGatewayIds
	}

	return fields
}

// convertValuesBlockFromDTO converts the values block from a DTO.
func convertValuesBlockFromDTO(es *cloud_risk_management_dto.RuleExtraSetting, origSetting *ExtraSettingModel) ExtraSettingModel {
	result := ExtraSettingModel{
		Name:     types.StringValue(es.Name),
		Type:     types.StringValue(es.Type),
		Value:    types.StringNull(),
		Values:   []ExtraSettingsValuesObjectModel{},
		ValueSet: nil, // When using values block, value_set should be nil
	}
	// Build a map of original plan values by their value field for lookup
	// For multiple-vpc-gateway-mappings, use vpcId as the key instead of value
	origValuesMap := make(map[string]*ExtraSettingsValuesObjectModel)
	useVpcIdAsKey := es.Type == "multiple-vpc-gateway-mappings"
	if origSetting != nil && len(origSetting.Values) > 0 {
		for i := range origSetting.Values {
			var key string
			if useVpcIdAsKey {
				key = origSetting.Values[i].VpcId.ValueString()
			} else {
				key = origSetting.Values[i].Value.ValueString()
			}
			origValuesMap[key] = &origSetting.Values[i]
		}
	}

	result.Values = []ExtraSettingsValuesObjectModel{}
	for _, value := range es.Values {
		valMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle value field
		var valueStr string
		if val, exists := valMap["value"]; exists {
			if strVal, ok := ConvertValueToString(val); ok {
				valueStr = strVal
			}
		}

		// For multiple-vpc-gateway-mappings, use vpcId as lookup key
		var lookupKey string
		if useVpcIdAsKey {
			if vpcId, exists := valMap["vpcId"]; exists {
				if vpcIdStr, ok := vpcId.(string); ok {
					lookupKey = vpcIdStr
				}
			}
		} else {
			lookupKey = valueStr
		}

		// Look up original plan value for this entry
		origValue := origValuesMap[lookupKey]

		// Get original field values or defaults
		origEnabled := types.BoolNull()
		origVpcId := types.StringNull()
		origGatewayIds := []types.String{}
		if origValue != nil {
			origEnabled = origValue.Enabled
			origVpcId = origValue.VpcId
			origGatewayIds = origValue.GatewayIds
		}

		valuesObj := ExtraSettingsValuesObjectModel{}
		fields := convertValuesObjectFromMap(valMap, valueStr, origEnabled, origVpcId, origGatewayIds)
		valuesObj.Value = fields.Value
		valuesObj.Enabled = fields.Enabled
		valuesObj.VpcId = fields.VpcId
		valuesObj.GatewayIds = fields.GatewayIds

		result.Values = append(result.Values, valuesObj)
	}

	return result
}
