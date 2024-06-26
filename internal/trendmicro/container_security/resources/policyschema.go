package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"terraform-provider-visionone/internal/trendmicro/container_security/resources/config"
)

const (
	IDSchemaName                         = "id"
	NameSchemaName                       = "name"
	DescriptionSchemaName                = "description"
	DefaultSchemaName                    = "default"
	RulesSchemaName                      = "rules"
	ExceptionsSchemaName                 = "exceptions"
	RuleTypeSchemaName                   = "type"
	RuleEnabledSchemaName                = "enabled"
	RuleActionSchemaName                 = "action"
	RuleMitigationSchemaName             = "mitigation"
	RuleStatementSchemaName              = "statement"
	RuleStatementPropertiesSchemaName    = "properties"
	RuleStatementPropertyKeySchemaName   = "key"
	RuleStatementPropertyValueSchemaName = "value"
	NamespacedListSchemaName             = "namespaced"
	NamespacedNameSchemaName             = "name"
	NamespacedNamespacesSchemaName       = "namespaces"
	RuntimeSchemaName                    = "runtime"
	RuntimeRulesetListSchemaName         = "rulesets"
	RuntimeRulesetIDSchemaName           = "id"
	XdrEnabledSchemaName                 = "xdr_enabled"
	CreatedDateTimeSchemaName            = "created_date_time"
	UpdateDateTimeSchemaName             = "updated_date_time"
	RulesetsUpdatedDateTimeSchemaName    = "rulesets_updated_date_time"

	RuleEnabledDefault    = true
	RuleActionDefault     = "none"
	RuleMitigationDefault = "none"
	XdrEnabledDefault     = true

	IDSchemaMarkdownDescription          = "The unique ID assigned to this policy."
	NameSchemaMarkdownDescription        = "A descriptive name for the policy."
	DescriptionSchemaMarkdownDescription = "A description of the policy."
	RulesSchemaMarkdownDescription       = "The set of policy rules. The rules are OR together."
	RuleTypeSchemaMarkdownDescription    = "The type of the policy rule." +
		"Enum: [podSecurityContext, containerSecurityContext, registry, image, tag, imagePath, vulnerabilities, cvssAttackVector, cvssAttackComplexity, cvssAvailability, checklists, checklistProfile, contents, malware, unscannedImage, podexec, portforward, capabilities]."
	RuleEnabledSchemaMarkdownDescription = "Enable the rule. " +
		"Default is \"true\"."
	RuleActionSchemaMarkdownDescription = "Action to take when the rule fails during the admission control phase. Action is ignored in exceptions. It returns none if there is no record. " +
		"Default is \"none\"." +
		"Enum: [block, log, none]."
	RuleMitigationSchemaMarkdownDescription = "Mitigation to take when the rule fails during runtime. Mitigation is ignored in exceptions. It returns none if there is no record." +
		"Default is \"none\"." +
		"Enum: [log, isolate, terminate, none]."
	RuleStatementPropertyKeySchemaMarkdownDescription = "See https://automation.trendmicro.com/xdr/api-v3#tag/Policies/paths/~1v3.0~1containerSecurity~1policies/post for more details."
	NamespacedListSchemaMarkdownDescription           = "The definition of all the policies."
	NamespacedNameSchemaMarkdownDescription           = "Descriptive name for the namespaced policy definition."
	NamespacedNamespacesSchemaMarkdownDescription     = "The namespaces that are associated with this policy definition."
	RuntimeSchemaMarkdownDescription                  = "The runtime properties of this policy."
	RuntimeRulesetListSchemaMarkdownDescription       = "The list of runtime rulesets associated to this policy."
	RuntimeRulesetIDSchemaMarkdownDescription         = "The ID of the ruleset"
	XdrEnabledSchemaMarkdownDescription               = "If true, enables XDR telemetry. " +
		"Default is \"true\"." +
		"Important: To use XDR telemetry, enable runtime security."
)

func generatePolicySchema() schema.Schema {
	return schema.Schema{
		Description: config.RESOURCE_TYPE_POLICY_DESCRIPTION,
		Attributes: map[string]schema.Attribute{
			IDSchemaName: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: IDSchemaMarkdownDescription,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			NameSchemaName: schema.StringAttribute{
				Required:            true,
				MarkdownDescription: NameSchemaMarkdownDescription,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			DescriptionSchemaName: schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: DescriptionSchemaMarkdownDescription,
			},
			DefaultSchemaName: schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					RulesSchemaName:      generatePolicyRuleListSchema(),
					ExceptionsSchemaName: generatePolicyExceptionListSchema(),
				},
			},
			NamespacedListSchemaName: generatePolicyNamespacedListSchema(),
			RuntimeSchemaName: schema.SingleNestedAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: RuntimeSchemaMarkdownDescription,
				Attributes: map[string]schema.Attribute{
					RuntimeRulesetListSchemaName: generatePolicyRuntimeRulesetListSchema(),
				},
			},
			XdrEnabledSchemaName: schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: XdrEnabledSchemaMarkdownDescription,
				Default:             booldefault.StaticBool(XdrEnabledDefault),
			},
			CreatedDateTimeSchemaName: schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			UpdateDateTimeSchemaName: schema.StringAttribute{
				Computed: true,
			},
			RulesetsUpdatedDateTimeSchemaName: schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func generatePolicyRuntimeRulesetListSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Required:            true,
		MarkdownDescription: RuntimeRulesetListSchemaMarkdownDescription,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				RuntimeRulesetIDSchemaName: schema.StringAttribute{
					Required:            true,
					MarkdownDescription: RuntimeRulesetIDSchemaMarkdownDescription,
				},
			},
		},
	}
}

func generatePolicyNamespacedListSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: NamespacedListSchemaMarkdownDescription,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				NamespacedNameSchemaName: schema.StringAttribute{
					Required:            true,
					MarkdownDescription: NamespacedNameSchemaMarkdownDescription,
				},
				NamespacedNamespacesSchemaName: schema.ListAttribute{
					Required:            true,
					MarkdownDescription: NamespacedNamespacesSchemaMarkdownDescription,
					ElementType:         types.StringType,
				},
				RulesSchemaName:      generatePolicyRuleListSchema(),
				ExceptionsSchemaName: generatePolicyExceptionListSchema(),
			},
		},
	}
}

func generatePolicyExceptionListSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: RulesSchemaMarkdownDescription,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				RuleTypeSchemaName: schema.StringAttribute{
					Required:            true,
					MarkdownDescription: RuleTypeSchemaMarkdownDescription,
				},
				RuleEnabledSchemaName: schema.BoolAttribute{
					Optional:            true,
					Computed:            true,
					MarkdownDescription: RuleEnabledSchemaMarkdownDescription,
					Default:             booldefault.StaticBool(RuleEnabledDefault),
				},
				RuleActionSchemaName: schema.StringAttribute{
					Optional:            true,
					Computed:            true,
					MarkdownDescription: RuleActionSchemaMarkdownDescription,
					Default:             stringdefault.StaticString(RuleActionDefault),
				},
				RuleMitigationSchemaName: schema.StringAttribute{
					Optional:            true,
					Computed:            true,
					MarkdownDescription: RuleMitigationSchemaMarkdownDescription,
					Default:             stringdefault.StaticString(RuleMitigationDefault),
				},
				RuleStatementSchemaName: schema.SingleNestedAttribute{
					Optional: true,
					PlanModifiers: []planmodifier.Object{
						objectplanmodifier.UseStateForUnknown(),
					},
					Attributes: map[string]schema.Attribute{
						RuleStatementPropertiesSchemaName: generatePolicyRuleStatementPropertyListSchema(),
					},
				},
			},
		},
	}
}

func generatePolicyRuleListSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Required:            true,
		MarkdownDescription: RulesSchemaMarkdownDescription,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				RuleTypeSchemaName: schema.StringAttribute{
					Required:            true,
					MarkdownDescription: RuleTypeSchemaMarkdownDescription,
				},
				RuleEnabledSchemaName: schema.BoolAttribute{
					Optional:            true,
					Computed:            true,
					MarkdownDescription: RuleEnabledSchemaMarkdownDescription,
					Default:             booldefault.StaticBool(RuleEnabledDefault),
				},
				RuleActionSchemaName: schema.StringAttribute{
					Optional:            true,
					Computed:            true,
					MarkdownDescription: RuleActionSchemaMarkdownDescription,
					Default:             stringdefault.StaticString(RuleActionDefault),
				},
				RuleMitigationSchemaName: schema.StringAttribute{
					Optional:            true,
					Computed:            true,
					MarkdownDescription: RuleMitigationSchemaMarkdownDescription,
					Default:             stringdefault.StaticString(RuleMitigationDefault),
				},
				RuleStatementSchemaName: schema.SingleNestedAttribute{
					Optional: true,
					Attributes: map[string]schema.Attribute{
						RuleStatementPropertiesSchemaName: generatePolicyRuleStatementPropertyListSchema(),
					},
				},
			},
		},
	}
}

func generatePolicyRuleStatementPropertyListSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Required: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				RuleStatementPropertyKeySchemaName: schema.StringAttribute{
					Required:            true,
					MarkdownDescription: RuleStatementPropertyKeySchemaMarkdownDescription,
				},
				RuleStatementPropertyValueSchemaName: schema.StringAttribute{
					Required: true,
				},
			},
		},
	}
}
