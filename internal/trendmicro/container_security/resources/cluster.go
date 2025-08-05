package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/container_security/api"
	"terraform-provider-vision-one/internal/trendmicro/container_security/resources/config"
	"terraform-provider-vision-one/pkg/dto"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &clusterResource{}
	_ resource.ResourceWithConfigure   = &clusterResource{}
	_ resource.ResourceWithImportState = &clusterResource{}
)

// NewClusterResource is a helper function to simplify the provider implementation.
func NewClusterResource() resource.Resource {
	return &clusterResource{
		client: &api.CsClient{},
	}
}

// clusterResource is the resources implementation.
type clusterResource struct {
	client *api.CsClient
}

// Metadata returns the resources type name.
func (r *clusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_CLUSTER
}

// Schema defines the schema for the resources.
func (r *clusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: config.RESOURCE_TYPE_CLUSTER_DESCRIPTION,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique ID of the cluster.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the cluster.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the cluster.",
				Optional:            true,
			},
			"policy_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the policy associated with the cluster.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"resource_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster of a different cloud provider.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The API key for cluster enrollment.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "The regional endpoint URL for Container Security.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_date_time": schema.StringAttribute{
				MarkdownDescription: "The time when the cluster was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_date_time": schema.StringAttribute{
				MarkdownDescription: "The time when the cluster was last updated.",
				Computed:            true,
			},
			"last_evaluated_date_time": schema.StringAttribute{
				MarkdownDescription: "Last time of the cluster was evaluated against the policy rules.",
				Computed:            true,
			},
			"group_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the group associated with the cluster. To get IDs of the groups within the user's management scope, use the Kubernetes cluster groups API to list these IDs.",
				Required:            true,
			},
			"namespaces": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "The namespaces of kubernetes you want to exclude from scanning. \nAccepted values: `calico-system`, `istio-system`, `kube-system`, `openshift*` Default value: `kube-system`",
				Optional:            true,
				Computed:            true,
				Default: setdefault.StaticValue(
					types.SetValueMust(types.StringType, []attr.Value{types.StringValue("kube-system")}),
				),
			},
			"runtime_security_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether runtime security is enabled for the cluster.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"vulnerability_scan_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether vulnerability scan is enabled for the cluster.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"malware_scan_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether malware scan is enabled for the cluster.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"secret_scan_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether secret scan is enabled for the cluster.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"inventory_collection": schema.BoolAttribute{
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"orchestrator": schema.StringAttribute{
				MarkdownDescription: "The orchestrator of the cluster.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"proxy": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The proxy server for in-cluster component connect to Vision One",
				Default: objectdefault.StaticValue(types.ObjectValueMust(
					map[string]attr.Type{
						"type":          types.StringType,
						"proxy_address": types.StringType,
						"port":          types.Int64Type,
						"username":      types.StringType,
						"password":      types.StringType,
						"https_proxy":   types.StringType,
					},
					map[string]attr.Value{
						"type":          types.StringNull(),
						"proxy_address": types.StringNull(),
						"port":          types.Int64Null(),
						"username":      types.StringNull(),
						"password":      types.StringNull(),
						"https_proxy":   types.StringNull(),
					},
				)),
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "The protocol of proxy. Accepted values: `HTTP` `SOCKS5`.",
						Required:            true,
					},
					"proxy_address": schema.StringAttribute{
						MarkdownDescription: "The address of proxy server.",
						Required:            true,
					},
					"port": schema.Int64Attribute{
						MarkdownDescription: "The port of proxy.",
						Required:            true,
					},
					"username": schema.StringAttribute{
						MarkdownDescription: "The username for proxy server authentication.",
						Optional:            true,
					},
					"password": schema.StringAttribute{
						MarkdownDescription: "The password for proxy server authentication.",
						Optional:            true,
						Sensitive:           true,
					},
					"https_proxy": schema.StringAttribute{
						MarkdownDescription: "The endpoint of proxy server.",
						Computed:            true,
					},
				},
			},
			"customizable_tags": schema.SetNestedAttribute{
				MarkdownDescription: "The custom tags and platform tags associated with the cluster. To obtain custom tags, refer to the following URL: https://automation.trendmicro.com/xdr/api-v3/#tag/Attack-Surface-Discovery/paths/~1v3.0~1asrm~1attackSurfaceCustomTags/get. However, platform tags will be provided through an additional API in the future. The difference between custom tags and platform tags is that properties of platform tags are defined by Trend Micro, while properties and values of custom tags can be created and updated by users.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The ID of the custom tag.",
							Required:            true,
						},
					},
				},
			},
		},
	}
}

// Create creates the resources and sets the initial Terraform state.
func (r *clusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dto.ClusterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Create new Cluster plan: %+v", plan))

	data := dto.CreateClusterRequest{
		Name:    plan.Name.ValueString(),
		GroupId: plan.GroupId.ValueString(),
	}
	if !plan.Description.IsNull() {
		data.Description = plan.Description.ValueString()
	}
	if !plan.PolicyId.IsNull() {
		data.PolicyId = plan.PolicyId.ValueString()
	}
	if !plan.ResourceId.IsNull() {
		data.ResourceId = plan.ResourceId.ValueString()
	}
	if !plan.CustomizableTagIDs.IsNull() {
		elements := plan.CustomizableTagIDs.Elements()
		tagIDs := make([]string, len(elements))
		for i, v := range elements {
			obj := v.(types.Object)
			tagID := obj.Attributes()["id"].(types.String)
			tagIDs[i] = tagID.ValueString()
		}
		data.CustomizableTagIDs = tagIDs
	}

	err := proxyHandler(&plan)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Create a Cluster",
			"Proxy configuration occurs error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Create new Cluster request: %v", data))
	apiResponse, err := r.client.CreateCluster(&data)

	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Create a Cluster",
			"An unexpected error occurred when creating the cluster. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	if apiResponse.ApiKey == "" || apiResponse.Endpoint == "" {
		tflog.Debug(ctx, "Create cluster API missing API key or Endpoint")
		resp.Diagnostics.AddError(
			"Unable to Create a Cluster",
			fmt.Sprintf("An unexpected error occurred when creating the cluster. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Create cluster API respond empty \nApikey: %v\nEndpoint: %v", apiResponse.ApiKey, apiResponse.Endpoint),
		)
		return
	}

	plan.ID = types.StringValue(apiResponse.ID)
	plan.Endpoint = types.StringValue(apiResponse.Endpoint)
	plan.ApiKey = types.StringValue(apiResponse.ApiKey)

	err = r.client.UpdateCurrentState(&plan)

	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Create a Cluster",
			"An unexpected error occurred when refreshing the cluster. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id string
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)

	var state dto.ClusterResourceModel
	req.State.Get(ctx, &state)
	state.ID = types.StringValue(id)

	err := r.client.UpdateCurrentState(&state)
	if errors.Is(err, dto.ErrorNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Read the Cluster",
			"An unexpected error occurred when reading the cluster. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	// assign default value for user
	state.Namespaces = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("kube-system")})

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resources and sets the updated Terraform state on success.
func (r *clusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dto.ClusterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := proxyHandler(&plan)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Update the Cluster",
			"Proxy configuration occurs error: "+err.Error(),
		)
		return
	}

	updateRequest := dto.UpdateClusterRequest{
		GroupId: plan.GroupId.ValueString(),
	}
	if !plan.Description.IsNull() {
		updateRequest.Description = plan.Description.ValueString()
	}
	if !plan.PolicyId.IsNull() {
		updateRequest.PolicyId = plan.PolicyId.ValueString()
	}
	if !plan.ResourceId.IsNull() {
		updateRequest.ResourceId = plan.ResourceId.ValueString()
	}
	if !plan.CustomizableTagIDs.IsNull() {
		elements := plan.CustomizableTagIDs.Elements()
		tagIDs := make([]string, len(elements))
		for i, v := range elements {
			obj := v.(types.Object)
			tagID := obj.Attributes()["id"].(types.String)
			tagIDs[i] = tagID.ValueString()
		}
		updateRequest.CustomizableTagIDs = tagIDs
	}

	err = r.client.UpdateCluster(plan.ID.ValueString(), &updateRequest)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Update the Cluster",
			"An unexpected error occurred when updating the cluster. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	err = r.client.UpdateCurrentState(&plan)

	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Update the Cluster",
			"An unexpected error occurred when refreshing the cluster. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resources and removes the Terraform state on success.
func (r *clusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dto.ClusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteClusterRequest := dto.DeleteClusterRequest{
		ID: state.ID.ValueString(),
	}

	err := r.client.DeleteCluster(&deleteClusterRequest)
	if errors.Is(err, dto.ErrorNotFound) {
		return
	}

	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Delete the Cluster",
			"An unexpected error occurred when reading the cluster. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"TrendMicro Client: "+err.Error(),
		)
		return
	}
}

// Configure adds the provider configured client to the resources.
func (r *clusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)

	tflog.SetField(ctx, "api client", client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *trendmicro.CsClient, got: %T. Please report this issue to the provider developers. Message: %v", req.ProviderData, client),
		)

		return
	}
	r.client.Client = client
}

func (r *clusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func proxyHandler(model *dto.ClusterResourceModel) error {
	if model.Proxy.Type.IsNull() {
		model.Proxy.HttpsProxy = types.StringNull()
		return nil
	}

	var endpoint string
	proxyType := strings.ToLower(model.Proxy.Type.ValueString())

	if proxyType == dto.ProxyTypeHttp {
		endpoint = fmt.Sprintf("%s://%s:%d", dto.ProxyTypeHttp, model.Proxy.ProxyAddress.ValueString(), model.Proxy.Port.ValueInt64())
	} else if proxyType == dto.ProxyTypeSocks5 {
		endpoint = fmt.Sprintf("%s://%s:%d", dto.ProxyTypeSocks5, model.Proxy.ProxyAddress.ValueString(), model.Proxy.Port.ValueInt64())
	} else {
		return dto.InvalidProxyType
	}
	model.Proxy.HttpsProxy = types.StringValue(endpoint)
	return nil
}
