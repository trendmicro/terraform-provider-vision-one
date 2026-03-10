package resources

import (
	"encoding/json"
	"fmt"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Converts go native []string to tf types.Set of strings
func stringSliceToSet(values []string) (types.Set, diag.Diagnostics) {
	if values == nil {
		return types.SetNull(types.StringType), nil
	}

	if len(values) == 0 {
		return types.SetValueMust(types.StringType, []attr.Value{}), nil
	}

	attrValues := make([]attr.Value, len(values))
	for i, v := range values {
		attrValues[i] = types.StringValue(v)
	}
	return types.SetValue(types.StringType, attrValues)
}

func complianceStandardsToSet(standards []cloud_risk_management_dto.ComplianceStandard) (types.Set, diag.Diagnostics) {
	if standards == nil {
		return types.SetNull(types.StringType), nil
	}

	if len(standards) == 0 {
		return types.SetValueMust(types.StringType, []attr.Value{}), nil
	}

	values := make([]attr.Value, len(standards))
	for i, cs := range standards {
		values[i] = types.StringValue(cs.ID)
	}
	return types.SetValue(types.StringType, values)
}

// maps the API response DTO to the Terraform state model.
func mapCommunicationConfigToState(config *cloud_risk_management_dto.CommunicationConfiguration, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Enabled = types.BoolValue(config.Enabled)
	state.Level = types.StringValue(config.Level)
	state.ChannelType = types.StringValue(config.ChannelType)

	// manual is only supported for SNS and ticketing channels (ServiceNow, Jira, Zendesk).
	// Only sync when true — false is the default and indistinguishable from "not set" in the
	// API response, so we leave state as-is to preserve the user's plan intent.
	if config.ChannelType == channelTypeSns || config.ChannelType == channelTypeServiceNow ||
		config.ChannelType == channelTypeJira || config.ChannelType == channelTypeZendesk {
		if config.Manual {
			state.Manual = types.BoolValue(true)
		}
	} else {
		state.Manual = types.BoolNull()
	}

	if config.ChannelLabel != "" {
		state.ChannelLabel = types.StringValue(config.ChannelLabel)
	} else {
		state.ChannelLabel = types.StringNull()
	}

	if config.AccountID != "" {
		state.AccountID = types.StringValue(config.AccountID)
	}

	if config.ChecksFilter == nil {
		state.ChecksFilter = nil
	} else {
		filter := &ChecksFilterModel{}
		var d diag.Diagnostics

		filter.Regions, d = stringSliceToSet(config.ChecksFilter.Regions)
		diags.Append(d...)
		filter.Services, d = stringSliceToSet(config.ChecksFilter.Services)
		diags.Append(d...)
		filter.RuleIDs, d = stringSliceToSet(config.ChecksFilter.RuleIDs)
		diags.Append(d...)
		filter.Categories, d = stringSliceToSet(config.ChecksFilter.Categories)
		diags.Append(d...)
		filter.RiskLevels, d = stringSliceToSet(config.ChecksFilter.RiskLevels)
		diags.Append(d...)
		filter.Tags, d = stringSliceToSet(config.ChecksFilter.Tags)
		diags.Append(d...)
		filter.ComplianceStandardIDs, d = complianceStandardsToSet(config.ChecksFilter.ComplianceStandards)
		diags.Append(d...)
		filter.Statuses, d = stringSliceToSet(config.ChecksFilter.Statuses)
		diags.Append(d...)

		state.ChecksFilter = filter
	}

	if diags.HasError() {
		return diags
	}

	switch config.ChannelType {
	case channelTypeEmail:
		diags.Append(mapEmailToState(config.ChannelConfiguration, state)...)
	case channelTypeSms:
		diags.Append(mapSmsToState(config.ChannelConfiguration, state)...)
	case channelTypeMsTeams:
		diags.Append(mapMsTeamsToState(config.ChannelConfiguration, state)...)
	case channelTypeSlack:
		diags.Append(mapSlackToState(config.ChannelConfiguration, state)...)
	case channelTypeSns:
		diags.Append(mapSnsToState(config.ChannelConfiguration, state)...)
	case channelTypePagerDuty:
		diags.Append(mapPagerDutyToState(config.ChannelConfiguration, state)...)
	case channelTypeWebhook:
		diags.Append(mapWebhookToState(config.ChannelConfiguration, state)...)
	case channelTypeJira:
		diags.Append(mapJiraToState(config.ChannelConfiguration, state)...)
	case channelTypeZendesk:
		diags.Append(mapZendeskToState(config.ChannelConfiguration, state)...)
	case channelTypeServiceNow:
		diags.Append(mapServiceNowToState(config.ChannelConfiguration, state)...)
	}

	return diags
}

// Channel-specific state mappers

func mapEmailToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.EmailChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse email channel configuration", err.Error())
		return diags
	}
	userIDs, d := stringSliceToSet(config.UserIDs)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	state.EmailConfiguration = &EmailConfigurationModel{
		UserIDs: userIDs,
	}
	return diags
}

func mapSmsToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.SmsChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse sms channel configuration", err.Error())
		return diags
	}
	userIDs, d := stringSliceToSet(config.UserIDs)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	state.SmsConfiguration = &SmsConfigurationModel{
		UserIDs: userIDs,
	}
	return diags
}

func mapMsTeamsToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.MsTeamsChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse ms teams channel configuration", err.Error())
		return diags
	}
	state.MsTeamsConfiguration = &MsTeamsConfigurationModel{
		URL:                 types.StringValue(config.URL),
		IncludeIntroducedBy: types.BoolValue(config.IncludeIntroducedBy),
		IncludeResource:     types.BoolValue(config.IncludeResource),
		IncludeTags:         types.BoolValue(config.IncludeTags),
		IncludeExtraData:    types.BoolValue(config.IncludeExtraData),
	}
	return diags
}

func mapSlackToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.SlackChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse slack channel configuration", err.Error())
		return diags
	}
	state.SlackConfiguration = &SlackConfigurationModel{
		URL:                 types.StringValue(config.URL),
		Channel:             types.StringValue(config.Channel),
		IncludeIntroducedBy: types.BoolValue(config.IncludeIntroducedBy),
		IncludeResource:     types.BoolValue(config.IncludeResource),
		IncludeTags:         types.BoolValue(config.IncludeTags),
		IncludeExtraData:    types.BoolValue(config.IncludeExtraData),
	}
	return diags
}

func mapSnsToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.SnsChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse sns channel configuration", err.Error())
		return diags
	}
	state.SnsConfiguration = &SnsConfigurationModel{
		Arn: types.StringValue(config.Arn),
	}
	return diags
}

func mapPagerDutyToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.PagerDutyChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse pagerduty channel configuration", err.Error())
		return diags
	}
	state.PagerDutyConfiguration = &PagerDutyConfigurationModel{
		ServiceName: types.StringValue(config.ServiceName),
		ServiceKey:  types.StringValue(config.ServiceKey),
	}
	return diags
}

func mapWebhookToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.WebhookChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse webhook channel configuration", err.Error())
		return diags
	}
	// Only update URL from API response
	// Headers and SecurityToken are not returned by API, so preserve existing state
	if state.WebhookConfiguration == nil {
		state.WebhookConfiguration = &WebhookConfigurationModel{}
	}
	state.WebhookConfiguration.URL = types.StringValue(config.URL)
	return diags
}

func mapJiraToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.JiraChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse jira channel configuration", err.Error())
		return diags
	}
	// APIToken is not returned by API, so preserve existing state
	if state.JiraConfiguration == nil {
		state.JiraConfiguration = &JiraConfigurationModel{}
	}
	state.JiraConfiguration.URL = types.StringValue(config.URL)
	state.JiraConfiguration.Username = types.StringValue(config.Username)
	state.JiraConfiguration.Project = types.StringValue(config.Project)
	state.JiraConfiguration.Type = types.StringValue(config.Type)
	if config.AssigneeID != nil {
		state.JiraConfiguration.AssigneeID = types.StringValue(*config.AssigneeID)
	} else {
		state.JiraConfiguration.AssigneeID = types.StringNull()
	}
	if config.Priority != nil {
		state.JiraConfiguration.Priority = types.StringValue(*config.Priority)
	} else {
		state.JiraConfiguration.Priority = types.StringNull()
	}
	return diags
}

func mapZendeskToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.ZendeskChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse zendesk channel configuration", err.Error())
		return diags
	}
	// Password and APIToken are not returned by API, so preserve existing state
	if state.ZendeskConfiguration == nil {
		state.ZendeskConfiguration = &ZendeskConfigurationModel{}
	}
	state.ZendeskConfiguration.URL = types.StringValue(config.URL)
	state.ZendeskConfiguration.Username = types.StringValue(config.Username)
	if config.Type != nil {
		state.ZendeskConfiguration.Type = types.StringValue(*config.Type)
	} else {
		state.ZendeskConfiguration.Type = types.StringNull()
	}
	if config.Priority != nil {
		state.ZendeskConfiguration.Priority = types.StringValue(*config.Priority)
	} else {
		state.ZendeskConfiguration.Priority = types.StringNull()
	}
	if config.GroupID != nil {
		state.ZendeskConfiguration.GroupID = types.Int64Value(*config.GroupID)
	} else {
		state.ZendeskConfiguration.GroupID = types.Int64Null()
	}
	if config.AssigneeID != nil {
		state.ZendeskConfiguration.AssigneeID = types.Int64Value(*config.AssigneeID)
	} else {
		state.ZendeskConfiguration.AssigneeID = types.Int64Null()
	}
	return diags
}

func mapServiceNowToState(raw json.RawMessage, state *CommunicationConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(raw) == 0 {
		return diags
	}
	var config cloud_risk_management_dto.ServiceNowChannelConfiguration
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Failed to parse servicenow channel configuration", err.Error())
		return diags
	}
	// Password is not returned by API, so preserve existing state
	if state.ServiceNowConfiguration == nil {
		state.ServiceNowConfiguration = &ServiceNowConfigurationModel{}
	}
	state.ServiceNowConfiguration.Type = types.StringValue(config.Type)
	state.ServiceNowConfiguration.URL = types.StringValue(config.URL)
	state.ServiceNowConfiguration.Username = types.StringValue(config.Username)
	if config.Assignee != nil {
		state.ServiceNowConfiguration.Assignee = types.StringValue(*config.Assignee)
	} else {
		state.ServiceNowConfiguration.Assignee = types.StringNull()
	}
	if config.Impact != nil {
		state.ServiceNowConfiguration.Impact = types.StringValue(*config.Impact)
	} else {
		state.ServiceNowConfiguration.Impact = types.StringNull()
	}
	if config.Urgency != nil {
		state.ServiceNowConfiguration.Urgency = types.StringValue(*config.Urgency)
	} else {
		state.ServiceNowConfiguration.Urgency = types.StringNull()
	}
	if len(config.DictionaryOverrides) > 0 {
		var overrides []ServiceNowDictionaryOverrideModel
		for _, override := range config.DictionaryOverrides {
			overrideModel := ServiceNowDictionaryOverrideModel{
				Trigger: types.StringValue(override.Trigger),
			}
			if len(override.KeyValuePairs) > 0 {
				var kvPairs []ServiceNowKeyValuePairModel
				for _, kv := range override.KeyValuePairs {
					kvPairs = append(kvPairs, ServiceNowKeyValuePairModel{
						Key:   types.StringValue(kv.Key),
						Value: types.StringValue(fmt.Sprintf("%v", kv.Value)),
					})
				}
				overrideModel.KeyValuePairs = kvPairs
			}
			overrides = append(overrides, overrideModel)
		}
		state.ServiceNowConfiguration.DictionaryOverrides = overrides
	}
	return diags
}
