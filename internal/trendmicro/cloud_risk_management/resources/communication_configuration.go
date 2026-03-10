package resources

import (
	"context"
	"errors"
	"fmt"
	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	"terraform-provider-vision-one/pkg/dto"
	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                     = &communicationConfigurationResource{}
	_ resource.ResourceWithConfigure        = &communicationConfigurationResource{}
	_ resource.ResourceWithImportState      = &communicationConfigurationResource{}
	_ resource.ResourceWithConfigValidators = &communicationConfigurationResource{}
)

// Channel type constants
const (
	channelTypeEmail      = "email"
	channelTypeSms        = "sms"
	channelTypeMsTeams    = "msTeams"
	channelTypeSlack      = "slack"
	channelTypeSns        = "amazonSns"
	channelTypePagerDuty  = "pagerDuty"
	channelTypeWebhook    = "webhook"
	channelTypeJira       = "jira"
	channelTypeZendesk    = "zendesk"
	channelTypeServiceNow = "serviceNow"
)

type communicationConfigurationResource struct {
	client *api.CrmClient
}

type CommunicationConfigurationResourceModel struct {
	ID           types.String `tfsdk:"id"`
	AccountID    types.String `tfsdk:"account_id"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Level        types.String `tfsdk:"level"`
	ChannelType  types.String `tfsdk:"channel_type"`
	ChannelLabel types.String `tfsdk:"channel_label"`
	Manual       types.Bool   `tfsdk:"manual"`

	ChecksFilter            *ChecksFilterModel            `tfsdk:"checks_filter"`
	EmailConfiguration      *EmailConfigurationModel      `tfsdk:"email_configuration"`
	SmsConfiguration        *SmsConfigurationModel        `tfsdk:"sms_configuration"`
	MsTeamsConfiguration    *MsTeamsConfigurationModel    `tfsdk:"ms_teams_configuration"`
	SlackConfiguration      *SlackConfigurationModel      `tfsdk:"slack_configuration"`
	SnsConfiguration        *SnsConfigurationModel        `tfsdk:"sns_configuration"`
	PagerDutyConfiguration  *PagerDutyConfigurationModel  `tfsdk:"pagerduty_configuration"`
	WebhookConfiguration    *WebhookConfigurationModel    `tfsdk:"webhook_configuration"`
	JiraConfiguration       *JiraConfigurationModel       `tfsdk:"jira_configuration"`
	ZendeskConfiguration    *ZendeskConfigurationModel    `tfsdk:"zendesk_configuration"`
	ServiceNowConfiguration *ServiceNowConfigurationModel `tfsdk:"servicenow_configuration"`
}

type ChecksFilterModel struct {
	Regions               types.Set `tfsdk:"regions"`
	Services              types.Set `tfsdk:"services"`
	RuleIDs               types.Set `tfsdk:"rule_ids"`
	Categories            types.Set `tfsdk:"categories"`
	RiskLevels            types.Set `tfsdk:"risk_levels"`
	Tags                  types.Set `tfsdk:"tags"`
	ComplianceStandardIDs types.Set `tfsdk:"compliance_standard_ids"`
	Statuses              types.Set `tfsdk:"statuses"`
}

type EmailConfigurationModel struct {
	UserIDs types.Set `tfsdk:"user_ids"`
}

type SmsConfigurationModel struct {
	UserIDs types.Set `tfsdk:"user_ids"`
}

type MsTeamsConfigurationModel struct {
	URL                 types.String `tfsdk:"url"`
	IncludeIntroducedBy types.Bool   `tfsdk:"include_introduced_by"`
	IncludeResource     types.Bool   `tfsdk:"include_resource"`
	IncludeTags         types.Bool   `tfsdk:"include_tags"`
	IncludeExtraData    types.Bool   `tfsdk:"include_extra_data"`
}

type SlackConfigurationModel struct {
	URL                 types.String `tfsdk:"url"`
	Channel             types.String `tfsdk:"channel"`
	IncludeIntroducedBy types.Bool   `tfsdk:"include_introduced_by"`
	IncludeResource     types.Bool   `tfsdk:"include_resource"`
	IncludeTags         types.Bool   `tfsdk:"include_tags"`
	IncludeExtraData    types.Bool   `tfsdk:"include_extra_data"`
}

type SnsConfigurationModel struct {
	Arn types.String `tfsdk:"arn"`
}

type PagerDutyConfigurationModel struct {
	ServiceName types.String `tfsdk:"service_name"`
	ServiceKey  types.String `tfsdk:"service_key"`
}

type WebhookConfigurationModel struct {
	URL           types.String         `tfsdk:"url"`
	SecurityToken types.String         `tfsdk:"security_token"`
	Headers       []WebhookHeaderModel `tfsdk:"headers"`
}

type WebhookHeaderModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type JiraConfigurationModel struct {
	URL        types.String `tfsdk:"url"`
	Username   types.String `tfsdk:"username"`
	APIToken   types.String `tfsdk:"api_token"`
	Project    types.String `tfsdk:"project"`
	Type       types.String `tfsdk:"type"`
	AssigneeID types.String `tfsdk:"assignee_id"`
	Priority   types.String `tfsdk:"priority"`
}

type ZendeskConfigurationModel struct {
	URL        types.String `tfsdk:"url"`
	Username   types.String `tfsdk:"username"`
	Password   types.String `tfsdk:"password"`
	APIToken   types.String `tfsdk:"api_token"`
	Type       types.String `tfsdk:"type"`
	Priority   types.String `tfsdk:"priority"`
	GroupID    types.Int64  `tfsdk:"group_id"`
	AssigneeID types.Int64  `tfsdk:"assignee_id"`
}

type ServiceNowConfigurationModel struct {
	Type                types.String                        `tfsdk:"type"`
	URL                 types.String                        `tfsdk:"url"`
	Username            types.String                        `tfsdk:"username"`
	Password            types.String                        `tfsdk:"password"`
	Assignee            types.String                        `tfsdk:"assignee"`
	Impact              types.String                        `tfsdk:"impact"`
	Urgency             types.String                        `tfsdk:"urgency"`
	DictionaryOverrides []ServiceNowDictionaryOverrideModel `tfsdk:"dictionary_overrides"`
}

type ServiceNowDictionaryOverrideModel struct {
	Trigger       types.String                  `tfsdk:"trigger"`
	KeyValuePairs []ServiceNowKeyValuePairModel `tfsdk:"key_value_pairs"`
}

type ServiceNowKeyValuePairModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

func NewCommunicationConfigurationResource() resource.Resource {
	return &communicationConfigurationResource{
		client: &api.CrmClient{},
	}
}

// Builds the channel configuration DTO and channel type
// based on which channel configuration block the user provided in their Terraform config.
func (m *CommunicationConfigurationResourceModel) BuildChannelConfiguration(ctx context.Context) (channelConfig any, channelType string, diags diag.Diagnostics) {
	if m.EmailConfiguration != nil {
		var userIDs []string
		diags = m.EmailConfiguration.UserIDs.ElementsAs(ctx, &userIDs, false)
		if diags.HasError() {
			return nil, "", diags
		}
		return cloud_risk_management_dto.EmailChannelConfiguration{
			UserIDs: userIDs,
		}, channelTypeEmail, diags
	}

	if m.SmsConfiguration != nil {
		var userIDs []string
		diags = m.SmsConfiguration.UserIDs.ElementsAs(ctx, &userIDs, false)
		if diags.HasError() {
			return nil, "", diags
		}
		return cloud_risk_management_dto.SmsChannelConfiguration{
			UserIDs: userIDs,
		}, channelTypeSms, diags
	}

	if m.MsTeamsConfiguration != nil {
		return cloud_risk_management_dto.MsTeamsChannelConfiguration{
			URL:                 m.MsTeamsConfiguration.URL.ValueString(),
			IncludeIntroducedBy: m.MsTeamsConfiguration.IncludeIntroducedBy.ValueBool(),
			IncludeResource:     m.MsTeamsConfiguration.IncludeResource.ValueBool(),
			IncludeTags:         m.MsTeamsConfiguration.IncludeTags.ValueBool(),
			IncludeExtraData:    m.MsTeamsConfiguration.IncludeExtraData.ValueBool(),
		}, channelTypeMsTeams, diags
	}

	if m.SlackConfiguration != nil {
		return cloud_risk_management_dto.SlackChannelConfiguration{
			URL:                 m.SlackConfiguration.URL.ValueString(),
			Channel:             m.SlackConfiguration.Channel.ValueString(),
			IncludeIntroducedBy: m.SlackConfiguration.IncludeIntroducedBy.ValueBool(),
			IncludeResource:     m.SlackConfiguration.IncludeResource.ValueBool(),
			IncludeTags:         m.SlackConfiguration.IncludeTags.ValueBool(),
			IncludeExtraData:    m.SlackConfiguration.IncludeExtraData.ValueBool(),
		}, channelTypeSlack, diags
	}

	if m.SnsConfiguration != nil {
		return cloud_risk_management_dto.SnsChannelConfiguration{
			Arn: m.SnsConfiguration.Arn.ValueString(),
		}, channelTypeSns, diags
	}

	if m.PagerDutyConfiguration != nil {
		return cloud_risk_management_dto.PagerDutyChannelConfiguration{
			ServiceName: m.PagerDutyConfiguration.ServiceName.ValueString(),
			ServiceKey:  m.PagerDutyConfiguration.ServiceKey.ValueString(),
		}, channelTypePagerDuty, diags
	}

	if m.WebhookConfiguration != nil {
		var headers []cloud_risk_management_dto.WebhookHeader
		for _, h := range m.WebhookConfiguration.Headers {
			headers = append(headers, cloud_risk_management_dto.WebhookHeader{
				Key:   h.Key.ValueString(),
				Value: h.Value.ValueString(),
			})
		}
		return cloud_risk_management_dto.WebhookChannelConfiguration{
			URL:           m.WebhookConfiguration.URL.ValueString(),
			SecurityToken: m.WebhookConfiguration.SecurityToken.ValueString(),
			Headers:       headers,
		}, channelTypeWebhook, diags
	}

	if m.JiraConfiguration != nil {
		return cloud_risk_management_dto.JiraChannelConfiguration{
			URL:        m.JiraConfiguration.URL.ValueString(),
			Username:   m.JiraConfiguration.Username.ValueString(),
			APIToken:   m.JiraConfiguration.APIToken.ValueString(),
			Project:    m.JiraConfiguration.Project.ValueString(),
			Type:       m.JiraConfiguration.Type.ValueString(),
			AssigneeID: m.JiraConfiguration.AssigneeID.ValueStringPointer(),
			Priority:   m.JiraConfiguration.Priority.ValueStringPointer(),
		}, channelTypeJira, diags
	}

	if m.ZendeskConfiguration != nil {
		config := cloud_risk_management_dto.ZendeskChannelConfiguration{
			URL:        m.ZendeskConfiguration.URL.ValueString(),
			Username:   m.ZendeskConfiguration.Username.ValueString(),
			Type:       m.ZendeskConfiguration.Type.ValueStringPointer(),
			Priority:   m.ZendeskConfiguration.Priority.ValueStringPointer(),
			GroupID:    m.ZendeskConfiguration.GroupID.ValueInt64Pointer(),
			AssigneeID: m.ZendeskConfiguration.AssigneeID.ValueInt64Pointer(),
		}
		// User provides either password or api_token, not both
		if !m.ZendeskConfiguration.Password.IsNull() {
			config.Password = m.ZendeskConfiguration.Password.ValueString()
		}
		if !m.ZendeskConfiguration.APIToken.IsNull() {
			config.APIToken = m.ZendeskConfiguration.APIToken.ValueString()
		}
		return config, channelTypeZendesk, diags
	}

	if m.ServiceNowConfiguration != nil {
		config := cloud_risk_management_dto.ServiceNowChannelConfiguration{
			Type:     m.ServiceNowConfiguration.Type.ValueString(),
			URL:      m.ServiceNowConfiguration.URL.ValueString(),
			Username: m.ServiceNowConfiguration.Username.ValueString(),
			Password: m.ServiceNowConfiguration.Password.ValueString(),
			Assignee: m.ServiceNowConfiguration.Assignee.ValueStringPointer(),
			Impact:   m.ServiceNowConfiguration.Impact.ValueStringPointer(),
			Urgency:  m.ServiceNowConfiguration.Urgency.ValueStringPointer(),
		}
		var overrides []cloud_risk_management_dto.ServiceNowDictionaryOverride
		for _, override := range m.ServiceNowConfiguration.DictionaryOverrides {
			dtoOverride := cloud_risk_management_dto.ServiceNowDictionaryOverride{
				Trigger: override.Trigger.ValueString(),
			}
			if len(override.KeyValuePairs) > 0 {
				var kvPairs []cloud_risk_management_dto.ServiceNowKeyValuePair
				for _, kv := range override.KeyValuePairs {
					kvPairs = append(kvPairs, cloud_risk_management_dto.ServiceNowKeyValuePair{
						Key:   kv.Key.ValueString(),
						Value: kv.Value.ValueString(),
					})
				}
				dtoOverride.KeyValuePairs = kvPairs
			}
			overrides = append(overrides, dtoOverride)
		}
		config.DictionaryOverrides = overrides
		return config, channelTypeServiceNow, diags
	}

	return nil, "", diags
}

// Builds the checks filter DTO from the user's Terraform config.
func (m *CommunicationConfigurationResourceModel) BuildChecksFilter(ctx context.Context, channelType string) (*cloud_risk_management_dto.ChecksFilterRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	if m.ChecksFilter == nil {
		return nil, diags
	}

	filter := &cloud_risk_management_dto.ChecksFilterRequest{}

	if !m.ChecksFilter.Regions.IsNull() {
		var regions []string
		diags.Append(m.ChecksFilter.Regions.ElementsAs(ctx, &regions, false)...)
		filter.Regions = regions
	}

	if !m.ChecksFilter.Services.IsNull() {
		var services []string
		diags.Append(m.ChecksFilter.Services.ElementsAs(ctx, &services, false)...)
		filter.Services = services
	}

	if !m.ChecksFilter.RuleIDs.IsNull() {
		var ruleIDs []string
		diags.Append(m.ChecksFilter.RuleIDs.ElementsAs(ctx, &ruleIDs, false)...)
		filter.RuleIDs = ruleIDs
	}

	if !m.ChecksFilter.Categories.IsNull() {
		var categories []string
		diags.Append(m.ChecksFilter.Categories.ElementsAs(ctx, &categories, false)...)
		filter.Categories = categories
	}

	if !m.ChecksFilter.RiskLevels.IsNull() {
		var riskLevels []string
		diags.Append(m.ChecksFilter.RiskLevels.ElementsAs(ctx, &riskLevels, false)...)
		filter.RiskLevels = riskLevels
	}

	if !m.ChecksFilter.Tags.IsNull() {
		var tags []string
		diags.Append(m.ChecksFilter.Tags.ElementsAs(ctx, &tags, false)...)
		filter.Tags = tags
	}

	if !m.ChecksFilter.ComplianceStandardIDs.IsNull() {
		var complianceStandardIDs []string
		diags.Append(m.ChecksFilter.ComplianceStandardIDs.ElementsAs(ctx, &complianceStandardIDs,
			false)...)
		filter.ComplianceStandardIDs = complianceStandardIDs
	}

	if channelType == channelTypeSns || channelType == channelTypeWebhook {
		if !m.ChecksFilter.Statuses.IsNull() {
			var statuses []string
			diags.Append(m.ChecksFilter.Statuses.ElementsAs(ctx, &statuses, false)...)
			filter.Statuses = statuses
		}
	}

	return filter, diags
}

// requiresReplaceIfConfigBlockAddedOrRemoved triggers replacement only when the config block
// is added (null → value) or removed (value → null). Updates within the block are allowed.
func requiresReplaceIfConfigBlockAddedOrRemoved() planmodifier.Object {
	return objectplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
			if req.StateValue.IsNull() != req.PlanValue.IsNull() {
				resp.RequiresReplace = true
			}
		},
		"Adding or removing channel configuration requires resource replacement",
		"Adding or removing channel configuration requires resource replacement",
	)
}

func ptr[T any](v T) *T {
	return &v
}

func (r *communicationConfigurationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_communication_configuration"
}

func (r *communicationConfigurationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = api.NewCrmClient(client.HostURL, client.BearerToken, client.ProviderVersion)
}

func (r *communicationConfigurationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cloud Risk Management Communication Configuration.\n\n" +
			"Exactly one channel configuration must be specified: `email_configuration`, `sms_configuration`, " +
			"`ms_teams_configuration`, `slack_configuration`, `sns_configuration`, `pagerduty_configuration`, " +
			"`webhook_configuration`, `jira_configuration`, `zendesk_configuration`, or `servicenow_configuration`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique ID of the communication configuration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the communication configuration is enabled",
				Required:            true,
			},
			"channel_type": schema.StringAttribute{
				MarkdownDescription: "The channel type. Automatically set based on the channel configuration block.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The CRM account ID. If provided, the configuration applies to that account only. If omitted, it applies globally to all accounts (company level).",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"channel_label": schema.StringAttribute{
				MarkdownDescription: "A label to distinguish between multiple instances of the same channel type.",
				Optional:            true,
			},
			"manual": schema.BoolAttribute{
				MarkdownDescription: "Whether to use manual mode. Available only for SNS and ticketing channels (ServiceNow, Jira, Zendesk).",
				Optional:            true,
			},
			"level": schema.StringAttribute{
				MarkdownDescription: "The communication configuration level (company or account).",
				Computed:            true,
			},
			"checks_filter": schema.SingleNestedAttribute{
				MarkdownDescription: "Filter to apply to checks for this communication configuration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"regions": schema.SetAttribute{
						MarkdownDescription: "Filter by cloud region.",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"services": schema.SetAttribute{
						MarkdownDescription: "Filter by cloud service.",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"rule_ids": schema.SetAttribute{
						MarkdownDescription: "Filter by specific rule ID.",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"categories": schema.SetAttribute{
						MarkdownDescription: "Filter by category.",
						ElementType:         types.StringType,
						Optional:            true,
						Validators: []validator.Set{
							setvalidator.ValueStringsAre(
								stringvalidator.OneOf("security", "cost-optimisation", "reliability", "performance-efficiency", "operational-excellence", "sustainability"),
							),
						},
					},
					"risk_levels": schema.SetAttribute{
						MarkdownDescription: "Filter by risk level (LOW, MEDIUM, HIGH, VERY_HIGH, EXTREME).",
						ElementType:         types.StringType,
						Optional:            true,
						Validators: []validator.Set{
							setvalidator.ValueStringsAre(
								stringvalidator.OneOf("LOW", "MEDIUM", "HIGH", "VERY_HIGH", "EXTREME"),
							),
						},
					},
					"tags": schema.SetAttribute{
						MarkdownDescription: "Filter by tag.",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"compliance_standard_ids": schema.SetAttribute{
						MarkdownDescription: "Filter by compliance standard ID.",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"statuses": schema.SetAttribute{
						MarkdownDescription: "Filter by check statuses (SUCCESS, FAILURE). Available only for webhook and sns communication configurations.",
						ElementType:         types.StringType,
						Optional:            true,
						Validators: []validator.Set{
							setvalidator.ValueStringsAre(
								stringvalidator.OneOf("SUCCESS", "FAILURE"),
							),
						},
					},
				},
			},
			"email_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Email channel configuration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"user_ids": schema.SetAttribute{
						MarkdownDescription: "List of user identifiers to receive notifications. Format: `{identifierId}#{companyId}`.",
						ElementType:         types.StringType,
						Required:            true,
					},
				},
			},
			"sms_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "SMS channel configuration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"user_ids": schema.SetAttribute{
						MarkdownDescription: "List of user identifiers to receive notifications. Format: `{identifierId}#{companyId}`.",
						ElementType:         types.StringType,
						Required:            true,
					},
				},
			},
			"ms_teams_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "MS Teams channel configuration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "The Microsoft Teams incoming webhook URL.",
						Required:            true,
						Sensitive:           true,
					},
					"include_introduced_by": schema.BoolAttribute{
						MarkdownDescription: "Whether to include information about what introduced the check in the notification.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"include_resource": schema.BoolAttribute{
						MarkdownDescription: "Whether to include information about the resource in the notification.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"include_tags": schema.BoolAttribute{
						MarkdownDescription: "Whether to include check tags in the notification.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"include_extra_data": schema.BoolAttribute{
						MarkdownDescription: "Whether to include extra data associated with a check in the notification.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},
			"slack_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Slack channel configuration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "The Slack incoming webhook URL.",
						Required:            true,
						Sensitive:           true,
					},
					"channel": schema.StringAttribute{
						MarkdownDescription: "The Slack channel to post to (e.g., #security-alerts).",
						Required:            true,
					},
					"include_introduced_by": schema.BoolAttribute{
						MarkdownDescription: "Whether to include information about what introduced the check in the notification. Defaults to false.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"include_resource": schema.BoolAttribute{
						MarkdownDescription: "Whether to include information about the resource in the notification. Defaults to false.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"include_tags": schema.BoolAttribute{
						MarkdownDescription: "Whether to include check tags in the notification. Defaults to false.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"include_extra_data": schema.BoolAttribute{
						MarkdownDescription: "Whether to include extra data associated with a check in the notification. Defaults to false.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},
			"sns_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Amazon SNS channel configuration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"arn": schema.StringAttribute{
						MarkdownDescription: "The Amazon SNS topic ARN.",
						Required:            true,
					},
				},
			},
			"pagerduty_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "PagerDuty channel configuration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"service_name": schema.StringAttribute{
						MarkdownDescription: "The PagerDuty service name.",
						Required:            true,
					},
					"service_key": schema.StringAttribute{
						MarkdownDescription: "The PagerDuty service integration key.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
			"webhook_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Webhook channel configuration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "The webhook URL to send notifications to.",
						Required:            true,
						Sensitive:           true,
					},
					"security_token": schema.StringAttribute{
						MarkdownDescription: "Secret token for HMAC-SHA256 webhook payload signing.",
						Optional:            true,
						Sensitive:           true,
					},
					"headers": schema.ListNestedAttribute{
						MarkdownDescription: "Custom headers to include in the webhook request.",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"key": schema.StringAttribute{
									MarkdownDescription: "The header name.",
									Required:            true,
								},
								"value": schema.StringAttribute{
									MarkdownDescription: "The header value.",
									Required:            true,
									Sensitive:           true,
								},
							},
						},
					},
				},
			},
			"jira_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Jira channel configuration for creating tickets.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "The Jira URL.",
						Required:            true,
					},
					"username": schema.StringAttribute{
						MarkdownDescription: "The Jira username.",
						Required:            true,
					},
					"api_token": schema.StringAttribute{
						MarkdownDescription: "The Jira API token.",
						Required:            true,
						Sensitive:           true,
					},
					"project": schema.StringAttribute{
						MarkdownDescription: "The Jira project key.",
						Required:            true,
					},
					"type": schema.StringAttribute{
						MarkdownDescription: "The Jira issue type (e.g., Bug, Task, Story).",
						Required:            true,
					},
					"assignee_id": schema.StringAttribute{
						MarkdownDescription: "The Jira assignee ID.",
						Optional:            true,
					},
					"priority": schema.StringAttribute{
						MarkdownDescription: "The Jira priority (e.g., High, Medium, Low).",
						Optional:            true,
					},
				},
			},
			"zendesk_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Zendesk channel configuration for creating tickets. Either `password` or `api_token` must be provided, but not both.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "The Zendesk URL.",
						Required:            true,
					},
					"username": schema.StringAttribute{
						MarkdownDescription: "The Zendesk username (agent email).",
						Required:            true,
					},
					"password": schema.StringAttribute{
						MarkdownDescription: "The Zendesk password. Either `password` or `api_token` must be provided.",
						Optional:            true,
						Sensitive:           true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("api_token"),
							),
						},
					},
					"api_token": schema.StringAttribute{
						MarkdownDescription: "The Zendesk API token. Either `password` or `api_token` must be provided.",
						Optional:            true,
						Sensitive:           true,
					},
					"type": schema.StringAttribute{
						MarkdownDescription: "The Zendesk ticket type.",
						Optional:            true,
					},
					"priority": schema.StringAttribute{
						MarkdownDescription: "The Zendesk ticket priority.",
						Optional:            true,
					},
					"group_id": schema.Int64Attribute{
						MarkdownDescription: "The Zendesk group ID.",
						Optional:            true,
					},
					"assignee_id": schema.Int64Attribute{
						MarkdownDescription: "The Zendesk assignee ID.",
						Optional:            true,
					},
				},
			},
			"servicenow_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "ServiceNow channel configuration for creating tickets.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					requiresReplaceIfConfigBlockAddedOrRemoved(),
				},
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "The ServiceNow ticket type. Must be `problem`, `incident`, or `configurationTestResult`.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("problem", "incident", "configurationTestResult"),
						},
					},
					"url": schema.StringAttribute{
						MarkdownDescription: "The ServiceNow URL.",
						Required:            true,
					},
					"username": schema.StringAttribute{
						MarkdownDescription: "The ServiceNow username.",
						Required:            true,
					},
					"password": schema.StringAttribute{
						MarkdownDescription: "The ServiceNow password.",
						Required:            true,
						Sensitive:           true,
					},
					"assignee": schema.StringAttribute{
						MarkdownDescription: "The assignee of the ServiceNow ticket.",
						Optional:            true,
					},
					"impact": schema.StringAttribute{
						MarkdownDescription: "The impact of the ServiceNow ticket.",
						Optional:            true,
					},
					"urgency": schema.StringAttribute{
						MarkdownDescription: "The urgency of the ServiceNow ticket.",
						Optional:            true,
					},
					"dictionary_overrides": schema.ListNestedAttribute{
						MarkdownDescription: "JSON payload overriding ticket creation POST body and resolution PATCH body.",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"trigger": schema.StringAttribute{
									MarkdownDescription: "The override action type. Must be `creation` or `resolution`.",
									Required:            true,
									Validators: []validator.String{
										stringvalidator.OneOf("creation", "resolution"),
									},
								},
								"key_value_pairs": schema.ListNestedAttribute{
									MarkdownDescription: "Key value pairs of overrides.",
									Optional:            true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"key": schema.StringAttribute{
												MarkdownDescription: "The override key.",
												Required:            true,
											},
											"value": schema.StringAttribute{
												MarkdownDescription: "The override value.",
												Required:            true,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *communicationConfigurationResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("email_configuration"),
			path.MatchRoot("sms_configuration"),
			path.MatchRoot("ms_teams_configuration"),
			path.MatchRoot("slack_configuration"),
			path.MatchRoot("sns_configuration"),
			path.MatchRoot("pagerduty_configuration"),
			path.MatchRoot("webhook_configuration"),
			path.MatchRoot("jira_configuration"),
			path.MatchRoot("zendesk_configuration"),
			path.MatchRoot("servicenow_configuration"),
		),
	}
}

func (r *communicationConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CommunicationConfigurationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Create new communication configuration plan: %+v", plan))

	body := &cloud_risk_management_dto.CreateCommunicationConfigurationRequest{
		Enabled: plan.Enabled.ValueBool(),
	}

	if !plan.AccountID.IsNull() && !plan.AccountID.IsUnknown() {
		body.AccountID = plan.AccountID.ValueString()
	}

	if !plan.ChannelLabel.IsNull() && !plan.ChannelLabel.IsUnknown() {
		body.ChannelLabel = plan.ChannelLabel.ValueString()
	}

	channelConfig, channelType, configDiags := plan.BuildChannelConfiguration(ctx)
	resp.Diagnostics.Append(configDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.ChannelConfiguration = channelConfig
	body.ChannelType = channelType

	if channelType == channelTypeSns || channelType == channelTypeServiceNow || channelType == channelTypeJira || channelType == channelTypeZendesk {
		body.Manual = plan.Manual.ValueBool()
	}

	checksFilter, filterDiags := plan.BuildChecksFilter(ctx, channelType)
	resp.Diagnostics.Append(filterDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.ChecksFilter = checksFilter

	createdConfig, err := r.client.CreateCommunicationConfiguration(body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Communication Configuration",
			"Could not create communication configuration: "+err.Error(),
		)
		return
	}
	plan.ID = types.StringValue(createdConfig.ID)

	fullConfig, err := r.client.GetCommunicationConfiguration(createdConfig.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Created Communication Configuration",
			"Created communication configuration but failed to read it: "+err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(mapCommunicationConfigToState(fullConfig, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *communicationConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CommunicationConfigurationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.GetCommunicationConfiguration(state.ID.ValueString())
	if errors.Is(err, dto.ErrorNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Communication Configuration",
			"Could not read communication configuration ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(mapCommunicationConfigToState(config, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *communicationConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CommunicationConfigurationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Update communication configuration plan: %+v", plan))

	body := &cloud_risk_management_dto.UpdateCommunicationConfigurationRequest{
		Enabled: ptr(plan.Enabled.ValueBool()),
	}

	if !plan.ChannelLabel.IsNull() && !plan.ChannelLabel.IsUnknown() {
		body.ChannelLabel = ptr(plan.ChannelLabel.ValueString())
	}

	channelConfig, channelType, configDiags := plan.BuildChannelConfiguration(ctx)
	resp.Diagnostics.Append(configDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.ChannelConfiguration = channelConfig

	if channelType == channelTypeSns || channelType == channelTypeServiceNow || channelType == channelTypeJira || channelType == channelTypeZendesk {
		body.Manual = ptr(plan.Manual.ValueBool())
	}

	checksFilter, filterDiags := plan.BuildChecksFilter(ctx, channelType)
	resp.Diagnostics.Append(filterDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.ChecksFilter = checksFilter

	tflog.Debug(ctx, fmt.Sprintf("Updating config with ID: %s", plan.ID.ValueString()))

	err := r.client.UpdateCommunicationConfiguration(plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Communication Configuration",
			"Could not update communication configuration ID "+plan.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Refresh state
	config, err := r.client.GetCommunicationConfiguration(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Communication Configuration",
			"Updated communication configuration but failed to read it: "+err.Error(),
		)
		return
	}

	diags = mapCommunicationConfigToState(config, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *communicationConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CommunicationConfigurationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteCommunicationConfiguration(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Communication Configuration",
			"Could not delete communication configuration ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}
}

func (r *communicationConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
