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
	msgraph "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/oauth2permissiongrants"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"
)

// Well-known Microsoft first-party app IDs (same across all tenants).
const (
	// microsoftGraphAppID is the app ID for Microsoft Graph.
	microsoftGraphAppID = "00000003-0000-0000-c000-000000000000"
)

// Required app role IDs (Application permissions) to be granted via appRoleAssignedTo on Microsoft Graph.
const (
	// graphDirectoryReadAllRoleID grants Directory.Read.All on Microsoft Graph.
	graphDirectoryReadAllRoleID = "7ab1d382-f21e-4acd-a863-ba3e13f7da61"
	// graphUserReadAllRoleID grants User.Read.All (application) on Microsoft Graph.
	graphUserReadAllRoleID = "df021288-bdef-4463-88db-98f22de89214"
	// graphPolicyReadAllRoleID grants Policy.Read.All (application) on Microsoft Graph.
	graphPolicyReadAllRoleID = "246dd0d5-5bd0-4def-940b-0421030a5b68"
)

// Required delegated permission scopes to be granted via oauth2PermissionGrants on Microsoft Graph.
const (
	// microsoftGraphDelegatedScopes are the delegated scopes that require admin consent on Microsoft Graph.
	microsoftGraphDelegatedScopes = "User.Read.All"
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
					stringplanmodifier.UseStateForUnknown(),
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
					stringplanmodifier.UseStateForUnknown(),
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

	if err := grantAdminConsent(ctx, client.GraphClient, *servicePrincipalID); err != nil {
		resp.Diagnostics.AddError("[Service Principal][Create] Failed to grant admin consent", err.Error())
		return
	}

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
		tflog.Info(ctx, fmt.Sprintf("[Service Principal][Read] Service principal for appId %s not found, removing from state", *app.GetAppId()))
		resp.State.RemoveResource(ctx)
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
	tags := plan.Tags
	if len(tags) == 0 {
		tags = state.Tags
	} else {
		tflog.Info(ctx, fmt.Sprintf("[Service Principal] Using provided barebone version: %s", plan.Tags[0]))
	}
	app := models.NewApplication()
	app.SetDisplayName(&displayName)
	app.SetTags(tags)

	_, err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).Patch(ctx, app, nil)
	if err != nil {
		resp.Diagnostics.AddError("Update Service Principal Failed", err.Error())
		return
	}

	if err := grantAdminConsent(ctx, client.GraphClient, state.PrincipalID.ValueString()); err != nil {
		resp.Diagnostics.AddError("[Service Principal][Update] Failed to grant admin consent", err.Error())
		return
	}

	state.ClientID = types.StringValue(state.ClientID.ValueString())
	state.DisplayName = types.StringValue(displayName)
	state.ObjectID = types.StringValue(state.ObjectID.ValueString())
	state.PrincipalID = types.StringValue(state.PrincipalID.ValueString())
	state.SubscriptionID = plan.SubscriptionID
	state.Tags = tags

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
		Client: client.WithTimeout(cam.CAMAPITimeout),
	}
	tflog.Debug(ctx, "[Service Principal] Service Principal resource configured successfully")
}

func (r *servicePrincipal) createServicePrincipal(ctx context.Context, client *msgraph.GraphServiceClient, applicationID, templateVersion *string) (*string, error) {
	// Check if service principal already exists before creating (idempotency)
	filter := fmt.Sprintf("appId eq '%s'", *applicationID)
	existing, err := client.ServicePrincipals().Get(ctx, &serviceprincipals.ServicePrincipalsRequestBuilderGetRequestConfiguration{
		QueryParameters: &serviceprincipals.ServicePrincipalsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	})
	if err == nil && existing != nil && len(existing.GetValue()) > 0 {
		tflog.Info(ctx, fmt.Sprintf("[Service Principal] Service principal for appId %s already exists, reusing existing", *applicationID))
		return existing.GetValue()[0].GetId(), nil
	}

	sp := models.NewServicePrincipal()
	sp.SetAppId(applicationID)
	sp.SetAppRoleAssignmentRequired(toBoolPointer(true))
	sp.SetTags([]string{*templateVersion})

	createdSp, err := client.ServicePrincipals().Post(ctx, sp, nil)
	if err != nil {
		return nil, err
	}

	return createdSp.GetId(), nil
}

// grantAdminConsent grants admin consent for API permissions required by the service principal.
// It mirrors the shell script's grant_admin_consent_00000002_* and grant_admin_consent_00000003_* functions.
// It checks whether all permissions are already in place and skips the entire operation if so,
// avoiding unnecessary Graph API mutations on every Terraform apply when nothing has changed.
func grantAdminConsent(ctx context.Context, client *msgraph.GraphServiceClient, servicePrincipalID string) error {
	tflog.Info(ctx, fmt.Sprintf("[Service Principal] Starting admin consent grant for service principal %s", servicePrincipalID))

	msGraphSPID, err := resolveServicePrincipalID(ctx, client, microsoftGraphAppID)
	if err != nil {
		return fmt.Errorf("resolving Microsoft Graph service principal: %w", err)
	}
	tflog.Info(ctx, fmt.Sprintf("[Service Principal] Resolved Microsoft Graph SP: appId=%s → objectId=%s", microsoftGraphAppID, msGraphSPID))

	alreadyGranted, err := arePermissionsGranted(ctx, client, servicePrincipalID, msGraphSPID)
	if err != nil {
		return fmt.Errorf("checking existing permissions: %w", err)
	}
	if alreadyGranted {
		tflog.Info(ctx, fmt.Sprintf("[Service Principal] Admin consent already fully granted for service principal %s, skipping", servicePrincipalID))
		return nil
	}
	tflog.Info(ctx, fmt.Sprintf("[Service Principal] Permissions not fully granted for service principal %s, proceeding to grant", servicePrincipalID))

	if err := grantOAuth2PermissionGrant(ctx, client, servicePrincipalID, msGraphSPID, microsoftGraphDelegatedScopes); err != nil {
		return fmt.Errorf("granting Microsoft Graph delegated permissions: %w", err)
	}
	if err := grantAppRoleAssignment(ctx, client, servicePrincipalID, msGraphSPID, graphDirectoryReadAllRoleID); err != nil {
		return fmt.Errorf("granting Microsoft Graph Directory.Read.All role: %w", err)
	}
	if err := grantAppRoleAssignment(ctx, client, servicePrincipalID, msGraphSPID, graphUserReadAllRoleID); err != nil {
		return fmt.Errorf("granting Microsoft Graph User.Read.All role: %w", err)
	}
	if err := grantAppRoleAssignment(ctx, client, servicePrincipalID, msGraphSPID, graphPolicyReadAllRoleID); err != nil {
		return fmt.Errorf("granting Microsoft Graph Policy.Read.All role: %w", err)
	}

	tflog.Info(ctx, fmt.Sprintf("[Service Principal] Admin consent granted for service principal %s", servicePrincipalID))
	return nil
}

// arePermissionsGranted returns true only if all required oauth2PermissionGrants and appRoleAssignments
// on Microsoft Graph are already present. This lets Update skip all mutations when the SP has not changed.
func arePermissionsGranted(ctx context.Context, client *msgraph.GraphServiceClient, servicePrincipalID, msGraphSPID string) (bool, error) {
	// Check oauth2PermissionGrants for Microsoft Graph
	filter := fmt.Sprintf("clientId eq '%s' and resourceId eq '%s'", servicePrincipalID, msGraphSPID)
	result, err := client.Oauth2PermissionGrants().Get(ctx, &oauth2permissiongrants.Oauth2PermissionGrantsRequestBuilderGetRequestConfiguration{
		QueryParameters: &oauth2permissiongrants.Oauth2PermissionGrantsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	})
	if err != nil {
		return false, fmt.Errorf("querying oauth2PermissionGrants for Microsoft Graph: %w", err)
	}
	if len(result.GetValue()) == 0 {
		return false, nil
	}

	// Check appRoleAssignments on the client SP (our app) instead of querying
	// appRoleAssignedTo on the resource SP (Microsoft Graph), because the latter
	// does not support $filter on principalId and returns "Links to EntitlementGrant
	// are not supported between specified entities".
	requiredRoles := []string{graphDirectoryReadAllRoleID, graphUserReadAllRoleID, graphPolicyReadAllRoleID}
	roleResult, err := client.ServicePrincipals().ByServicePrincipalId(servicePrincipalID).AppRoleAssignments().Get(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("querying appRoleAssignments for service principal: %w", err)
	}
	assigned := make(map[string]bool)
	for _, a := range roleResult.GetValue() {
		if a.GetAppRoleId() != nil && a.GetResourceId() != nil {
			resID, _ := uuid.Parse(msGraphSPID)
			if *a.GetResourceId() == resID {
				assigned[a.GetAppRoleId().String()] = true
			}
		}
	}
	for _, role := range requiredRoles {
		if !assigned[role] {
			return false, nil
		}
	}

	return true, nil
}

// resolveServicePrincipalID looks up the object ID of a first-party service principal by its appId.
func resolveServicePrincipalID(ctx context.Context, client *msgraph.GraphServiceClient, appID string) (string, error) {
	filter := fmt.Sprintf("appId eq '%s'", appID)
	result, err := client.ServicePrincipals().Get(ctx, &serviceprincipals.ServicePrincipalsRequestBuilderGetRequestConfiguration{
		QueryParameters: &serviceprincipals.ServicePrincipalsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	})
	if err != nil {
		return "", fmt.Errorf("querying service principal for appId %s: %w", appID, err)
	}
	list := result.GetValue()
	if len(list) == 0 {
		return "", fmt.Errorf("service principal for appId %s not found in tenant", appID)
	}
	id := list[0].GetId()
	if id == nil {
		return "", fmt.Errorf("service principal for appId %s has no object ID", appID)
	}
	return *id, nil
}

// grantOAuth2PermissionGrant ensures a delegated permission grant (admin consent) exists for the
// given clientSPID → resourceSPID with the specified scope.
// If a grant already exists it is PATCHed with the desired scope; otherwise a new grant is POSTed.
// This avoids the "revoke and recreate" cycle that causes the Azure Portal to briefly show
// "Unable to determine status" after every Terraform apply.
func grantOAuth2PermissionGrant(ctx context.Context, client *msgraph.GraphServiceClient, clientSPID, resourceSPID, scope string) error {
	filter := fmt.Sprintf("clientId eq '%s' and resourceId eq '%s'", clientSPID, resourceSPID)
	existing, err := client.Oauth2PermissionGrants().Get(ctx, &oauth2permissiongrants.Oauth2PermissionGrantsRequestBuilderGetRequestConfiguration{
		QueryParameters: &oauth2permissiongrants.Oauth2PermissionGrantsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	})
	if err != nil {
		return fmt.Errorf("listing oauth2PermissionGrants for resource %s: %w", resourceSPID, err)
	}

	grants := existing.GetValue()
	if len(grants) > 0 {
		grantID := grants[0].GetId()
		if grantID == nil {
			return fmt.Errorf("existing oauth2PermissionGrant for resource %s has no ID", resourceSPID)
		}
		patch := models.NewOAuth2PermissionGrant()
		patch.SetScope(&scope)
		_, err = client.Oauth2PermissionGrants().ByOAuth2PermissionGrantId(*grantID).Patch(ctx, patch, nil)
		if err != nil {
			return fmt.Errorf("patching oauth2PermissionGrant %s: %w", *grantID, err)
		}
		tflog.Info(ctx, fmt.Sprintf("[Service Principal] oauth2PermissionGrant for resource %s patched scope=%q", resourceSPID, scope))
		return nil
	}

	grant := models.NewOAuth2PermissionGrant()
	grant.SetClientId(&clientSPID)
	grant.SetConsentType(toStringPointer("AllPrincipals"))
	grant.SetResourceId(&resourceSPID)
	grant.SetScope(&scope)
	_, err = client.Oauth2PermissionGrants().Post(ctx, grant, nil)
	if err != nil {
		return fmt.Errorf("posting oauth2PermissionGrant for resource %s: %w", resourceSPID, err)
	}
	tflog.Info(ctx, fmt.Sprintf("[Service Principal] oauth2PermissionGrant for resource %s created scope=%q", resourceSPID, scope))
	return nil
}

// grantAppRoleAssignment assigns an application role to the service principal on a resource SP.
// Errors indicating the assignment already exists are silently ignored.
func grantAppRoleAssignment(ctx context.Context, client *msgraph.GraphServiceClient, principalID, resourceSPID, appRoleIDStr string) error {
	principalUUID, err := uuid.Parse(principalID)
	if err != nil {
		return fmt.Errorf("parsing principal UUID %s: %w", principalID, err)
	}
	resourceUUID, err := uuid.Parse(resourceSPID)
	if err != nil {
		return fmt.Errorf("parsing resource UUID %s: %w", resourceSPID, err)
	}
	appRoleUUID, err := uuid.Parse(appRoleIDStr)
	if err != nil {
		return fmt.Errorf("parsing appRoleId UUID %s: %w", appRoleIDStr, err)
	}

	assignment := models.NewAppRoleAssignment()
	assignment.SetPrincipalId(&principalUUID)
	assignment.SetResourceId(&resourceUUID)
	assignment.SetAppRoleId(&appRoleUUID)

	_, err = client.ServicePrincipals().ByServicePrincipalId(resourceSPID).AppRoleAssignedTo().Post(ctx, assignment, nil)
	if err != nil {
		if strings.Contains(err.Error(), "Permission being assigned already exists on the object") {
			tflog.Info(ctx, fmt.Sprintf("[Service Principal] appRoleAssignment role=%s on resource %s already exists, skipping", appRoleIDStr, resourceSPID))
			return nil
		}
		return err
	}
	tflog.Info(ctx, fmt.Sprintf("[Service Principal] appRoleAssignment role=%s granted on resource %s", appRoleIDStr, resourceSPID))
	return nil
}
