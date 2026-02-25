package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/microsoftgraph/msgraph-sdk-go/models"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"
)

type appRegistration struct {
	client *api.CamClient
}

type camAppRegistrationModel struct {
	ClientID       types.String `tfsdk:"application_id"`
	DisplayName    types.String `tfsdk:"display_name"`
	ObjectID       types.String `tfsdk:"object_id"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	TenantID       types.String `tfsdk:"tenant_id"`
}

func NewAppRegistration() resource.Resource {
	return &appRegistration{}
}

func (r *appRegistration) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_APP_REGISTRATION
}

func (r *appRegistration) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Trend Micro Vision One CAM Azure App Registration resource",
		Attributes: map[string]schema.Attribute{
			"application_id": schema.StringAttribute{
				MarkdownDescription: "The Application ID (Client ID) of the Azure App Registration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name of the Azure App Registration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"object_id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier of the Azure App Registration.",
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
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Tenant ID of the Azure App Registration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *appRegistration) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan camAppRegistrationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(plan.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[App Registration][Create] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	displayName := getDisplayName(plan.DisplayName, subscriptionID)
	app := r.buildApplicationModel(displayName)

	createdApp, err := client.GraphClient.Applications().Post(ctx, app, nil)
	if err != nil {
		resp.Diagnostics.AddError("[App Registration][Create] Creation Failed", err.Error())
		return
	}

	applicationID := createdApp.GetAppId()
	objectId := createdApp.GetId()
	if objectId == nil {
		resp.Diagnostics.AddError("[App Registration][Create] Creation Failed", "The created application does not have an Object ID.")
		return
	}

	tenantID, err := api.GetDefaultTenantID()
	if err != nil {
		resp.Diagnostics.AddError("[App Registration][Create] Failed to get tenant ID", err.Error())
		return
	}

	plan.ClientID = types.StringValue(*applicationID)
	plan.DisplayName = types.StringValue(displayName)
	plan.ObjectID = types.StringValue(*createdApp.GetId())
	plan.SubscriptionID = types.StringValue(subscriptionID)
	plan.TenantID = types.StringValue(tenantID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *appRegistration) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state camAppRegistrationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()
	if subscriptionID == "" {
		resp.Diagnostics.AddError("[App Registration][Read] Missing Subscription ID", "The Subscription ID is required to read the App Registration.")
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
			tflog.Info(ctx, fmt.Sprintf("[App Registration][Read] App registration %s no longer exists, removing from state", state.ObjectID.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("[App Registration][Read] Failed to read app registration", err.Error())
		return
	}

	state.ClientID = types.StringValue(*app.GetAppId())
	state.DisplayName = types.StringValue(*app.GetDisplayName())
	state.ObjectID = types.StringValue(*app.GetId())
	state.SubscriptionID = types.StringValue(subscriptionID)
	state.TenantID = types.StringValue(state.TenantID.ValueString())

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *appRegistration) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan camAppRegistrationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state camAppRegistrationModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(plan.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[App Registration][Update] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	displayName := getDisplayName(plan.DisplayName, subscriptionID)
	app := models.NewApplication()
	app.SetDisplayName(&displayName)

	_, err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).Patch(ctx, app, nil)
	if err != nil {
		resp.Diagnostics.AddError("Update App Registration Failed", err.Error())
		return
	}

	state.ClientID = types.StringValue(*app.GetAppId())
	state.DisplayName = types.StringValue(displayName)
	state.ObjectID = types.StringValue(*app.GetId())
	state.SubscriptionID = plan.SubscriptionID
	state.TenantID = plan.TenantID

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *appRegistration) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state camAppRegistrationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID, err := getSubscriptionID(state.SubscriptionID)
	if err != nil {
		resp.Diagnostics.AddError("[App Registration][Delete] Failed to get subscription", err.Error())
		return
	}

	client, diags := api.GetAzureClients(ctx, subscriptionID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).Delete(ctx, nil)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			tflog.Info(ctx, fmt.Sprintf("[App Registration][Delete] App registration %s already deleted or does not exist", state.ObjectID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("[App Registration][Delete] Failed to delete app registration", err.Error())
		return
	}
}

func (r *appRegistration) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	tflog.Debug(ctx, "[App Registration] App Registration resource configured successfully")
}

func buildResourceAccess(idStr, typ string) models.ResourceAccessable {
	ra := models.NewResourceAccess()
	guid := uuid.MustParse(idStr)
	ra.SetId(&guid)
	ra.SetTypeEscaped(toStringPointer(typ))
	return ra
}

func (r *appRegistration) buildApplicationModel(displayName string) models.Applicationable {
	app := models.NewApplication()
	app.SetDisplayName(&displayName)
	app.SetRequiredResourceAccess([]models.RequiredResourceAccessable{
		r.buildAADResourceAccess(),
		r.buildGraphResourceAccess(),
	})
	return app
}

func (r *appRegistration) buildAADResourceAccess() models.RequiredResourceAccessable {
	aadAccess := models.NewRequiredResourceAccess()
	aadAccess.SetResourceAppId(toStringPointer("00000002-0000-0000-c000-000000000000"))
	aadAccess.SetResourceAccess([]models.ResourceAccessable{
		buildResourceAccess("311a71cc-e848-46a1-bdf8-97ff7156d8e6", "Scope"),
	})
	return aadAccess
}

func (r *appRegistration) buildGraphResourceAccess() models.RequiredResourceAccessable {
	graphAccess := models.NewRequiredResourceAccess()
	graphAccess.SetResourceAppId(toStringPointer("00000003-0000-0000-c000-000000000000"))
	graphAccess.SetResourceAccess([]models.ResourceAccessable{
		buildResourceAccess("e1fe6dd8-ba31-4d61-89e7-88639da4683d", "Scope"),
		buildResourceAccess("a154be20-db9c-4678-8ab7-66f6cc099a59", "Scope"),
		buildResourceAccess("7ab1d382-f21e-4acd-a863-ba3e13f7da61", "Role"),
		buildResourceAccess("df021288-bdef-4463-88db-98f22de89214", "Role"),
		buildResourceAccess("246dd0d5-5bd0-4def-940b-0421030a5b68", "Role"),
	})
	return graphAccess
}

// getIssuerURL returns the issuer URL based on the CAM deployment region
func getIssuerURL(deployRegion string) string {
	switch deployRegion {
	case "au":
		return "https://cloudaccounts-au.xdr.trendmicro.com"
	case "sg":
		return "https://cloudaccounts-sg.xdr.trendmicro.com"
	case "us":
		return "https://cloudaccounts-us.xdr.trendmicro.com"
	case "in":
		return "https://cloudaccounts-in.xdr.trendmicro.com"
	case "jp":
		return "https://cloudaccounts-jp.xdr.trendmicro.com"
	case "eu":
		return "https://cloudaccounts-eu.xdr.trendmicro.com"
	case "mea":
		return "https://cloudaccounts-mea.xdr.trendmicro.com"
	case "ca":
		return "https://cloudaccounts-ca.xdr.trendmicro.com"
	case "uk":
		return "https://cloudaccounts-uk.xdr.trendmicro.com"
	default:
		return "https://cloudaccounts-us.xdr.trendmicro.com"
	}
}

func getSubscriptionID(subscriptionID types.String) (string, error) {
	if subscriptionID.IsNull() || subscriptionID.IsUnknown() {
		return api.GetDefaultSubscription()
	}
	return subscriptionID.ValueString(), nil
}

func getDisplayName(displayName types.String, subscriptionID string) string {
	if displayName.IsNull() || displayName.IsUnknown() {
		return config.AZURE_APP_REGISTRATION_NAME + subscriptionID + "-" + cam.GenerateRandomString(4)
	}
	return displayName.ValueString()
}

func toStringPointer(s string) *string {
	return &s
}

func toBoolPointer(b bool) *bool {
	return &b
}
