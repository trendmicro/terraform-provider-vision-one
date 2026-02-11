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

const (
	defaultVisionOneRegionCode = "us"
)

type federatedIdentity struct {
	client *api.CamClient
}

type camFederatedIdentityCredentialModel struct {
	CamDeployedRegion     types.String `tfsdk:"cam_deployed_region"`
	VisionOneRegionCode   types.String `tfsdk:"vision_one_region_code"`
	IssuerURL             types.String `tfsdk:"issuer_url"`
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
			"vision_one_region_code": schema.StringAttribute{
				MarkdownDescription: "Vision One region code for the federated identity credential. If not specified, the region code will be automatically extracted from the provider's `regional_fqdn` configuration. The supported region codes are `au`, `sg`, `us`, `in`, `jp`, `eu`, `mea`, `ca`, `uk`. Defaults to `us` if no region can be determined.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"issuer_url": schema.StringAttribute{
				MarkdownDescription: "Issuer URL for the federated identity credential. If not specified, the issuer URL will be automatically derived from the `vision_one_region_code`. This allows advanced users to specify custom issuer URLs for testing or specialized deployments.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cam_deployed_region": schema.StringAttribute{
				MarkdownDescription: "**Deprecated**: Use `vision_one_region_code` instead. This field is kept for backwards compatibility and will be removed in a future version.",
				Optional:            true,
				DeprecationMessage:  "Use vision_one_region_code instead. This field will be removed in a future version.",
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

	// Get regional FQDN from provider configuration
	regionalFQDN := ""
	if r.client != nil && r.client.Client != nil {
		regionalFQDN = r.client.Client.HostURL
	}

	// Resolve Vision One region code with fallback logic
	visionOneRegionCode := getVisionOneRegionCode(plan.VisionOneRegionCode.ValueString(), plan.CamDeployedRegion.ValueString(), regionalFQDN)

	// Validate region code
	if !isValidVisionOneRegionCode(visionOneRegionCode) {
		resp.Diagnostics.AddError(
			"[Federated Identity][Create] Invalid Vision One Region Code",
			fmt.Sprintf("Invalid region code '%s'. Supported region codes are: au, sg, us, in, jp, eu, mea, ca, uk", visionOneRegionCode),
		)
		return
	}

	// Determine issuer URL - use explicit value if provided, otherwise derive from region
	issuerURL := plan.IssuerURL.ValueString()
	if issuerURL == "" {
		issuerURL = getIssuerURL(visionOneRegionCode)
	}

	fed := models.NewFederatedIdentityCredential()
	fed.SetName(toStringPointer(getFederatedIdentityName(plan.FederatedIdentityName.ValueString())))
	fed.SetDescription(toStringPointer(config.FEDERATED_CREDENTIALS_DESCRIPTION))
	fed.SetAudiences([]string{"api://AzureADTokenExchange"})
	fed.SetIssuer(toStringPointer(issuerURL))
	fed.SetSubject(toStringPointer(`urn:visionone:identity:` + visionOneRegionCode + `:` + v1BusinessID + `:account/` + v1BusinessID))

	_, err = client.GraphClient.Applications().ByApplicationId(*objectId).FederatedIdentityCredentials().Post(ctx, fed, nil)
	if err != nil {
		resp.Diagnostics.AddError("[Federated Identity][Create] Failed to create federated identity credential", err.Error())
		return
	}

	plan.ClientID = types.StringValue(plan.ClientID.ValueString())
	plan.FederatedIdentityName = types.StringValue(getFederatedIdentityName(plan.FederatedIdentityName.ValueString()))
	plan.ObjectID = types.StringValue(plan.ObjectID.ValueString())
	plan.SubscriptionID = types.StringValue(subscriptionID)
	plan.VisionOneRegionCode = types.StringValue(visionOneRegionCode)
	plan.IssuerURL = types.StringValue(issuerURL)

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

	// Preserve the region code from state (don't overwrite with resolved value)
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
		resp.Diagnostics.AddError("[Federated Identity][Update] Invalid Business ID", "V1 Business ID cannot be empty")
		return
	}

	// Get regional FQDN from provider configuration
	regionalFQDN := ""
	if r.client != nil && r.client.Client != nil {
		regionalFQDN = r.client.Client.HostURL
	}

	// Resolve Vision One region code with fallback logic
	visionOneRegionCode := getVisionOneRegionCode(plan.VisionOneRegionCode.ValueString(), plan.CamDeployedRegion.ValueString(), regionalFQDN)

	// Validate region code
	if !isValidVisionOneRegionCode(visionOneRegionCode) {
		resp.Diagnostics.AddError(
			"[Federated Identity][Update] Invalid Vision One Region Code",
			fmt.Sprintf("Invalid region code '%s'. Supported region codes are: au, sg, us, in, jp, eu, mea, ca, uk", visionOneRegionCode),
		)
		return
	}

	// Determine issuer URL - use explicit value if provided, otherwise derive from region
	issuerURL := plan.IssuerURL.ValueString()
	if issuerURL == "" {
		issuerURL = getIssuerURL(visionOneRegionCode)
	}

	fed := models.NewFederatedIdentityCredential()
	fed.SetName(toStringPointer(getFederatedIdentityName(plan.FederatedIdentityName.ValueString())))
	fed.SetDescription(toStringPointer(config.FEDERATED_CREDENTIALS_DESCRIPTION))
	fed.SetAudiences([]string{"api://AzureADTokenExchange"})
	fed.SetIssuer(toStringPointer(issuerURL))
	fed.SetSubject(toStringPointer(`urn:visionone:identity:` + visionOneRegionCode + `:` + v1BusinessID + `:account/` + v1BusinessID))
	_, err = client.GraphClient.Applications().ByApplicationId(state.ObjectID.ValueString()).FederatedIdentityCredentials().Post(ctx, fed, nil)
	if err != nil {
		resp.Diagnostics.AddError("[Federated Identity][Update] Update Federated Identity Credential Failed", err.Error())
		return
	}

	state.VisionOneRegionCode = types.StringValue(visionOneRegionCode)
	state.IssuerURL = types.StringValue(issuerURL)
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

// extractRegionFromFQDN extracts the region code from Vision One regional FQDN
// Maps FQDNs to region codes based on the official Vision One regional domains
func extractRegionFromFQDN(fqdn string) string {
	// Remove protocol if present
	fqdn = strings.TrimPrefix(fqdn, "https://")
	fqdn = strings.TrimPrefix(fqdn, "http://")

	// Regional FQDN mapping
	fqdnToRegion := map[string]string{
		"api.au.xdr.trendmicro.com":  "au",                       // Australia
		"api.ca.xdr.trendmicro.com":  "ca",                       // Canada
		"api.eu.xdr.trendmicro.com":  "eu",                       // Germany
		"api.in.xdr.trendmicro.com":  "in",                       // India
		"api.xdr.trendmicro.co.jp":   "jp",                       // Japan
		"api.sg.xdr.trendmicro.com":  "sg",                       // Singapore
		"api.mea.xdr.trendmicro.com": "mea",                      // United Arab Emirates
		"api.uk.xdr.trendmicro.com":  "uk",                       // United Kingdom
		"api.xdr.trendmicro.com":     defaultVisionOneRegionCode, // United States
	}

	if region, ok := fqdnToRegion[fqdn]; ok {
		return region
	}

	// Return empty string if not found (will trigger fallback logic)
	return ""
}

// getVisionOneRegionCode resolves the Vision One region code with fallback logic
// Priority: vision_one_region_code > regional_fqdn from provider > cam_deployed_region (deprecated) > default "us"
func getVisionOneRegionCode(visionOneRegion, camDeployedRegion, regionalFQDN string) string {
	// Prefer vision_one_region_code if provided
	if visionOneRegion != "" {
		return visionOneRegion
	}

	// Try to extract region from provider's regional_fqdn
	if regionalFQDN != "" {
		if region := extractRegionFromFQDN(regionalFQDN); region != "" {
			return region
		}
	}

	// Fall back to cam_deployed_region for backwards compatibility
	if camDeployedRegion != "" {
		return camDeployedRegion
	}

	// Default to "us"
	return defaultVisionOneRegionCode
}

// isValidVisionOneRegionCode validates the Vision One region code
func isValidVisionOneRegionCode(region string) bool {
	validRegions := []string{"au", "sg", "us", "in", "jp", "eu", "mea", "ca", "uk"}
	for _, valid := range validRegions {
		if region == valid {
			return true
		}
	}
	return false
}
