package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/microsoftgraph/msgraph-sdk-go/models"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"
)

type federatedIdentity struct {
	client *api.CamClient
}

type camFederatedIdentityCredentialModel struct {
	CamDeployedRegion     types.String `tfsdk:"cam_deployed_region"`
	ClientID              types.String `tfsdk:"application_id"`
	FederatedIdentityName types.String `tfsdk:"federated_identity_name"`
	ObjectID              types.String `tfsdk:"app_registration_object_id"`
	SubscriptionID        types.String `tfsdk:"subscription_id"`
	V1BusinessID          types.String `tfsdk:"v1_business_id"`
}

func NewFederatedIdentity() resource.Resource {
	return &federatedIdentity{}
}

func (r *federatedIdentity) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_FEDERATED_IDENTITY
}

func (r *federatedIdentity) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Trend Micro Vision One CAM Azure Federated Identity Credential resource",
		Attributes: map[string]schema.Attribute{
			"v1_business_id": schema.StringAttribute{
				MarkdownDescription: "Business ID of the Trend Vision One account. Used for the Federated Identity Credential.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cam_deployed_region": schema.StringAttribute{
				MarkdownDescription: "The region where CAM is deployed. Used to determine the issuer URL for the Federated Identity Credential. The supported regions are `au`, `sg`, `us`, `in`, `jp`, `eu`, and `mea`.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
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
			"federated_identity_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Federated Identity.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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

func (r *federatedIdentity) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan camFederatedIdentityCredentialModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(plan.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[Federated Identity][Create] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	v1BusinessID := plan.V1BusinessID.ValueString()
	if v1BusinessID == "" {
		resp.Diagnostics.AddError("[Federated Identity][Create] Invalid Business ID", "V1 Business ID cannot be empty")
		return
	}

	objectId := plan.ObjectID.ValueStringPointer()
	if objectId == nil || *objectId == "" {
		resp.Diagnostics.AddError("[Federated Identity][Create] Invalid App Registration Object ID", "App Registration Object ID cannot be empty")
		return
	}

	camDeployedRegion := getDeployedRegion(plan.CamDeployedRegion.ValueString())

	fed := models.NewFederatedIdentityCredential()
	fed.SetName(toStringPointer(getFederatedIdentityName(plan.FederatedIdentityName.ValueString())))
	fed.SetDescription(toStringPointer(config.FEDERATED_CREDENTIALS_DESCRIPTION))
	fed.SetAudiences([]string{"api://AzureADTokenExchange"})
	fed.SetIssuer(toStringPointer(getIssuerURL(camDeployedRegion)))
	fed.SetSubject(toStringPointer(`urn:visionone:identity:` + camDeployedRegion + `:` + v1BusinessID + `:account/` + v1BusinessID))

	_, err = client.GraphClient.Applications().ByApplicationId(*objectId).FederatedIdentityCredentials().Post(ctx, fed, nil)
	if err != nil {
		resp.Diagnostics.AddError("[Federated Identity][Create] Failed to create federated identity credential", err.Error())
		return
	}

	plan.ClientID = types.StringValue(plan.ClientID.ValueString())
	plan.FederatedIdentityName = types.StringValue(getFederatedIdentityName(plan.FederatedIdentityName.ValueString()))
	plan.ObjectID = types.StringValue(plan.ObjectID.ValueString())
	plan.SubscriptionID = types.StringValue(subscriptionID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *federatedIdentity) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state camFederatedIdentityCredentialModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()
	if subscriptionID == "" {
		resp.Diagnostics.AddError("[Federated Identity][Read] Missing Subscription ID", "The Subscription ID is required to read the App Registration.")
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	app, err := client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).Get(ctx, nil)
	if err != nil {
		// Check if the error indicates the resource doesn't exist or has reference-property issues
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "reference-property objects are not present") {
			tflog.Info(ctx, fmt.Sprintf("[Federated Identity][Read] App registration %s no longer exists, removing from state", state.ObjectID.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("[Federated Identity][Read] Failed to read app registration", err.Error())
		return
	}

	state.CamDeployedRegion = types.StringValue(state.CamDeployedRegion.ValueString())
	state.ClientID = types.StringValue(*app.GetAppId())
	state.FederatedIdentityName = types.StringValue(getFederatedIdentityName(state.FederatedIdentityName.ValueString()))
	state.ObjectID = types.StringValue(*app.GetId())
	state.SubscriptionID = types.StringValue(subscriptionID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *federatedIdentity) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan camFederatedIdentityCredentialModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state camFederatedIdentityCredentialModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(plan.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[Federated Identity][Update] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	app := models.NewApplication()

	_, err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).Patch(ctx, app, nil)
	if err != nil {
		resp.Diagnostics.AddError("[Federated Identity][Update] Failed to update app registration", err.Error())
		return
	}

	v1BusinessID := plan.V1BusinessID.ValueString()
	if v1BusinessID == "" {
		resp.Diagnostics.AddError("Invalid Business ID", "V1 Business ID cannot be empty")
		return
	}
	camDeployedRegion := getDeployedRegion(plan.CamDeployedRegion.ValueString())
	fed := models.NewFederatedIdentityCredential()
	fed.SetName(toStringPointer(getFederatedIdentityName(plan.FederatedIdentityName.ValueString())))
	fed.SetDescription(toStringPointer(config.FEDERATED_CREDENTIALS_DESCRIPTION))
	fed.SetAudiences([]string{"api://AzureADTokenExchange"})
	fed.SetIssuer(toStringPointer(getIssuerURL(camDeployedRegion)))
	fed.SetSubject(toStringPointer(`urn:visionone:identity:` + camDeployedRegion + `:` + v1BusinessID + `:account/` + v1BusinessID))
	_, err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).FederatedIdentityCredentials().Post(ctx, fed, nil)
	if err != nil {
		resp.Diagnostics.AddError("[Federated Identity][Update] Update Federated Identity Credential Failed", err.Error())
		return
	}

	state.CamDeployedRegion = types.StringValue(camDeployedRegion)
	state.ClientID = types.StringValue(*app.GetAppId())
	state.ObjectID = types.StringValue(*app.GetId())
	state.SubscriptionID = plan.SubscriptionID
	state.V1BusinessID = plan.V1BusinessID
	state.FederatedIdentityName = types.StringValue(getFederatedIdentityName(plan.FederatedIdentityName.ValueString()))

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *federatedIdentity) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state camFederatedIdentityCredentialModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(state.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[Federated Identity][Delete] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Delete only the federated identity credential, not the entire app registration
	federatedIdentityName := getFederatedIdentityName(state.FederatedIdentityName.ValueString())
	err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).FederatedIdentityCredentials().ByFederatedIdentityCredentialId(federatedIdentityName).Delete(ctx, nil)
	if err != nil {
		// Check if the error indicates the resource doesn't exist
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			tflog.Info(ctx, fmt.Sprintf("[Federated Identity][Delete] Federated identity credential %s already deleted or does not exist", federatedIdentityName))
			return
		}
		resp.Diagnostics.AddError("[Federated Identity][Delete] Failed to delete federated identity credential", err.Error())
		return
	}
}

func (r *federatedIdentity) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	tflog.Debug(ctx, "[Federated Identity] Federated Identity resource configured successfully")
}

func getFederatedIdentityName(name string) string {
	if name == "" {
		return "v1-fed-credential"
	}
	return name
}

func getDeployedRegion(region string) string {
	if region == "" {
		return "us"
	}
	return region
}
