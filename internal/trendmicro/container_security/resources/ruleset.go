package resources

import (
	"context"
	"errors"
	"fmt"
	"terraform-provider-visionone/internal/trendmicro/container_security/resources/config"

	"terraform-provider-visionone/internal/trendmicro"
	"terraform-provider-visionone/internal/trendmicro/container_security/api"
	"terraform-provider-visionone/pkg/dto"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &RulesetResource{}
	_ resource.ResourceWithConfigure   = &RulesetResource{}
	_ resource.ResourceWithImportState = &RulesetResource{}
)

func NewRulesetResource() resource.Resource {
	return &RulesetResource{
		client: &api.CsClient{},
	}
}

// ExampleResource defines the resources implementation.
type RulesetResource struct {
	client *api.CsClient
}

type RulesetResourceModel struct {
	Id              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Labels          []labelModel `tfsdk:"labels"`
	Rules           []ruleModel  `tfsdk:"rules"`
	CreatedDateTime types.String `tfsdk:"createdtime"`
	UpdatedDateTime types.String `tfsdk:"updatedtime"`
}

type labelModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type ruleModel struct {
	Id         types.String `tfsdk:"id"`
	Mitigation types.String `tfsdk:"mitigation"`
}

func (r *RulesetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_RULESET
}

func (r *RulesetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: config.RESOURCE_TYPE_RULESET_DESCRIPTION,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique ID assigned to this ruleset.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the ruleset.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the ruleset.",
				Optional:            true,
			},
			"labels": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "The key of the container object label.",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "The value of the container object label.",
							Required:            true,
						},
					},
				},
			},
			"rules": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique ID assigned to the rule.",
							Required:            true,
						},
						"mitigation": schema.StringAttribute{
							MarkdownDescription: "Enum:[ log, isolate, terminate ] Default to log.",
							Optional:            true,
						},
					},
				},
			},
			"createdtime": schema.StringAttribute{
				MarkdownDescription: "The time when the ruleset was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updatedtime": schema.StringAttribute{
				MarkdownDescription: "The time when the ruleset was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *RulesetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

	r.client.Client = client
}

func (r *RulesetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RulesetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rulesetRequest dto.CreateRulesetRequest
	rulesetRequest.Name = plan.Name.ValueString()
	if !plan.Description.IsNull() {
		rulesetRequest.Description = plan.Description.ValueString()
	}
	rulesetRequest.Labels = make([]dto.Label, 0)
	for _, label := range plan.Labels {
		rulesetRequest.Labels = append(rulesetRequest.Labels, dto.Label{
			Key:   label.Key.ValueString(),
			Value: label.Value.ValueString(),
		})
	}
	rulesetRequest.Rules = make([]dto.Rule, 0)
	for _, rule := range plan.Rules {
		rulesetRequest.Rules = append(rulesetRequest.Rules, dto.Rule{
			Id:         rule.Id.ValueString(),
			Mitigation: rule.Mitigation.ValueString(),
		})
	}

	createdRuleset, err := r.client.CreateRuleset(&rulesetRequest)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Create a ruleset",
			"An unexpected error occurred when creating the Container Security ruleset. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro CsClient Error: "+err.Error(),
		)
		return
	}

	plan.Id = types.StringValue(createdRuleset.Id)
	plan.CreatedDateTime = types.StringValue(createdRuleset.CreatedDateTime)
	plan.UpdatedDateTime = types.StringValue(createdRuleset.UpdatedDateTime)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *RulesetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RulesetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleset, err := r.client.GetRuleset(state.Id.ValueString())
	if err != nil {
		if errors.Is(err, dto.ErrorNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Unable to read ruleset",
			"Ruleset ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	state.Name = types.StringValue(ruleset.Name)
	if ruleset.Description != "" {
		state.Description = types.StringValue(ruleset.Description)
	}

	if ruleset.Labels != nil {
		state.Labels = make([]labelModel, 0)
		for _, label := range ruleset.Labels {
			state.Labels = append(state.Labels, labelModel{
				Key:   types.StringValue(label.Key),
				Value: types.StringValue(label.Value),
			})
		}
	}

	if ruleset.Rules != nil {
		state.Rules = make([]ruleModel, 0)
		for _, rule := range ruleset.Rules {
			state.Rules = append(state.Rules, ruleModel{
				Id:         types.StringValue(rule.Id),
				Mitigation: types.StringValue(rule.Mitigation),
			})
		}
	}

	state.CreatedDateTime = types.StringValue(ruleset.CreatedDateTime)
	state.UpdatedDateTime = types.StringValue(ruleset.UpdatedDateTime)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *RulesetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RulesetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rulesetRequest dto.CreateRulesetRequest
	rulesetRequest.Name = plan.Name.ValueString()
	if !plan.Description.IsNull() {
		rulesetRequest.Description = plan.Description.ValueString()
	}
	rulesetRequest.Labels = make([]dto.Label, 0)
	for _, label := range plan.Labels {
		rulesetRequest.Labels = append(rulesetRequest.Labels, dto.Label{
			Key:   label.Key.ValueString(),
			Value: label.Value.ValueString(),
		})
	}
	rulesetRequest.Rules = make([]dto.Rule, 0)
	for _, rule := range plan.Rules {
		rulesetRequest.Rules = append(rulesetRequest.Rules, dto.Rule{
			Id:         rule.Id.ValueString(),
			Mitigation: rule.Mitigation.ValueString(),
		})
	}

	ruleset, err := r.client.UpdateRuleset(plan.Id.ValueString(), &rulesetRequest)
	if err != nil {
		if errors.Is(err, dto.ErrorNotFound) {
			resp.Diagnostics.AddError(
				"Unable to found ruleset id "+plan.Id.ValueString(),
				"An unexpected error occurred when updating the Container Security ruleset. "+
					"If the error is not clear, please contact the provider developers.\n\n"+
					"TrendMicro Client Error: "+err.Error())
			return
		}

		resp.Diagnostics.AddError(
			"Unable to read ruleset",
			"Ruleset ID "+plan.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	plan.UpdatedDateTime = types.StringValue(ruleset.UpdatedDateTime)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *RulesetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RulesetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteRuleset(state.Id.ValueString())
	if err != nil {
		if errors.Is(err, dto.ErrorNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Unable to delete ruleset",
			"Ruleset ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}
}

func (r *RulesetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
