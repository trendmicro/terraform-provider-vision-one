package utils

import (
	"encoding/json"
	"sort"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// =============================================================================
// DTO to Model Converters (API Response -> Terraform State)
// =============================================================================
const (
	multipleObjectValuesType             = "multiple-object-values"
	choiceMultipleValueType              = "choice-multiple-value"
	choiceMultipleValueWithTagsType      = "choice-multiple-value-with-tags"
	choiceMultipleValueWithRiskLevelType = "choice-multiple-value-with-risk-level"
	ttlType                              = "ttl"
	numericTypeMultipleNumberValues      = "multiple-number-values"
)

// ConvertExceptionsFromDTO converts a RuleExceptions DTO from the API response
// to a RuleExceptionsModel for the Terraform state.
func ConvertExceptionsFromDTO(exceptions *cloud_risk_management_dto.RuleExceptions) *RuleExceptionsModel {
	if exceptions == nil {
		return nil
	}

	result := &RuleExceptionsModel{}

	// Only set FilterTags if the API returned it (even if empty)
	// nil means the field was not sent/returned, [] means it was sent as empty
	if exceptions.FilterTags != nil {
		if len(exceptions.FilterTags) > 0 {
			filterTags := make([]types.String, len(exceptions.FilterTags))
			for j, ft := range exceptions.FilterTags {
				filterTags[j] = types.StringValue(ft)
			}
			result.FilterTags = filterTags
		} else {
			// Empty array was explicitly sent/returned
			result.FilterTags = []types.String{}
		}
	}

	// Only set ResourceIds if the API returned it (even if empty)
	if exceptions.ResourceIds != nil {
		if len(exceptions.ResourceIds) > 0 {
			resourceIds := make([]types.String, len(exceptions.ResourceIds))
			for j, rid := range exceptions.ResourceIds {
				resourceIds[j] = types.StringValue(rid)
			}
			result.ResourceIds = resourceIds
		} else {
			// Empty array was explicitly sent/returned
			result.ResourceIds = []types.String{}
		}
	}

	return result
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
	case choiceMultipleValueWithTagsType:
		result = convertChoiceMultipleValueFromDTO(es, origSetting)
	case choiceMultipleValueWithRiskLevelType:
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

	if es.Values != nil && len(*es.Values) > 0 {
		result.Values = []ExtraSettingsValuesObjectModel{}
		for _, value := range *es.Values {
			valMap, ok := value.(map[string]interface{})
			if !ok {
				continue
			}

			valuesObj := ExtraSettingsValuesObjectModel{
				// For multiple-object-values, all fields except value should be null/nil/empty
				Enabled:             types.BoolNull(),
				VpcId:               types.StringNull(),
				GatewayIds:          nil,
				CustomizedTags:      types.SetNull(types.StringType),
				CustomizedRiskLevel: types.StringNull(),
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

// convertChoiceMultipleValueFromDTO handles the choice-multiple-value, choice-multiple-value-with-tags, and choice-multiple-value-with-risk-level type conversion.
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

	if es.Values != nil && len(*es.Values) > 0 {
		result.Values = []ExtraSettingsValuesObjectModel{}
		for _, value := range *es.Values {
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

			valuesObj := ExtraSettingsValuesObjectModel{}
			fields := convertValuesObjectFromMap(valMap, valueStr, origValue, es.Type)
			valuesObj.Value = fields.Value
			valuesObj.Enabled = fields.Enabled
			valuesObj.VpcId = types.StringNull()
			valuesObj.GatewayIds = nil
			valuesObj.CustomizedTags = fields.CustomizedTags
			valuesObj.CustomizedRiskLevel = fields.CustomizedRiskLevel

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
	useValueSet := origSetting != nil && origSetting.ValueSet != nil

	// For value_set types, populate value_set if original plan used it
	if IsValueSetType(es.Type) && useValueSet {
		var esValues []any
		if es.Values != nil {
			esValues = *es.Values
		}
		valueSet := make([]types.String, 0, len(esValues))
		for _, value := range esValues {
			// Handle map values (e.g., {"value": "..."})
			valMap, ok := value.(map[string]interface{})
			if ok {
				if val, exists := valMap["value"]; exists {
					if strVal, ok := ConvertValueToString(val); ok {
						valueSet = append(valueSet, types.StringValue(strVal))
					}
				}
				continue
			}
			// Handle plain string/numeric values (e.g., "eu-west-1")
			if strVal, ok := ConvertValueToString(value); ok {
				valueSet = append(valueSet, types.StringValue(strVal))
			}
		}
		result.ValueSet = valueSet
		result.Values = []ExtraSettingsValuesObjectModel{} // When using value_set, values should be empty list
		result.Value = types.StringNull()                  // When using value_set, value should be null
	} else if es.Values != nil && len(*es.Values) > 0 {
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
	Value               types.String
	Enabled             types.Bool
	VpcId               types.String
	GatewayIds          []types.String
	CustomizedTags      types.Set
	CustomizedRiskLevel types.String
}

// convertValuesObjectFromMap converts all fields from an API values map entry into the Terraform model fields.
// settingType is used to gate type-specific fields:
//   - customizedTags is only populated for choice-multiple-value-with-tags
//   - customizedRiskLevel is only populated for choice-multiple-value-with-risk-level
//   - vpcId and gatewayIds are only populated for multiple-vpc-gateway-mappings
func convertValuesObjectFromMap(
	valMap map[string]interface{},
	valueStr string,
	origValue *ExtraSettingsValuesObjectModel,
	settingType string,
) valuesObjectFields {
	fields := valuesObjectFields{
		CustomizedTags:      types.SetNull(types.StringType),
		CustomizedRiskLevel: types.StringNull(),
	}

	// Value is already extracted - set to null if empty
	if valueStr != "" {
		fields.Value = types.StringValue(valueStr)
	} else {
		fields.Value = types.StringNull()
	}

	// Get defaults from origValue (or use nulls)
	origEnabled := types.BoolNull()
	origVpcId := types.StringNull()
	var origGatewayIds []types.String

	if origValue != nil {
		origEnabled = origValue.Enabled
		origVpcId = origValue.VpcId
		origGatewayIds = origValue.GatewayIds
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

	// Type-gated fields: only populate for the specific type that uses them.
	switch settingType {
	case choiceMultipleValueWithTagsType:
		if tagsVal, exists := valMap["customizedTags"]; exists {
			if tagsSlice, ok := tagsVal.([]interface{}); ok {
				elems := make([]attr.Value, 0, len(tagsSlice))
				for _, tag := range tagsSlice {
					if tagStr, ok := tag.(string); ok {
						elems = append(elems, types.StringValue(tagStr))
					}
				}
				fields.CustomizedTags = types.SetValueMust(types.StringType, elems)
			}
		}
	case choiceMultipleValueWithRiskLevelType:
		if riskVal, exists := valMap["customizedRiskLevel"]; exists {
			if riskStr, ok := riskVal.(string); ok {
				fields.CustomizedRiskLevel = types.StringValue(riskStr)
			}
		}
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
	if es.Values != nil {
		for _, value := range *es.Values {
			// Handle plain string/numeric values (some API responses use this format)
			if strVal, ok := ConvertValueToString(value); ok {
				valuesObj := ExtraSettingsValuesObjectModel{
					Value:               types.StringValue(strVal),
					Enabled:             types.BoolNull(),
					VpcId:               types.StringNull(),
					GatewayIds:          nil,
					CustomizedTags:      types.SetNull(types.StringType),
					CustomizedRiskLevel: types.StringNull(),
				}
				result.Values = append(result.Values, valuesObj)
				continue
			}

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

			valuesObj := ExtraSettingsValuesObjectModel{}
			fields := convertValuesObjectFromMap(valMap, valueStr, origValue, es.Type)
			valuesObj.Value = fields.Value
			valuesObj.Enabled = fields.Enabled
			valuesObj.VpcId = fields.VpcId
			valuesObj.GatewayIds = fields.GatewayIds
			valuesObj.CustomizedTags = fields.CustomizedTags
			valuesObj.CustomizedRiskLevel = fields.CustomizedRiskLevel
			result.Values = append(result.Values, valuesObj)
		}
	}

	return result
}
