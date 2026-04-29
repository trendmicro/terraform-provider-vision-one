package utils

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ScanRuleBaseAttributes returns the shared scan rule attributes used by both
// profile and account rule settings resources.
func ScanRuleBaseAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "The rule ID.",
			Required:            true,
		},
		"provider": schema.StringAttribute{
			MarkdownDescription: "The cloud provider. Allowed values: aws, azure, gcp, oci, alibabaCloud.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf("aws", "azure", "gcp", "oci", "alibabaCloud"),
			},
		},
		"enabled": schema.BoolAttribute{
			MarkdownDescription: "Whether the rule is enabled.",
			Required:            true,
		},
		"risk_level": schema.StringAttribute{
			MarkdownDescription: "The risk level of the rule. Allowed values: LOW, MEDIUM, HIGH, VERY_HIGH, EXTREME.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf("LOW", "MEDIUM", "HIGH", "VERY_HIGH", "EXTREME"),
			},
		},
	}
}

// ExceptionsSchemaBlock returns the shared exceptions block used by both
// profile and account rule settings resources.
func ExceptionsSchemaBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "Rule exceptions configuration.",
		Attributes: map[string]schema.Attribute{
			"filter_tags": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of filter tags for exceptions.",
				Optional:            true,
			},
			"resource_ids": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of resource IDs for exceptions.",
				Optional:            true,
			},
		},
	}
}

// ExtraSettingsSchemaBlock returns the shared extra_settings block used by both
// profile and account rule settings resources.
func ExtraSettingsSchemaBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		MarkdownDescription: "Additional rule settings.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{
					MarkdownDescription: "The name of the setting.",
					Required:            true,
				},
				"type": schema.StringAttribute{
					MarkdownDescription: "The type of the setting. Allowed values: `multiple-string-values`, `multiple-object-values`, `choice-multiple-value`, `choice-single-value`, `countries`, `multiple-aws-account-values`, `multiple-ip-values`, `multiple-number-values`, `regions`, `ignored-regions`, `single-number-value`, `single-string-value`, `single-value-regex`, `ttl`, `multiple-vpc-gateway-mappings`, `tags`, `choice-multiple-value-with-tags`, `choice-multiple-value-with-risk-level`.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.OneOf(
							"multiple-string-values",
							"multiple-object-values",
							"choice-multiple-value",
							"choice-single-value",
							"countries",
							"multiple-aws-account-values",
							"multiple-ip-values",
							"multiple-number-values",
							"regions",
							"ignored-regions",
							"single-number-value",
							"single-string-value",
							"single-value-regex",
							"ttl",
							"multiple-vpc-gateway-mappings",
							"tags",
							"choice-multiple-value-with-tags",
							"choice-multiple-value-with-risk-level",
						),
					},
				},
				"value": schema.StringAttribute{
					MarkdownDescription: "Single value for the setting. For numeric types (`ttl`, `single-number-value`, `multiple-number-values`), the value is automatically converted to a number.",
					Optional:            true,
				},
				"value_set": schema.SetAttribute{
					ElementType:         types.StringType,
					MarkdownDescription: "Set of string values for simple types like multiple-string-values, multiple-ip-values, multiple-aws-account-values, multiple-number-values, regions, ignored-regions, tags, countries. For `multiple-number-values`, values are automatically converted to numbers.",
					Optional:            true,
				},
			},
			Blocks: map[string]schema.Block{
				"values": schema.ListNestedBlock{
					MarkdownDescription: "Multiple values for the setting.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"value": schema.StringAttribute{
								MarkdownDescription: "Value for the setting. For `multiple-object-values` type, use JSON string (or `jsonencode` function). For numeric types, values are automatically converted to numbers.",
								Optional:            true,
							},
							"enabled": schema.BoolAttribute{
								MarkdownDescription: "Enabled value for the setting.",
								Optional:            true,
							},
							"vpc_id": schema.StringAttribute{
								MarkdownDescription: "The VPC ID (only for multiple-vpc-gateway-mappings type).",
								Optional:            true,
							},
							"gateway_ids": schema.SetAttribute{
								ElementType:         types.StringType,
								MarkdownDescription: "List of gateway IDs (only for multiple-vpc-gateway-mappings type).",
								Optional:            true,
							},
							"customized_tags": schema.SetAttribute{
								ElementType:         types.StringType,
								MarkdownDescription: "List of customized tags (only for choice-multiple-value-with-tags type).",
								Optional:            true,
								Computed:            true,
							},
							"customized_risk_level": schema.StringAttribute{
								MarkdownDescription: "Customized risk level (only for choice-multiple-value-with-risk-level type). Allowed values: LOW, MEDIUM, HIGH, VERY_HIGH, EXTREME, NOT_CUSTOMIZED",
								Optional:            true,
								Computed:            true,
								Validators: []validator.String{
									stringvalidator.OneOf(
										"LOW",
										"MEDIUM",
										"HIGH",
										"VERY_HIGH",
										"EXTREME",
										"NOT_CUSTOMIZED",
									),
								},
							},
						},
					},
				},
			},
		},
	}
}
