package resources

import (
	"context"
	"errors"
	"fmt"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	"terraform-provider-vision-one/pkg/dto"
	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &groupResource{}
	_ resource.ResourceWithConfigure   = &groupResource{}
	_ resource.ResourceWithImportState = &groupResource{}
)

type groupResource struct {
	client *api.CrmClient
}

type groupResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Tags types.Set    `tfsdk:"tags"`
}

func NewGroupResource() resource.Resource {
	return &groupResource{
		client: &api.CrmClient{},
	}
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_group"
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloud Risk Management Group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique ID of the group.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the group.",
				Required:            true,
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Tags associated with the group.",
				ElementType:         types.StringType,
				Optional:            true,
			},
		},
	}
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Create new group plan: %+v", plan))

	body := &cloud_risk_management_dto.CreateGroupRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		var tags []string
		diags = plan.Tags.ElementsAs(ctx, &tags, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Tags = tags
	}

	group, err := r.client.CreateGroup(body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Cloud Risk Management Group",
			"An error was encountered creating the Cloud Risk Management Group. "+
				"Please review the following error message for details: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(group.ID)

	fullGroup, err := r.client.GetGroup(group.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading created group",
			"Created group but failed to read it: "+err.Error(),
		)
		return
	}

	plan.Name = types.StringValue(fullGroup.Name)

	tagsValues := make([]attr.Value, len(fullGroup.Tags))
	for i, tag := range fullGroup.Tags {
		tagsValues[i] = types.StringValue(tag)
	}
	plan.Tags, diags = types.SetValue(types.StringType, tagsValues)
	resp.Diagnostics.Append(diags...)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.GetGroup(state.ID.ValueString())
	if errors.Is(err, dto.ErrorNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Risk Management Group",
			"Could not read group ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.Name = types.StringValue(group.Name)

	tagValues := make([]attr.Value, len(group.Tags))
	for i, tag := range group.Tags {
		tagValues[i] = types.StringValue(tag)
	}
	state.Tags, diags = types.SetValue(types.StringType, tagValues)
	resp.Diagnostics.Append(diags...)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Update group plan: %+v", plan))

	body := &cloud_risk_management_dto.UpdateGroupRequest{
		Name: plan.Name.ValueString(),
	}

	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		var tags []string
		diags = plan.Tags.ElementsAs(ctx, &tags, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Tags = tags
	}

	err := r.client.UpdateGroup(plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Cloud Risk Management Group",
			"Could not update group ID "+plan.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Refresh state
	group, err := r.client.GetGroup(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Group",
			"Updated group but failed to read it: "+err.Error(),
		)
		return
	}

	plan.Name = types.StringValue(group.Name)

	tagValues := make([]attr.Value, len(group.Tags))
	for i, tag := range group.Tags {
		tagValues[i] = types.StringValue(tag)
	}
	plan.Tags, diags = types.SetValue(types.StringType, tagValues)
	resp.Diagnostics.Append(diags...)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteGroup(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Cloud Risk Management Group",
			"Could not delete group ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
