package datasources

import (
	"context"
	"fmt"
	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure interface compliance at compile time
var (
	_ datasource.DataSource                     = &CRMAccountDataSource{}
	_ datasource.DataSourceWithConfigure        = &CRMAccountDataSource{}
	_ datasource.DataSourceWithConfigValidators = &CRMAccountDataSource{}
)

type CRMAccountDataSource struct {
	client *api.CrmClient
}

type CRMAccountDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	AwsAccountID        types.String `tfsdk:"aws_account_id"`
	AzureSubscriptionID types.String `tfsdk:"azure_subscription_id"`
	GcpProjectID        types.String `tfsdk:"gcp_project_id"`
	OciCompartmentID    types.String `tfsdk:"oci_compartment_id"`
	AlibabaAccountID    types.String `tfsdk:"alibaba_account_id"`
}

func NewCRMAccountDataSource() datasource.DataSource {
	return &CRMAccountDataSource{}
}

func (d *CRMAccountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_account"
}

func (d *CRMAccountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Provider Data",
			"Expected *trendmicro.Client, got something else.",
		)
		return
	}

	d.client = api.NewCrmClient(client.HostURL, client.BearerToken, client.ProviderVersion)
}

func (d *CRMAccountDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a Cloud Risk Management account ID using a cloud provider account identifier.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The Cloud Risk Management account ID (UUID).",
				Computed:    true,
			},
			"aws_account_id": schema.StringAttribute{
				Description: "The AWS account ID.",
				Optional:    true,
			},
			"azure_subscription_id": schema.StringAttribute{
				Description: "The Azure subscription ID.",
				Optional:    true,
			},
			"gcp_project_id": schema.StringAttribute{
				Description: "The GCP project ID.",
				Optional:    true,
			},
			"oci_compartment_id": schema.StringAttribute{
				Description: "The OCI compartment ID.",
				Optional:    true,
			},
			"alibaba_account_id": schema.StringAttribute{
				Description: "The Alibaba Cloud account ID.",
				Optional:    true,
			},
		},
	}
}

func (d *CRMAccountDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("aws_account_id"),
			path.MatchRoot("azure_subscription_id"),
			path.MatchRoot("gcp_project_id"),
			path.MatchRoot("oci_compartment_id"),
			path.MatchRoot("alibaba_account_id"),
		),
	}
}

func (d *CRMAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CRMAccountDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Looking up CRM account", map[string]any{
		"aws_account_id":        data.AwsAccountID.ValueString(),
		"azure_subscription_id": data.AzureSubscriptionID.ValueString(),
		"gcp_project_id":        data.GcpProjectID.ValueString(),
		"oci_compartment_id":    data.OciCompartmentID.ValueString(),
		"alibaba_account_id":    data.AlibabaAccountID.ValueString(),
	})

	filter := api.CloudProviderFilter{
		AwsAccountID:        data.AwsAccountID.ValueString(),
		AzureSubscriptionID: data.AzureSubscriptionID.ValueString(),
		GcpProjectID:        data.GcpProjectID.ValueString(),
		AlibabaAccountID:    data.AlibabaAccountID.ValueString(),
		OciCompartmentID:    data.OciCompartmentID.ValueString(),
	}

	var response *cloud_risk_management_dto.ListAccountsResponse
	var err error

	// Retry logic for eventual consistency:
	// When a cloud account is onboarded via Cloud Account Management (CAM),
	// the account record in Cloud Risk Management (CRM) is created asynchronously.
	// This may result in a brief delay before the account is queryable.
	// We retry with exponential backoff to handle this race condition.
	maxRetries := 3
	baseDelay := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		response, err = d.client.ListAccounts(&filter)
		if err != nil {
			tflog.Error(ctx, "Failed to list CRM accounts", map[string]any{
				"error": err.Error(),
			})
			resp.Diagnostics.AddError(
				"Error Reading CRM Account",
				fmt.Sprintf("Unable to read CRM account: %s", err),
			)
			return
		}

		if len(response.Items) > 0 {
			break
		}

		if attempt < maxRetries {
			waitTime := baseDelay * time.Duration(1<<attempt)
			tflog.Debug(ctx, "Account not found in CRM yet, retrying", map[string]any{
				"attempt":     attempt + 1,
				"max_retries": maxRetries,
				"wait_secs":   waitTime.Seconds(),
			})
			time.Sleep(waitTime)
		}
	}

	if len(response.Items) == 0 {
		resp.Diagnostics.AddError(
			"Account Not Found",
			"No Cloud Risk Management account found matching the provided cloud provider ID.\n\n"+
				"Possible causes:\n"+
				"  - The account was recently onboarded and is still being set up\n"+
				"  - The account was never onboarded via Cloud Account Management\n"+
				"  - There was an issue during account provisioning\n\n"+
				"Ensure the account is onboarded via Cloud Account Management before using this data source.",
		)
		return
	}

	if len(response.Items) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Accounts Found",
			fmt.Sprintf(
				"Expected exactly one account but found %d. "+
					"This indicates a data inconsistency. Please contact support.",
				len(response.Items),
			),
		)
		return
	}

	tflog.Debug(ctx, "Found CRM account", map[string]any{
		"id": response.Items[0].ID,
	})

	data.ID = types.StringValue(response.Items[0].ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
