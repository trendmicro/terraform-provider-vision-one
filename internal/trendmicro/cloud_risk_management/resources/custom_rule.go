package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &customRuleResource{}
	_ resource.ResourceWithConfigure   = &customRuleResource{}
	_ resource.ResourceWithImportState = &customRuleResource{}
)

// NewCustomRuleResource is a helper function to simplify the provider implementation.
func NewCustomRuleResource() resource.Resource {
	return &customRuleResource{
		client: &api.CrmClient{},
	}
}

type customRuleResource struct {
	client *api.CrmClient
}

func (r *customRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_custom_rule"
}

func (r *customRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloud Risk Management custom rule.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique ID of the custom rule.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The custom rule name (max 255 characters).",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The custom rule description (max 255 characters).",
				Required:            true,
			},
			"categories": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Categories of the custom rule. Allowed values: security, cost-optimisation, reliability, performance-efficiency, operational-excellence, sustainability.",
				Required:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"risk_level": schema.StringAttribute{
				MarkdownDescription: "The risk level. Allowed values: LOW, MEDIUM, HIGH, VERY_HIGH, EXTREME.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("LOW", "MEDIUM", "HIGH", "VERY_HIGH", "EXTREME"),
				},
			},
			"cloud_provider": schema.StringAttribute{
				MarkdownDescription: "The cloud provider. Allowed values: aws, azure, gcp.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("aws", "azure", "gcp"),
				},
			},
			"remediation_note": schema.StringAttribute{
				MarkdownDescription: "The remediation notes for the custom rule (max 1000 characters).",
				Optional:            true,
			},
			"resolution_reference_link": schema.StringAttribute{
				MarkdownDescription: "A reference link for resolution guidance.",
				Optional:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the rule is enabled or not.",
				Required:            true,
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The cloud service ID.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_type": schema.StringAttribute{
				MarkdownDescription: "The type of resource this custom rule applies to (max 100 characters).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "The slug of the custom rule. The system uses the slug to form the rule ID (max 200 characters).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"attribute": schema.ListNestedBlock{
				MarkdownDescription: "The attributes of the resource data to be evaluated.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the attribute.",
							Required:            true,
						},
						"path": schema.StringAttribute{
							MarkdownDescription: "The path to the attribute in the resource data.",
							Required:            true,
						},
						"required": schema.BoolAttribute{
							MarkdownDescription: "Whether this attribute is required.",
							Required:            true,
						},
					},
				},
			},
			"event_rule": schema.ListNestedBlock{
				MarkdownDescription: "The events to be evaluated by the custom rule.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{
							MarkdownDescription: "The description of the event rule.",
							Required:            true,
						},
					},
					Blocks: map[string]schema.Block{
						"conditions": schema.SingleNestedBlock{
							MarkdownDescription: "The conditions for event evaluation.",
							Attributes: map[string]schema.Attribute{
								"operator": schema.StringAttribute{
									MarkdownDescription: "Logical operator. Allowed values: all, any.",
									Optional:            true,
									Validators: []validator.String{
										stringvalidator.OneOf("all", "any"),
									},
								},
							},
							Blocks: map[string]schema.Block{
								"condition": schema.ListNestedBlock{
									MarkdownDescription: "List of conditions to evaluate.",
									NestedObject: schema.NestedBlockObject{
										Attributes: map[string]schema.Attribute{
											"operator": schema.StringAttribute{
												MarkdownDescription: "The operator to evaluate the input of the condition.\n\nAvailable operators by category:\n- Regex: pattern\n- String: equal, notEqual, lessThan, lessThanInclusive, greaterThan, greaterThanInclusive\n- Array: in, notIn, contains, doesNotContain\n- Nullish: isNullOrUndefined\n- Date: dateComparison\n\nEnum: [pattern, equal, notEqual, lessThan, lessThanInclusive, greaterThan, greaterThanInclusive, in, notIn, contains, doesNotContain, isNullOrUndefined, dateComparison]",
												Required:            true,
											},
											"value": schema.StringAttribute{
												MarkdownDescription: "The value to compare against. Accepts a string or a jsonencode value (string, number, boolean, object, or array).",
												Required:            true,
											},
											"path": schema.StringAttribute{
												MarkdownDescription: "The path for evaluation.",
												Optional:            true,
											},
											"fact": schema.StringAttribute{
												MarkdownDescription: "The fact name for event rule conditions.",
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

func (r *customRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type customRuleModel struct {
	ID                      types.String             `tfsdk:"id"`
	Name                    types.String             `tfsdk:"name"`
	Description             types.String             `tfsdk:"description"`
	Categories              types.List               `tfsdk:"categories"`
	RiskLevel               types.String             `tfsdk:"risk_level"`
	CloudProvider           types.String             `tfsdk:"cloud_provider"`
	RemediationNote         types.String             `tfsdk:"remediation_note"`
	ResolutionReferenceLink types.String             `tfsdk:"resolution_reference_link"`
	Enabled                 types.Bool               `tfsdk:"enabled"`
	Service                 types.String             `tfsdk:"service"`
	ResourceType            types.String             `tfsdk:"resource_type"`
	Slug                    types.String             `tfsdk:"slug"`
	Attributes              []resourceAttributeModel `tfsdk:"attribute"`
	EventRules              []eventRuleModel         `tfsdk:"event_rule"`
}

type resourceAttributeModel struct {
	Name     types.String `tfsdk:"name"`
	Path     types.String `tfsdk:"path"`
	Required types.Bool   `tfsdk:"required"`
}

type eventRuleModel struct {
	Description types.String     `tfsdk:"description"`
	Conditions  *conditionsModel `tfsdk:"conditions"`
}

type conditionsModel struct {
	Operator types.String     `tfsdk:"operator"`
	Operands []conditionModel `tfsdk:"condition"`
}

type conditionModel struct {
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
	Path     types.String `tfsdk:"path"`
	Fact     types.String `tfsdk:"fact"`
}

func (r *customRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customRuleModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var categories []string
	diags = plan.Categories.ElementsAs(ctx, &categories, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	attributes := make([]cloud_risk_management_dto.ResourceAttribute, len(plan.Attributes))
	for i, attr := range plan.Attributes {
		attributes[i] = cloud_risk_management_dto.ResourceAttribute{
			Name:     attr.Name.ValueString(),
			Path:     attr.Path.ValueString(),
			Required: attr.Required.ValueBool(),
		}
	}

	eventRules := make([]cloud_risk_management_dto.EventRule, len(plan.EventRules))
	for i, er := range plan.EventRules {
		eventRules[i] = cloud_risk_management_dto.EventRule{
			Description: er.Description.ValueString(),
		}
		if er.Conditions != nil {
			eventRules[i].Conditions = convertConditionsToDTOForEventRule(er.Conditions)
		}
	}

	createReq := cloud_risk_management_dto.CreateCustomRuleRequest{
		Name:                    plan.Name.ValueString(),
		Description:             plan.Description.ValueString(),
		Categories:              categories,
		RiskLevel:               plan.RiskLevel.ValueString(),
		Provider:                plan.CloudProvider.ValueString(),
		RemediationNote:         plan.RemediationNote.ValueString(),
		ResolutionReferenceLink: plan.ResolutionReferenceLink.ValueString(),
		Enabled:                 plan.Enabled.ValueBool(),
		Service:                 plan.Service.ValueString(),
		ResourceType:            plan.ResourceType.ValueString(),
		Attributes:              attributes,
		EventRules:              eventRules,
		Slug:                    plan.Slug.ValueString(),
	}

	tflog.Debug(ctx, "Creating custom rule", map[string]interface{}{
		"name": createReq.Name,
	})

	customRule, err := r.client.CreateCustomRule(&createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating custom rule",
			"Could not create custom rule, unexpected error: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(customRule.ID)
	plan.Slug = types.StringValue(customRule.Slug)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *customRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customRuleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	customRule, err := r.client.GetCustomRule(state.ID.ValueString())
	if api.IsNotFoundError(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading custom rule",
			"Could not read custom rule ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.Name = types.StringValue(customRule.Name)
	state.Description = types.StringValue(customRule.Description)
	state.RiskLevel = types.StringValue(customRule.RiskLevel)
	state.CloudProvider = types.StringValue(customRule.Provider)
	state.RemediationNote = types.StringValue(customRule.RemediationNote)
	state.ResolutionReferenceLink = types.StringValue(customRule.ResolutionReferenceLink)
	state.Enabled = types.BoolValue(customRule.Enabled)
	state.Service = types.StringValue(customRule.Service)
	state.ResourceType = types.StringValue(customRule.ResourceType)

	categoriesAttr := make([]attr.Value, len(customRule.Categories))
	for i, cat := range customRule.Categories {
		categoriesAttr[i] = types.StringValue(cat)
	}
	categoriesList, diags := types.ListValue(types.StringType, categoriesAttr)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Categories = categoriesList

	stateAttributes := make([]resourceAttributeModel, len(customRule.Attributes))
	for i, attr := range customRule.Attributes {
		stateAttributes[i] = resourceAttributeModel{
			Name:     types.StringValue(attr.Name),
			Path:     types.StringValue(attr.Path),
			Required: types.BoolValue(attr.Required),
		}
	}
	state.Attributes = stateAttributes

	stateEventRules := make([]eventRuleModel, len(customRule.EventRules))
	for i, er := range customRule.EventRules {
		stateEventRules[i] = eventRuleModel{
			Description: types.StringValue(er.Description),
		}
		if er.Conditions != nil && (len(er.Conditions.Any) > 0 || len(er.Conditions.All) > 0) {
			stateEventRules[i].Conditions = convertConditionsFromDTOForEventRule(er.Conditions)
		}
	}
	state.EventRules = stateEventRules

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *customRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customRuleModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var categories []string
	diags = plan.Categories.ElementsAs(ctx, &categories, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	attributes := make([]cloud_risk_management_dto.ResourceAttribute, len(plan.Attributes))
	for i, attr := range plan.Attributes {
		attributes[i] = cloud_risk_management_dto.ResourceAttribute{
			Name:     attr.Name.ValueString(),
			Path:     attr.Path.ValueString(),
			Required: attr.Required.ValueBool(),
		}
	}

	eventRules := make([]cloud_risk_management_dto.EventRule, len(plan.EventRules))
	for i, er := range plan.EventRules {
		eventRules[i] = cloud_risk_management_dto.EventRule{
			Description: er.Description.ValueString(),
		}
		if er.Conditions != nil {
			eventRules[i].Conditions = convertConditionsToDTOForEventRule(er.Conditions)
		}
	}

	enabled := plan.Enabled.ValueBool()
	updateReq := cloud_risk_management_dto.UpdateCustomRuleRequest{
		Name:                    plan.Name.ValueString(),
		Description:             plan.Description.ValueString(),
		Categories:              categories,
		RiskLevel:               plan.RiskLevel.ValueString(),
		Provider:                plan.CloudProvider.ValueString(),
		RemediationNote:         plan.RemediationNote.ValueString(),
		ResolutionReferenceLink: plan.ResolutionReferenceLink.ValueString(),
		Enabled:                 &enabled,
		Service:                 plan.Service.ValueString(),
		ResourceType:            plan.ResourceType.ValueString(),
		Attributes:              attributes,
		EventRules:              eventRules,
	}

	err := r.client.UpdateCustomRule(plan.ID.ValueString(), &updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating custom rule",
			"Could not update custom rule, unexpected error: "+err.Error(),
		)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *customRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customRuleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteCustomRule(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting custom rule",
			"Could not delete custom rule, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *customRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// converts conditions for event rules using any/all format
func convertConditionsToDTOForEventRule(conditions *conditionsModel) *cloud_risk_management_dto.Conditions {
	operands := make([]cloud_risk_management_dto.Condition, len(conditions.Operands))
	for i, op := range conditions.Operands {
		condition := cloud_risk_management_dto.Condition{
			Operator: op.Operator.ValueString(),
		}
		if !op.Value.IsNull() && !op.Value.IsUnknown() {
			raw := strings.TrimSpace(op.Value.ValueString())
			if raw == "" {
				condition.Value = ""
			} else {
				var decoded interface{}
				if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
					condition.Value = decoded
				} else {
					condition.Value = raw
				}
			}
		}
		if !op.Fact.IsNull() && !op.Fact.IsUnknown() {
			condition.Fact = op.Fact.ValueString()
		}
		if !op.Path.IsNull() && !op.Path.IsUnknown() {
			condition.Path = op.Path.ValueString()
		}
		operands[i] = condition
	}

	result := &cloud_risk_management_dto.Conditions{}
	operator := conditions.Operator.ValueString()
	if operator == "all" || operator == "" {
		result.All = operands
	} else if operator == "any" {
		result.Any = operands
	}

	return result
}

// converts conditions from DTO back to Terraform model for event rules
func convertConditionsFromDTOForEventRule(conditions *cloud_risk_management_dto.Conditions) *conditionsModel {
	var operands []conditionModel
	var operator string

	if len(conditions.Any) > 0 {
		operator = "any"
		operands = make([]conditionModel, len(conditions.Any))
		for i, op := range conditions.Any {
			operands[i] = conditionModel{
				Operator: types.StringValue(op.Operator),
			}
			if op.Value != nil {
				operands[i].Value = types.StringValue(fmt.Sprintf("%v", op.Value))
			} else {
				operands[i].Value = types.StringNull()
			}
			if op.Path != "" {
				operands[i].Path = types.StringValue(op.Path)
			} else {
				operands[i].Path = types.StringNull()
			}
			if op.Fact != "" {
				operands[i].Fact = types.StringValue(op.Fact)
			} else {
				operands[i].Fact = types.StringNull()
			}
		}
	} else if len(conditions.All) > 0 {
		operator = "all"
		operands = make([]conditionModel, len(conditions.All))
		for i, op := range conditions.All {
			operands[i] = conditionModel{
				Operator: types.StringValue(op.Operator),
			}
			if op.Value != nil {
				operands[i].Value = types.StringValue(fmt.Sprintf("%v", op.Value))
			} else {
				operands[i].Value = types.StringNull()
			}
			if op.Path != "" {
				operands[i].Path = types.StringValue(op.Path)
			} else {
				operands[i].Path = types.StringNull()
			}
			if op.Fact != "" {
				operands[i].Fact = types.StringValue(op.Fact)
			} else {
				operands[i].Fact = types.StringNull()
			}
		}
	}

	return &conditionsModel{
		Operator: types.StringValue(operator),
		Operands: operands,
	}
}
