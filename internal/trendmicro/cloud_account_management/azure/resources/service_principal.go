package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	msgraph "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"
)

type servicePrincipal struct {
	client *api.CamClient
}

type camServicePrincipalModel struct {
	ClientID       types.String `tfsdk:"application_id"`
	DisplayName    types.String `tfsdk:"display_name"`
	ObjectID       types.String `tfsdk:"app_registration_object_id"`
	PrincipalID    types.String `tfsdk:"principal_id"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	Tags           []string     `tfsdk:"tags"`
}

func NewServicePrincipal() resource.Resource {
	return &servicePrincipal{}
}

func (r *servicePrincipal) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_SERVICE_PRINCIPAL
}

func (r *servicePrincipal) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Trend Micro Vision One CAM Azure Service Principal resource",
		Attributes: map[string]schema.Attribute{
			"application_id": schema.StringAttribute{
				MarkdownDescription: "The Application ID (Client ID) of the Azure App Registration.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_registration_object_id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier of the Azure App Registration.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Tags to associate with the Service Principal.",
				Optional:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name of the Azure App Registration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal_id": schema.StringAttribute{
				MarkdownDescription: "Principal ID of the Azure App Registration service principal.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "Azure Subscription ID that will be connected to Trend Vision One.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *servicePrincipal) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan camServicePrincipalModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(plan.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[Service Principal][Create] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	displayName := getDisplayName(plan.DisplayName, subscriptionID)

	// Validate that the app registration exists before creating service principal
	_, err = client.GraphClient.Applications().ByApplicationId(plan.ObjectID.ValueString()).Get(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("[Service Principal][Create] App registration not found", fmt.Sprintf("The app registration with object ID %s does not exist: %s", plan.ObjectID.ValueString(), err.Error()))
		return
	}

	var bareboneVersion string
	if len(plan.Tags) > 0 {
		bareboneVersion = plan.Tags[0]
		tflog.Info(ctx, fmt.Sprintf("[Service Principal] Using provided barebone version: %s", bareboneVersion))
	} else {
		// throw a error if the user does not provide a version
		resp.Diagnostics.AddError("[Service Principal][Create] Missing barebone version", "The barebone version is required to create the Service Principal. Please provide a version in the `tags` attribute, e.g., `Version:2.0.1842`.")
		return
	}

	servicePrincipalID, err := r.createServicePrincipal(ctx, client.GraphClient, plan.ClientID.ValueStringPointer(), &bareboneVersion)
	if err != nil {
		resp.Diagnostics.AddError("[Service Principal][Create] Failed to create service principal", err.Error())
		return
	}

	plan.ClientID = types.StringValue(plan.ClientID.ValueString())
	plan.DisplayName = types.StringValue(displayName)
	plan.ObjectID = types.StringValue(plan.ObjectID.ValueString())
	plan.PrincipalID = types.StringValue(*servicePrincipalID)
	plan.SubscriptionID = types.StringValue(subscriptionID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *servicePrincipal) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state camServicePrincipalModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()
	if subscriptionID == "" {
		resp.Diagnostics.AddError("[Service Principal][Read] Missing Subscription ID", "The Subscription ID is required to read the Service Principal.")
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	app, err := client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).Get(ctx, nil)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "reference-property objects are not present") {
			tflog.Info(ctx, fmt.Sprintf("[Service Principal][Read] App registration %s no longer exists, removing from state", state.ObjectID.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("[Service Principal][Read] Failed to read app registration", err.Error())
		return
	}

	filter := fmt.Sprintf("appId eq '%s'", *app.GetAppId())
	servicePrincipals, err := client.GraphClient.ServicePrincipals().Get(ctx, &serviceprincipals.ServicePrincipalsRequestBuilderGetRequestConfiguration{
		QueryParameters: &serviceprincipals.ServicePrincipalsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("[Service Principal][Read] Failed to get service principals", err.Error())
		return
	}

	servicePrincipalList := servicePrincipals.GetValue()
	if len(servicePrincipalList) == 0 {
		resp.Diagnostics.AddError("[Service Principal][Read] Failed to find service principal", "Service principal not found")
		return
	}
	servicePrincipal := servicePrincipalList[0]

	state.ClientID = types.StringValue(*app.GetAppId())
	state.DisplayName = types.StringValue(*app.GetDisplayName())
	state.ObjectID = types.StringValue(*app.GetId())
	state.PrincipalID = types.StringValue(*servicePrincipal.GetId())
	state.SubscriptionID = types.StringValue(subscriptionID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *servicePrincipal) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan camServicePrincipalModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state camServicePrincipalModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(plan.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[Service Principal][Update] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	displayName := getDisplayName(plan.DisplayName, subscriptionID)
	var tags []string
	if len(tags) == 0 {
		tags = state.Tags
	} else {
		tflog.Info(ctx, fmt.Sprintf("[Service Principal] Using provided barebone version: %s", plan.Tags[0]))
		tags = plan.Tags
	}
	app := models.NewApplication()
	app.SetDisplayName(&displayName)
	app.SetTags(tags)

	_, err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).Patch(ctx, app, nil)
	if err != nil {
		resp.Diagnostics.AddError("Update Service Principal Failed", err.Error())
		return
	}

	state.ClientID = types.StringValue(*app.GetAppId())
	state.DisplayName = types.StringValue(displayName)
	state.ObjectID = types.StringValue(*app.GetId())
	state.PrincipalID = types.StringValue(state.PrincipalID.ValueString())
	state.SubscriptionID = plan.SubscriptionID

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *servicePrincipal) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state camServicePrincipalModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(state.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[Service Principal][Delete] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Delete the service principal using the principal ID, not the app registration
	err = client.GraphClient.ServicePrincipals().ByServicePrincipalId(state.PrincipalID.ValueString()).Delete(ctx, nil)
	if err != nil {
		// Check if the error indicates the resource doesn't exist
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			tflog.Info(ctx, fmt.Sprintf("[Service Principal][Delete] Service principal %s already deleted or does not exist", state.PrincipalID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("[Service Principal][Delete] Failed to delete service principal", err.Error())
		return
	}
}

func (r *servicePrincipal) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Type",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = &api.CamClient{
		Client: client,
	}
	tflog.Debug(ctx, "[Service Principal] Service Principal resource configured successfully")
}

func (r *servicePrincipal) createServicePrincipal(ctx context.Context, client *msgraph.GraphServiceClient, applicationID, templateVersion *string) (*string, error) {
	sp := models.NewServicePrincipal()
	sp.SetAppId(applicationID)
	sp.SetAppRoleAssignmentRequired(toBoolPointer(true))
	sp.SetServicePrincipalType(toStringPointer("Application"))
	sp.SetTags([]string{*templateVersion})

	createdSp, err := client.ServicePrincipals().Post(ctx, sp, nil)
	if err != nil {
		return nil, err
	}

	return createdSp.GetId(), nil
}
