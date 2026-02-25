package cloud_account_management

import (
	"context"
	"crypto/rand"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// GenerateRandomString generates a random string of specified length
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)

	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic("failed to generate random bytes: " + err.Error())
	}

	for i := range randomBytes {
		result[i] = charset[int(randomBytes[i])%len(charset)]
	}

	return string(result)
}

// ConvertStringSliceToListValue converts a string slice to a types.List
func ConvertStringSliceToListValue(permissions []string) types.List {
	elements := make([]string, len(permissions))
	copy(elements, permissions)

	listValue, _ := types.ListValueFrom(context.Background(), types.StringType, elements)
	return listValue
}

// GetStringValue safely converts string to types.String, returning null for empty strings
func GetStringValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// GetInt64Value safely converts int to types.Int64, returning null for zero values
func GetInt64Value(i int) types.Int64 {
	if i == 0 {
		return types.Int64Null()
	}
	return types.Int64Value(int64(i))
}

// GetBoolValue converts bool to types.Bool
func GetBoolValue(b bool) types.Bool {
	return types.BoolValue(b)
}

// GetBoolPointerValue safely converts *bool to types.Bool, returning null for nil pointers
func GetBoolPointerValue(b *bool) types.Bool {
	if b == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*b)
}

// ConvertStringSlice converts native string slice to types.String slice
func ConvertStringSlice(slice []string) []types.String {
	if slice == nil {
		return nil
	}
	result := make([]types.String, len(slice))
	for i, s := range slice {
		result[i] = types.StringValue(s)
	}
	return result
}

// ConvertTypesStringSliceToStringSlice converts Terraform types.String slice to native string slice
func ConvertTypesStringSliceToStringSlice(typesSlice []types.String) []string {
	if len(typesSlice) == 0 {
		return []string{}
	}
	result := make([]string, len(typesSlice))
	for i, ts := range typesSlice {
		result[i] = ts.ValueString()
	}
	return result
}

// GetStringFromInterface safely extracts string from interface{} and converts to types.String
func GetStringFromInterface(value any) types.String {
	if value == nil {
		return types.StringNull()
	}
	if str, ok := value.(string); ok {
		return GetStringValue(str)
	}
	return types.StringNull()
}

// ConvertInterfaceSliceToStringSlice safely converts interface{} slice to types.String slice
func ConvertInterfaceSliceToStringSlice(value any) []types.String {
	if value == nil {
		return nil
	}
	if slice, ok := value.([]any); ok {
		result := make([]types.String, len(slice))
		for i, item := range slice {
			if str, ok := item.(string); ok {
				result[i] = types.StringValue(str)
			} else {
				result[i] = types.StringNull()
			}
		}
		return result
	}
	return nil
}

// ConnectedSecurityServiceModel represents a connected security service in Terraform state
type ConnectedSecurityServiceModel struct {
	InstanceIds []types.String `tfsdk:"instance_ids"`
	Name        types.String   `tfsdk:"name"`
}

// FeatureModel represents a cloud account feature in Terraform state
type FeatureModel struct {
	ID              types.String   `tfsdk:"id"`
	Regions         []types.String `tfsdk:"regions"`
	TemplateVersion types.String   `tfsdk:"template_version"`
}

// ConnectedSecurityService represents a connected security service from the API
type ConnectedSecurityService struct {
	Name        string   `json:"name"`
	InstanceIds []string `json:"instanceIds"`
}

// ConvertConnectedSecurityServices transforms API security services to Terraform model
func ConvertConnectedSecurityServices(services []ConnectedSecurityService) []ConnectedSecurityServiceModel {
	if services == nil {
		return nil
	}
	result := make([]ConnectedSecurityServiceModel, len(services))
	for i, service := range services {
		result[i] = ConnectedSecurityServiceModel{
			Name:        GetStringValue(service.Name),
			InstanceIds: ConvertStringSlice(service.InstanceIds),
		}
	}
	return result
}

// ConvertFeatures transforms API features (which can be any type) to Terraform model
// The API returns features as interface{} which requires type assertion
func ConvertFeatures(features any) []FeatureModel {
	if features == nil {
		return nil
	}

	// The features field is defined as any in the API
	// We need to handle it carefully as it might be a slice or other type
	switch f := features.(type) {
	case []any:
		result := make([]FeatureModel, len(f))
		for i, feature := range f {
			if featureMap, ok := feature.(map[string]any); ok {
				result[i] = FeatureModel{
					ID:              GetStringFromInterface(featureMap["id"]),
					Regions:         ConvertInterfaceSliceToStringSlice(featureMap["regions"]),
					TemplateVersion: GetStringFromInterface(featureMap["templateVersion"]),
				}
			}
		}
		return result
	default:
		// If it's not a slice, return empty slice
		return []FeatureModel{}
	}
}
