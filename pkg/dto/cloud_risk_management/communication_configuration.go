package cloud_risk_management_dto

import "encoding/json"

type CommunicationConfiguration struct {
	ID              string `json:"id,omitempty"`
	AccountID       string `json:"accountId,omitempty"`
	Enabled         bool   `json:"enabled"`
	Level           string `json:"level,omitempty"`
	ChannelType     string `json:"channelType,omitempty"`
	ChannelLabel    string `json:"channelLabel,omitempty"`
	CreatedBy       string `json:"createdBy,omitempty"`
	CreatedDateTime string `json:"createdDateTime,omitempty"`
	UpdatedDateTime string `json:"updatedDateTime,omitempty"`
	// Available only for ticketing channels (servicenow, jira, zendesk) and sns
	Manual bool `json:"manual,omitempty"`

	ChannelConfiguration json.RawMessage       `json:"channelConfiguration,omitempty"`
	ChecksFilter         *ChecksFilterResponse `json:"checksFilter,omitempty"`
}

type EmailChannelConfiguration struct {
	UserIDs []string `json:"users"`
}

type SmsChannelConfiguration struct {
	UserIDs []string `json:"users"`
}

type MsTeamsChannelConfiguration struct {
	URL                 string `json:"url"`
	IncludeIntroducedBy bool   `json:"includeIntroducedBy"`
	IncludeResource     bool   `json:"includeResource"`
	IncludeTags         bool   `json:"includeTags"`
	IncludeExtraData    bool   `json:"includeExtraData"`
}

type SlackChannelConfiguration struct {
	URL                 string `json:"url"`
	Channel             string `json:"channel"`
	IncludeIntroducedBy bool   `json:"includeIntroducedBy"`
	IncludeResource     bool   `json:"includeResource"`
	IncludeTags         bool   `json:"includeTags"`
	IncludeExtraData    bool   `json:"includeExtraData"`
}

type SnsChannelConfiguration struct {
	Arn string `json:"arn"`
}

type PagerDutyChannelConfiguration struct {
	ServiceName string `json:"serviceName"`
	ServiceKey  string `json:"serviceKey"`
}

type WebhookChannelConfiguration struct {
	URL           string          `json:"url"`
	SecurityToken string          `json:"securityToken,omitempty"`
	Headers       []WebhookHeader `json:"headers,omitempty"`
}

type WebhookHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type JiraChannelConfiguration struct {
	URL        string  `json:"url"`
	Username   string  `json:"username,omitempty"`
	APIToken   string  `json:"apiToken,omitempty"`
	Project    string  `json:"project"`
	Type       string  `json:"type"`
	AssigneeID *string `json:"assigneeId"`
	Priority   *string `json:"priority"`
}

type ZendeskChannelConfiguration struct {
	URL        string  `json:"url"`
	Username   string  `json:"username"`
	Password   string  `json:"password,omitempty"`
	APIToken   string  `json:"apiToken,omitempty"`
	Type       *string `json:"type"`
	Priority   *string `json:"priority"`
	GroupID    *int64  `json:"groupId"`
	AssigneeID *int64  `json:"assigneeId"`
}

type ServiceNowChannelConfiguration struct {
	Type                string                         `json:"type"`
	URL                 string                         `json:"url"`
	Username            string                         `json:"username"`
	Password            string                         `json:"password,omitempty"`
	Assignee            *string                        `json:"assignee"`
	Impact              *string                        `json:"impact"`
	Urgency             *string                        `json:"urgency"`
	DictionaryOverrides []ServiceNowDictionaryOverride `json:"dictionaryOverrides"`
}

type ServiceNowDictionaryOverride struct {
	Trigger       string                   `json:"trigger"`
	KeyValuePairs []ServiceNowKeyValuePair `json:"keyValuePairs,omitempty"`
}

type ServiceNowKeyValuePair struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type ChecksFilterRequest struct {
	Regions               []string `json:"regions"`
	Services              []string `json:"services"`
	RuleIDs               []string `json:"ruleIds"`
	Categories            []string `json:"categories"`
	RiskLevels            []string `json:"riskLevels"`
	Tags                  []string `json:"tags"`
	ComplianceStandardIDs []string `json:"complianceStandardIds"`
	// Available only for webhook and sns communication configurations
	Statuses []string `json:"statuses"`
}

type ChecksFilterResponse struct {
	Regions             []string             `json:"regions"`
	Services            []string             `json:"services"`
	RuleIDs             []string             `json:"ruleIds"`
	Categories          []string             `json:"categories"`
	RiskLevels          []string             `json:"riskLevels"`
	Tags                []string             `json:"tags"`
	ComplianceStandards []ComplianceStandard `json:"complianceStandards"`
	// Available only for webhook and sns communication configurations
	Statuses []string `json:"statuses"`
}

type ComplianceStandard struct {
	ID string `json:"id"`
}

type CreateCommunicationConfigurationRequest struct {
	AccountID            string               `json:"accountId,omitempty"`
	Enabled              bool                 `json:"enabled"`
	ChannelType          string               `json:"channelType"`
	ChannelLabel         string               `json:"channelLabel,omitempty"`
	ChannelConfiguration any                  `json:"channelConfiguration"`
	ChecksFilter         *ChecksFilterRequest `json:"checksFilter,omitempty"`
	// Available only for ticketing channels (servicenow, jira, zendesk) and sns
	Manual bool `json:"manual,omitempty"`
}

type UpdateCommunicationConfigurationRequest struct {
	Enabled              *bool                `json:"enabled,omitempty"`
	ChannelLabel         *string              `json:"channelLabel"`
	ChannelConfiguration any                  `json:"channelConfiguration,omitempty"`
	ChecksFilter         *ChecksFilterRequest `json:"checksFilter"`
	// Available only for ticketing channels (servicenow, jira, zendesk) and sns
	Manual *bool `json:"manual,omitempty"`
}

type MultiStatusResponseItem struct {
	Status  int                         `json:"status"`
	Headers []MultiStatusResponseHeader `json:"headers,omitempty"` // Appears only on 201 status
	Body    json.RawMessage             `json:"body,omitempty"`    // Appears only on 400 or 404
}

type MultiStatusResponseHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
