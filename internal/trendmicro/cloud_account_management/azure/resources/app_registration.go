package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
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

// resourceAccessEntryModel represents one entry in the requiredResourceAccess
// list — a permission UUID with its Microsoft Graph "type" ("Role" for
// Application permissions, "Scope" for Delegated permissions).
type resourceAccessEntryModel struct {
	ID   types.String `tfsdk:"id"`
	Type types.String `tfsdk:"type"`
}

type camAppRegistrationModel struct {
	ClientID               types.String               `tfsdk:"application_id"`
	DisplayName            types.String               `tfsdk:"display_name"`
	ObjectID               types.String               `tfsdk:"object_id"`
	SubscriptionID         types.String               `tfsdk:"subscription_id"`
	TenantID               types.String               `tfsdk:"tenant_id"`
	GraphResourceAccess    []resourceAccessEntryModel `tfsdk:"graph_resource_access"`
	AADGraphResourceAccess []resourceAccessEntryModel `tfsdk:"aad_graph_resource_access"`
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
					stringplanmodifier.UseStateForUnknown(),
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
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Tenant ID of the Azure App Registration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"graph_resource_access": schema.ListNestedAttribute{
				MarkdownDescription: "Microsoft Graph (`00000003-0000-0000-c000-000000000000`) `requiredResourceAccess` entries to declare on the App Registration. Each entry is `{id, type}` where `type` is `\"Role\"` for Application permissions or `\"Scope\"` for Delegated permissions. When omitted, the provider falls back to the built-in default set.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Permission UUID.",
						},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Permission type — `Role` (Application) or `Scope` (Delegated).",
						},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"aad_graph_resource_access": schema.ListNestedAttribute{
				MarkdownDescription: "Azure Active Directory Graph (`00000002-0000-0000-c000-000000000000`) `requiredResourceAccess` entries. Same shape as `graph_resource_access`. When omitted, the provider falls back to the built-in default (`User.Read` Delegated).",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Permission UUID.",
						},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Permission type — `Role` (Application) or `Scope` (Delegated).",
						},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
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
	app := r.buildApplicationModel(displayName, plan.GraphResourceAccess, plan.AADGraphResourceAccess)

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
	app := r.buildApplicationModel(displayName, plan.GraphResourceAccess, plan.AADGraphResourceAccess)

	_, err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).Patch(ctx, app, nil)
	if err != nil {
		resp.Diagnostics.AddError("[App Registration][Update] Update Failed", err.Error())
		return
	}

	state.ClientID = types.StringValue(state.ClientID.ValueString())
	state.DisplayName = types.StringValue(displayName)
	state.ObjectID = types.StringValue(state.ObjectID.ValueString())
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
		Client: client.WithTimeout(cam.CAMAPITimeout),
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

// Default Microsoft Graph permission UUIDs used when the caller does not pass
// `graph_resource_access`. Matches the legacy single-account App Registration
// manifest before the dynamic-permission migration.
var defaultGraphResourceAccess = []resourceAccessEntryModel{
	{ID: types.StringValue("e1fe6dd8-ba31-4d61-89e7-88639da4683d"), Type: types.StringValue("Scope")}, // User.Read (Delegated)
	{ID: types.StringValue("a154be20-db9c-4678-8ab7-66f6cc099a59"), Type: types.StringValue("Scope")}, // User.Read.All (Delegated)
	{ID: types.StringValue("7ab1d382-f21e-4acd-a863-ba3e13f7da61"), Type: types.StringValue("Role")},  // Directory.Read.All (Application)
	{ID: types.StringValue("df021288-bdef-4463-88db-98f22de89214"), Type: types.StringValue("Role")},  // User.Read.All (Application)
	{ID: types.StringValue("246dd0d5-5bd0-4def-940b-0421030a5b68"), Type: types.StringValue("Role")},  // Policy.Read.All (Application)
}

// Default Azure AD Graph permission UUIDs used when the caller does not pass
// `aad_graph_resource_access`.
var defaultAADGraphResourceAccess = []resourceAccessEntryModel{
	{ID: types.StringValue("311a71cc-e848-46a1-bdf8-97ff7156d8e6"), Type: types.StringValue("Scope")}, // User.Read (Delegated)
}

func (r *appRegistration) buildApplicationModel(
	displayName string,
	graphAccess []resourceAccessEntryModel,
	aadGraphAccess []resourceAccessEntryModel,
) models.Applicationable {
	if len(graphAccess) == 0 {
		graphAccess = defaultGraphResourceAccess
	}
	if len(aadGraphAccess) == 0 {
		aadGraphAccess = defaultAADGraphResourceAccess
	}

	app := models.NewApplication()
	app.SetDisplayName(&displayName)
	app.SetRequiredResourceAccess([]models.RequiredResourceAccessable{
		buildRequiredResourceAccess("00000002-0000-0000-c000-000000000000", aadGraphAccess),
		buildRequiredResourceAccess("00000003-0000-0000-c000-000000000000", graphAccess),
	})
	return app
}

func buildRequiredResourceAccess(resourceAppID string, entries []resourceAccessEntryModel) models.RequiredResourceAccessable {
	access := models.NewRequiredResourceAccess()
	access.SetResourceAppId(toStringPointer(resourceAppID))

	out := make([]models.ResourceAccessable, 0, len(entries))
	for _, e := range entries {
		out = append(out, buildResourceAccess(e.ID.ValueString(), e.Type.ValueString()))
	}
	access.SetResourceAccess(out)
	return access
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
	case "za":
		return "https://cloudaccounts-za.xdr.trendmicro.com"
	case "id":
		return "https://cloudaccounts-id.xdr.trendmicro.com"
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
