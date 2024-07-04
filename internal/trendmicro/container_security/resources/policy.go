package resources

import (
	"context"
	"errors"
	"fmt"
	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/container_security/api"
	"terraform-provider-vision-one/pkg/dto"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"terraform-provider-vision-one/internal/trendmicro/container_security/resources/config"
)

var (
	_ resource.Resource                = &PolicyResource{}
	_ resource.ResourceWithConfigure   = &PolicyResource{}
	_ resource.ResourceWithImportState = &PolicyResource{}
)

func NewPolicyResource() resource.Resource {
	return &PolicyResource{
		client: &api.CsClient{},
	}
}

type PolicyResource struct {
	client *api.CsClient
}

func (p *PolicyResource) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if request.ProviderData == nil {
		return
	}

	client, ok := request.ProviderData.(*trendmicro.Client)
	tflog.SetField(ctx, "api client", client)

	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *trendmicro.CsClient, got: %T. Please report this issue to the provider developers. Message: %v", request.ProviderData, client),
		)

		return
	}

	p.client.Client = client
}

func (p *PolicyResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_" + config.RESOURCE_TYPE_POLICY
}

func (p *PolicyResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = generatePolicySchema()
}

func (p *PolicyResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data dto.PolicyResourceModel

	// Read Terraform plan data into the model
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	// Generate the API request from the model
	apiRequest := generateCreatePolicyRequest(&data)

	apiResponse, err := p.client.CreatePolicy(&apiRequest)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		response.Diagnostics.AddError(
			"Unable to Create a policy",
			"An unexpected error occurred when creating the Container Security policy. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro Client Error: "+err.Error())
		return
	}

	data.ID = types.StringValue(apiResponse.ID)
	data.CreatedDateTime = types.StringValue(apiResponse.CreatedDateTime)
	data.UpdatedDateTime = types.StringValue(apiResponse.UpdatedDateTime)
	data.RulesetsUpdatedDateTime = types.StringValue(apiResponse.RulesetsUpdatedDateTime)

	tflog.Trace(ctx, "created a policy resource")

	// Save data into Terraform state
	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}
}

func generateCreatePolicyRequest(data *dto.PolicyResourceModel) dto.CreatePolicyRequest {
	result := dto.CreatePolicyRequest{}

	if !data.Name.IsNull() {
		result.Name = data.Name.ValueString()
	}

	if !data.Description.IsNull() {
		result.Description = data.Description.ValueString()
	}

	// Convert the policy default object
	if data.PolicyDefault != nil {
		result.PolicyDefault = &dto.PolicyDefault{}
		if data.PolicyDefault.PolicyRuleList != nil {
			result.PolicyDefault.PolicyRuleList =
				generatePolicyRequestForPolicyRuleList(data.PolicyDefault.PolicyRuleList)
		} else {
			result.PolicyDefault.PolicyRuleList = nil
		}
		if data.PolicyDefault.PolicyExceptionList != nil {
			result.PolicyDefault.PolicyExceptionList =
				generatePolicyRequestForPolicyRuleList(data.PolicyDefault.PolicyExceptionList)
		} else {
			result.PolicyDefault.PolicyExceptionList = nil
		}
	} else {
		result.PolicyDefault = nil
	}

	// Convert the policy namespaced list
	if data.PolicyNamespacedList != nil {
		result.PolicyNamespacedList =
			generatePolicyRequestForPolicyNamespacedList(data.PolicyNamespacedList)
	} else {
		result.PolicyNamespacedList = nil
	}

	// Convert the policy runtime object
	if data.PolicyRuntime != nil && data.PolicyRuntime.PolicyRulesetList != nil {
		result.PolicyRuntime = &dto.PolicyRuntime{}
		result.PolicyRuntime.PolicyRulesetList =
			generateRequestForPolicyRulesetList(data.PolicyRuntime.PolicyRulesetList)
	} else {
		result.PolicyRuntime = nil
	}

	result.XdrEnabled = data.XdrEnabled.ValueBool()

	return result
}

func generateRequestForPolicyRulesetList(data []dto.PolicyRulesetResourceModel) []dto.PolicyRuleset {
	result := make([]dto.PolicyRuleset, 0)
	for _, ruleset := range data {
		policyRuleset := dto.PolicyRuleset{}
		if !ruleset.ID.IsNull() {
			policyRuleset.ID = ruleset.ID.ValueString()
		}

		result = append(result, policyRuleset)
	}

	return result
}

func generatePolicyRequestForPolicyNamespacedList(data []dto.PolicyNamespacedResourceModel) []dto.PolicyNamespaced {
	result := make([]dto.PolicyNamespaced, 0)
	for _, namespaced := range data {
		policyNamespaced := dto.PolicyNamespaced{}
		if !namespaced.Name.IsNull() {
			policyNamespaced.Name = namespaced.Name.ValueString()
		}
		if len(namespaced.Namespaces) > 0 {
			policyNamespaced.Namespaces = make([]string, 0)
			for _, namespace := range namespaced.Namespaces {
				policyNamespaced.Namespaces = append(
					policyNamespaced.Namespaces, namespace.ValueString())
			}
		}
		if namespaced.PolicyRuleList != nil {
			policyNamespaced.PolicyRuleList =
				generatePolicyRequestForPolicyRuleList(namespaced.PolicyRuleList)
		}
		if namespaced.PolicyExceptionList != nil {
			policyNamespaced.PolicyExceptionList =
				generatePolicyRequestForPolicyRuleList(namespaced.PolicyExceptionList)
		}

		result = append(result, policyNamespaced)
	}

	return result
}

func generatePolicyRequestForPolicyRuleList(data []dto.PolicyRuleResourceModel) []dto.PolicyRule {
	result := make([]dto.PolicyRule, 0)
	if data == nil {
		return result
	}
	for _, rule := range data {
		policyRule := dto.PolicyRule{}
		if !rule.Type.IsNull() {
			policyRule.Type = rule.Type.ValueString()
		}
		if !rule.Enabled.IsNull() {
			policyRule.Enabled = rule.Enabled.ValueBool()
		}
		if !rule.Action.IsNull() {
			policyRule.Action = rule.Action.ValueString()
		}
		if !rule.Mitigation.IsNull() {
			policyRule.Mitigation = rule.Mitigation.ValueString()
		}

		if rule.PolicyRuleStatement != nil && rule.PolicyRuleStatement.PolicyRulePropertyList != nil {
			policyRule.PolicyRuleStatement = &dto.PolicyRuleStatement{}
			for _, property := range rule.PolicyRuleStatement.PolicyRulePropertyList {
				policyRuleProperty := dto.PolicyRuleProperty{}
				if !property.Key.IsNull() {
					policyRuleProperty.Key = property.Key.ValueString()
				}
				if !property.Value.IsNull() {
					policyRuleProperty.Value = property.Value.ValueString()
				}

				policyRule.PolicyRuleStatement.PolicyRulePropertyList = append(
					policyRule.PolicyRuleStatement.PolicyRulePropertyList, policyRuleProperty)
			}
		} else {
			policyRule.PolicyRuleStatement = nil
		}

		result = append(result, policyRule)
	}

	return result
}

func (p *PolicyResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data dto.PolicyResourceModel

	// Read Terraform prior state data into the model
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	apiResponse, err := p.client.GetPolicy(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, dto.ErrorNotFound) {
			tflog.Debug(ctx, err.Error())
			response.State.RemoveResource(ctx)
			return
		} else {
			tflog.Debug(ctx, err.Error())
			response.Diagnostics.AddError(
				"Unable to Get a policy id "+data.ID.ValueString(),
				"An unexpected error occurred when getting the Container Security policy. "+
					"If the error is not clear, please contact the provider developers.\n\n"+
					"TrendMicro Client Error: "+err.Error())
			return
		}
	}

	tflog.Trace(ctx, "read a resource")
	data.ID = types.StringValue(apiResponse.ID)
	data.Name = types.StringValue(apiResponse.Name)
	if apiResponse.Description != "" {
		data.Description = types.StringValue(apiResponse.Description)
	}

	if apiResponse.Default != nil {
		data.PolicyDefault = &dto.PolicyDefaultResourceModel{}
		if apiResponse.Default.PolicyRuleList != nil {
			// Save the policy default object's rule list state
			data.PolicyDefault.PolicyRuleList = saveStatePolicyRuleList(apiResponse.Default.PolicyRuleList)
		}
		if apiResponse.Default.PolicyExceptionList != nil {
			// Save the policy default object's exception list state
			data.PolicyDefault.PolicyExceptionList = saveStatePolicyRuleList(apiResponse.Default.PolicyExceptionList)
		}
	} else {
		data.PolicyDefault = nil
	}

	// Save the policy namespaced list state
	if apiResponse.Namespaced != nil {
		data.PolicyNamespacedList = saveStatePolicyNamespacedList(apiResponse.Namespaced)
	} else {
		data.PolicyNamespacedList = nil
	}

	// Save the policy runtime object's ruleset list state
	data.PolicyRuntime = saveStatePolicyRuntime(apiResponse.Runtime)

	data.XdrEnabled = types.BoolValue(apiResponse.XdrEnabled)
	data.CreatedDateTime = types.StringValue(apiResponse.CreatedDateTime)
	data.UpdatedDateTime = types.StringValue(apiResponse.UpdatedDateTime)
	data.RulesetsUpdatedDateTime = types.StringValue(apiResponse.RulesetsUpdatedDateTime)

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}
}

func saveStatePolicyRuntime(apiResponsePolicyRuntime *dto.PolicyRuntime) *dto.PolicyRuntimeResourceModel {
	result := &dto.PolicyRuntimeResourceModel{}
	resultRulesetList := make([]dto.PolicyRulesetResourceModel, 0)

	if apiResponsePolicyRuntime != nil && apiResponsePolicyRuntime.PolicyRulesetList != nil {
		apiResponsePolicyRulesetList := apiResponsePolicyRuntime.PolicyRulesetList
		for _, ruleset := range apiResponsePolicyRulesetList {
			rulesetResult := dto.PolicyRulesetResourceModel{}
			rulesetResult.ID = types.StringValue(ruleset.ID)

			resultRulesetList = append(resultRulesetList, rulesetResult)
		}
		result.PolicyRulesetList = resultRulesetList
	} else {
		result = nil
	}

	return result
}

func saveStatePolicyNamespacedList(apiResponsePolicyNamespaced []dto.PolicyNamespaced) []dto.PolicyNamespacedResourceModel {
	resultList := make([]dto.PolicyNamespacedResourceModel, 0)
	for _, namespaced := range apiResponsePolicyNamespaced {
		namespacedResult := dto.PolicyNamespacedResourceModel{}
		namespacedResult.Name = types.StringValue(namespaced.Name)
		namespacedResult.Namespaces = make([]types.String, 0)
		for _, namespace := range namespaced.Namespaces {
			namespacedResult.Namespaces = append(namespacedResult.Namespaces, types.StringValue(namespace))
		}

		if namespaced.PolicyRuleList != nil {
			// Save the namespaced object's rule list state
			namespacedResult.PolicyRuleList = saveStatePolicyRuleList(namespaced.PolicyRuleList)
		} else {
			namespacedResult.PolicyRuleList = nil
		}
		if namespaced.PolicyExceptionList != nil {
			// Save the namespaced object's exception list state
			namespacedResult.PolicyExceptionList = saveStatePolicyRuleList(namespaced.PolicyExceptionList)
		} else {
			namespacedResult.PolicyExceptionList = nil
		}

		resultList = append(resultList, namespacedResult)
	}

	return resultList
}

func saveStatePolicyRuleList(apiResponsePolicyRule []dto.PolicyRule) []dto.PolicyRuleResourceModel {
	resultList := make([]dto.PolicyRuleResourceModel, 0)
	for _, rule := range apiResponsePolicyRule {
		ruleResult := dto.PolicyRuleResourceModel{}
		ruleResult.Type = types.StringValue(rule.Type)
		ruleResult.Enabled = types.BoolValue(rule.Enabled)
		ruleResult.Action = types.StringValue(rule.Action)
		ruleResult.Mitigation = types.StringValue(rule.Mitigation)

		// Save the  default object's rule statement object's property list
		if rule.PolicyRuleStatement != nil && rule.PolicyRuleStatement.PolicyRulePropertyList != nil {
			ruleResult.PolicyRuleStatement = &dto.PolicyRuleStatementResourceModel{}
			ruleResult.PolicyRuleStatement.PolicyRulePropertyList = make([]dto.PolicyRulePropertyResourceModel, 0)
			for _, property := range rule.PolicyRuleStatement.PolicyRulePropertyList {
				propertyResult := dto.PolicyRulePropertyResourceModel{}
				propertyResult.Key = types.StringValue(property.Key)
				propertyResult.Value = types.StringValue(property.Value)

				ruleResult.PolicyRuleStatement.PolicyRulePropertyList = append(
					ruleResult.PolicyRuleStatement.PolicyRulePropertyList, propertyResult)
			}
		} else {
			ruleResult.PolicyRuleStatement = nil
		}

		resultList = append(resultList, ruleResult)
	}

	return resultList
}

func (p *PolicyResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var data dto.PolicyResourceModel

	// Read Terraform plan data into the model
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	// Generate the API request from the model
	apiRequest := generateUpdatePolicyRequest(&data)

	apiResponse, err := p.client.UpdatePolicy(data.ID.ValueString(), &apiRequest)
	if err != nil {
		if errors.Is(err, dto.ErrorNotFound) {
			tflog.Debug(ctx, err.Error())
			response.Diagnostics.AddError(
				"Unable to found policy id "+data.ID.ValueString(),
				"An unexpected error occurred when updating the Container Security policy. "+
					"If the error is not clear, please contact the provider developers.\n\n"+
					"TrendMicro Client Error: "+err.Error())
			return
		} else {
			tflog.Debug(ctx, err.Error())
			response.Diagnostics.AddError(
				"Unable to update the policy id "+data.ID.ValueString(),
				"An unexpected error occurred when updating the Container Security policy. "+
					"If the error is not clear, please contact the provider developers.\n\n"+
					"TrendMicro Client Error: "+err.Error())
			return
		}
	}

	data.UpdatedDateTime = types.StringValue(apiResponse.UpdatedDateTime)
	data.RulesetsUpdatedDateTime = types.StringValue(apiResponse.RulesetsUpdatedDateTime)

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}
}

func generateUpdatePolicyRequest(data *dto.PolicyResourceModel) dto.UpdatePolicyRequest {
	result := dto.UpdatePolicyRequest{}

	if !data.Description.IsNull() {
		result.Description = data.Description.ValueString()
	}

	// Convert the policy default object
	if data.PolicyDefault != nil {
		result.PolicyDefault = &dto.PolicyDefault{}
		if data.PolicyDefault.PolicyRuleList != nil {
			result.PolicyDefault.PolicyRuleList =
				generatePolicyRequestForPolicyRuleList(data.PolicyDefault.PolicyRuleList)
		} else {
			result.PolicyDefault.PolicyRuleList = nil
		}
		if data.PolicyDefault.PolicyExceptionList != nil {
			result.PolicyDefault.PolicyExceptionList =
				generatePolicyRequestForPolicyRuleList(data.PolicyDefault.PolicyExceptionList)
		} else {
			result.PolicyDefault.PolicyExceptionList = nil
		}
	} else {
		result.PolicyDefault = nil
	}

	// Convert the policy namespaced list
	if data.PolicyNamespacedList != nil {
		result.PolicyNamespacedList =
			generatePolicyRequestForPolicyNamespacedList(data.PolicyNamespacedList)
	} else {
		result.PolicyNamespacedList = nil
	}

	// Convert the policy runtime object
	if data.PolicyRuntime != nil && data.PolicyRuntime.PolicyRulesetList != nil {
		result.PolicyRuntime = &dto.PolicyRuntime{}
		result.PolicyRuntime.PolicyRulesetList =
			generateRequestForPolicyRulesetList(data.PolicyRuntime.PolicyRulesetList)
	} else {
		result.PolicyRuntime = nil
	}

	result.XdrEnabled = data.XdrEnabled.ValueBool()

	return result
}

func (p *PolicyResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data dto.PolicyResourceModel

	// Read Terraform prior state data into the model
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	err := p.client.DeletePolicy(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, dto.ErrorNotFound) {
			tflog.Debug(ctx, err.Error())
			return
		} else {
			tflog.Debug(ctx, err.Error())
			response.Diagnostics.AddError(
				"Unable to delete the policy id "+data.ID.ValueString(),
				"An unexpected error occurred when deleting the Container Security policy. "+
					"If the error is not clear, please contact the provider developers.\n\n"+
					"TrendMicro Client Error: "+err.Error())
			return
		}
	}
}

func (p *PolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(IDSchemaName), req, resp)
}
