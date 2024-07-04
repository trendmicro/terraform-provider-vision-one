package provider

import (
	"context"
	"os"
	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/container_security/resources"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	TF_KEY_API_KEY        = "api_key"
	TF_KEY_REG_FQDN       = "regional_fqdn"
	ENV_VAR_NAME_API_KEY  = "VISIONONE_API_KEY"
	ENV_VAR_NAME_REG_FQDN = "VISIONONE_REGIONAL_FQDN"
)

const (
	UnkonwnAPIKeyErrDetail  = "The provider cannot create the Trend Vision One API client as there is an unknown configuration value for the Vision One API Key. You could obtain a valid key from Vision One Console or API. Either target apply the source of the value first, set the value statically in the configuration, or use the " + ENV_VAR_NAME_API_KEY + " environment variable."
	UnkonwnRegFQDNErrDetail = "The provider cannot create the Trend Vision One API client as there is an unknown configuration value for the Vision One Regional FQDN. Either target apply the source of the value first, set the value statically in the configuration, or use the " + ENV_VAR_NAME_REG_FQDN + " environment variable."
)

// Ensure TrendMicroProvider satisfies various provider interfaces.
var _ provider.Provider = &TrendMicroProvider{}
var _ provider.ProviderWithFunctions = &TrendMicroProvider{}

// TrendMicroProvider defines the provider implementation.
type TrendMicroProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// TrendMicroProviderModel describes the provider data model.
type TrendMicroProviderModel struct {
	ApiKey  types.String `tfsdk:"api_key"`
	RegFQDN types.String `tfsdk:"regional_fqdn"`
}

func (p *TrendMicroProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "visionone"
	resp.Version = p.version
}

func (p *TrendMicroProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			TF_KEY_REG_FQDN: schema.StringAttribute{
				MarkdownDescription: "Trend Vision One provides a server in each region where the service endpoint is hosted. You must specify the correct domain name for your region. Reference: https://automation.trendmicro.com/xdr/Guides/Regional-Domains",
				Optional:            true,
			},
			TF_KEY_API_KEY: schema.StringAttribute{
				MarkdownDescription: "API Key from Vision One Console",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *TrendMicroProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data TrendMicroProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.ApiKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(TF_KEY_API_KEY),
			"Unknown VisionOne API Key",
			UnkonwnAPIKeyErrDetail,
		)
	}

	if data.RegFQDN.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(TF_KEY_REG_FQDN),
			"Unknown VisionOne Regional FQDN",
			UnkonwnRegFQDNErrDetail,
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	var apiKey string
	var host string

	if !data.ApiKey.IsNull() {
		apiKey = data.ApiKey.ValueString()
	} else {
		apiKey = os.Getenv(ENV_VAR_NAME_API_KEY)
	}

	if !data.RegFQDN.IsNull() {
		host = data.RegFQDN.ValueString()
	} else {
		host = os.Getenv(ENV_VAR_NAME_REG_FQDN)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root(TF_KEY_API_KEY),
			"Missing Vision One API Key",
			UnkonwnAPIKeyErrDetail,
		)
	}

	if host == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root(TF_KEY_REG_FQDN),
			"Unknown Vision One Regional FQDN",
			UnkonwnRegFQDNErrDetail,
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Example client configuration for data sources and resources

	ctx = tflog.SetField(ctx, TF_KEY_REG_FQDN, host)
	ctx = tflog.SetField(ctx, TF_KEY_API_KEY, apiKey)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, TF_KEY_API_KEY, apiKey)

	tflog.Debug(ctx, "Creating Trend Vision One API client")

	client, err := trendmicro.NewClient(&host, &apiKey, p.version)
	if err != nil {
		tflog.Debug(ctx, err.Error())
		resp.Diagnostics.AddError(
			"Unable to Create Trend Vision One API client",
			"An unexpected error occurred when creating the Trend Vision One API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Trend Vision One client error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured Trend Vision One client", map[string]any{"success": true})
}

func (p *TrendMicroProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewClusterResource,
		resources.NewRulesetResource,
		resources.NewPolicyResource,
	}
}

func (p *TrendMicroProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// NewVisionOneOneDataSource,
	}
}

func (p *TrendMicroProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		// NewExampleFunction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TrendMicroProvider{
			version: version,
		}
	}
}
