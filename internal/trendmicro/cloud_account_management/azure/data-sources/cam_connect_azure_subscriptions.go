package data_sources

import (
	"context"
	"fmt"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/data-sources/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/data-sources/config"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &CAMCloudAccountsDataSource{}
	_ datasource.DataSourceWithConfigure = &CAMCloudAccountsDataSource{}
)

func NewCAMCloudAccountsDataSource() datasource.DataSource {
	return &CAMCloudAccountsDataSource{}
}

// CAMCloudAccountModel represents a cloud account across different cloud providers.
// Fields are organized by cloud provider usage and common functionality.
type CAMCloudAccountModel struct {
	// Common fields used across all providers
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	State             types.String `tfsdk:"state"`
	CamDeployedRegion types.String `tfsdk:"cam_deployed_region"`

	// Azure-specific fields
	TenantID      types.String `tfsdk:"tenant_id"`
	ApplicationID types.String `tfsdk:"application_id"`

	// Deployment and infrastructure (common)
	IsTerraformDeployed types.Bool `tfsdk:"is_terraform_deployed"`

	// Trend Micro security features and services (common)
	IsCAMCloudASRMEnabled     types.Bool                          `tfsdk:"is_cam_cloud_asrm_enabled"`
	IsCloudASRMEditable       types.Bool                          `tfsdk:"is_cloud_asrm_editable"`
	IsCloudASRMEnabled        types.Bool                          `tfsdk:"is_cloud_asrm_enabled"`
	ConnectedSecurityServices []cam.ConnectedSecurityServiceModel `tfsdk:"connected_security_services"`
	Features                  []cam.FeatureModel                  `tfsdk:"features"`

	// Metadata and tracking (common)
	CloudAssetCount    types.Int64    `tfsdk:"cloud_asset_count"`
	Sources            []types.String `tfsdk:"sources"`
	CreatedDateTime    types.String   `tfsdk:"created_date_time"`
	UpdatedDateTime    types.String   `tfsdk:"updated_date_time"`
	LastSyncedDateTime types.String   `tfsdk:"last_synced_date_time"`
}

type CAMCloudAccountsDataSource struct {
	client *api.CamClient
}

type CAMAzureSubscriptionDataSourceModel struct {
	CloudAccounts   []CAMCloudAccountModel `tfsdk:"cloud_accounts"`
	SubscriptionIds []types.String         `tfsdk:"subscription_ids"`
	State           types.String           `tfsdk:"state"`
	Top             types.Int64            `tfsdk:"top"`
}

// Metadata sets the data source type name for CAM Cloud Accounts
func (d *CAMCloudAccountsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.DATA_SOURCE_TYPE_CAM_CONNECT_AZURE_SUBSCRIPTIONS
}

// Schema defines the data source schema for CAM Cloud Accounts
func (d *CAMCloudAccountsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for retrieving cloud accounts from Trend Micro Vision One Cloud Account Management.",
		Attributes: map[string]schema.Attribute{
			"cloud_accounts": schema.ListNestedAttribute{
				MarkdownDescription: "List of cloud accounts managed by Trend Micro Vision One Cloud Account Management.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: d.getCloudAccountAttributes(),
				},
			},
			"subscription_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "List of Azure subscription IDs to filter the cloud accounts.",
			},
			"state": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Current state of the cloud account.",
			},
			"top": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of cloud accounts to return. Valid values: 25, 50, 100, 500, 1000, 5000.",
			},
		},
	}
}

// getCloudAccountAttributes returns the schema attributes for a cloud account
func (d *CAMCloudAccountsDataSource) getCloudAccountAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		// Common fields used across all providers
		"id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Unique identifier for the cloud account.",
		},
		"name": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "The name of the cloud account.",
		},
		"description": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Description of the cloud account.",
		},
		"state": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Current state of the cloud account.",
		},
		"cam_deployed_region": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "The region where CAM is deployed.",
		},
		// Azure-specific fields
		"tenant_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Azure tenant ID.",
		},
		"application_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Azure application ID for the cloud account.",
		},
		// Deployment and infrastructure (common)
		"is_terraform_deployed": schema.BoolAttribute{
			Optional:            true,
			MarkdownDescription: "Whether the account was deployed via Terraform.",
		},
		// Trend Micro security features and services (common)
		"is_cam_cloud_asrm_enabled": schema.BoolAttribute{
			Optional:            true,
			MarkdownDescription: "Whether CAM Cloud ASRM is enabled.",
		},
		"is_cloud_asrm_editable": schema.BoolAttribute{
			Optional:            true,
			MarkdownDescription: "Whether Cloud ASRM is editable.",
		},
		"is_cloud_asrm_enabled": schema.BoolAttribute{
			Optional:            true,
			MarkdownDescription: "Whether Cloud ASRM is enabled.",
		},
		"connected_security_services": schema.ListNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Connected security services.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: d.getConnectedSecurityServicesAttributes(),
			},
		},
		"features": schema.ListNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Features enabled for the cloud account.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: d.getFeaturesAttributes(),
			},
		},
		// Metadata and tracking (common)
		"cloud_asset_count": schema.Int64Attribute{
			Optional:            true,
			MarkdownDescription: "Number of cloud assets in the account.",
		},
		"sources": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			MarkdownDescription: "Sources for the cloud account.",
		},
		"created_date_time": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Date and time when the account was created.",
		},
		"updated_date_time": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Date and time when the account was last updated.",
		},
		"last_synced_date_time": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Date and time of last synchronization.",
		},
	}
}

// getConnectedSecurityServicesAttributes returns schema attributes for connected security services
func (d *CAMCloudAccountsDataSource) getConnectedSecurityServicesAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"instance_ids": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "List of instance IDs.",
		},
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Name of the security service.",
		},
	}
}

// getFeaturesAttributes returns schema attributes for features
func (d *CAMCloudAccountsDataSource) getFeaturesAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Feature ID.",
		},
		"regions": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Regions where the feature is enabled.",
		},
		"template_version": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Template version for the feature.",
		},
	}
}

// Read retrieves cloud accounts from the CAM API and populates the data source state
func (d *CAMCloudAccountsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CAMAzureSubscriptionDataSourceModel

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionIDs := cam.ConvertTypesStringSliceToStringSlice(data.SubscriptionIds)

	var top int64
	if data.Top.ValueInt64() > 0 {
		top = data.Top.ValueInt64()
	} else {
		top = 100
	}

	var state string
	if !data.State.IsNull() && !data.State.IsUnknown() {
		state = data.State.ValueString()
	} else {
		state = ""
	}

	response, err := d.client.ListAzureSubscriptions(subscriptionIDs, top, state)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Read CAM Cloud Accounts",
			fmt.Sprintf("Unable to retrieve Azure subscription data from CAM API. Subscription IDs: %v. Error: %s", subscriptionIDs, err.Error()),
		)
		return
	}
	if response != nil {
		if len(response.CloudAccounts) == 0 {
			tflog.Warn(ctx, "[CAM Cloud Accounts] No cloud accounts found")
			data.CloudAccounts = make([]CAMCloudAccountModel, 0)
		} else {
			data.CloudAccounts = convertToCAMCloudAccountModel(response)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "[CAM Cloud Accounts] Failed to set state", map[string]interface{}{
			"errors": resp.Diagnostics.Errors(),
		})
	} else {
		tflog.Debug(ctx, "[CAM Cloud Accounts] Read operation completed successfully")
	}
}

// Configure initializes the CAM client for the data source
func (d *CAMCloudAccountsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Provider Data Type",
			"Expected *trendmicro.Client, but received a different type.",
		)
		return
	}

	d.client = &api.CamClient{
		Client: client,
	}
	tflog.Debug(ctx, "[CAM Cloud Accounts] CAM Cloud Accounts data source configured successfully")
}

// convertToCAMCloudAccountModel transforms API response into Terraform data source model
func convertToCAMCloudAccountModel(response *api.CAMCloudAccountsResponse) []CAMCloudAccountModel {
	if response == nil || len(response.CloudAccounts) == 0 {
		return []CAMCloudAccountModel{}
	}

	accounts := make([]CAMCloudAccountModel, 0, len(response.CloudAccounts))

	// Convert API response to model format
	for i := range response.CloudAccounts {
		account := &response.CloudAccounts[i]
		model := CAMCloudAccountModel{
			// Common fields
			ID:          cam.GetStringValue(account.ID),
			Name:        cam.GetStringValue(account.Name),
			Description: cam.GetStringValue(account.Description),
			State:       cam.GetStringValue(account.State),

			// Azure-specific fields
			TenantID:      cam.GetStringValue(account.TenantID),
			ApplicationID: cam.GetStringValue(account.ApplicationID),

			// Deployment and infrastructure
			IsTerraformDeployed: cam.GetBoolValue(account.IsTerraformDeployed),
			CamDeployedRegion:   cam.GetStringValue(account.CamDeployedRegion),

			// Trend Micro security features
			IsCAMCloudASRMEnabled:     cam.GetBoolValue(account.IsCAMCloudASRMEnabled),
			IsCloudASRMEditable:       cam.GetBoolValue(account.IsCloudASRMEditable),
			IsCloudASRMEnabled:        cam.GetBoolValue(account.IsCloudASRMEnabled),
			ConnectedSecurityServices: cam.ConvertConnectedSecurityServices(account.ConnectedSecurityServices),
			Features:                  cam.ConvertFeatures(account.Features),

			// Metadata and tracking
			CloudAssetCount:    cam.GetInt64Value(account.CloudAssetCount),
			Sources:            cam.ConvertStringSlice(account.Sources),
			CreatedDateTime:    cam.GetStringValue(account.CreatedDateTime),
			UpdatedDateTime:    cam.GetStringValue(account.UpdatedDateTime),
			LastSyncedDateTime: cam.GetStringValue(account.LastSyncedDateTime),
		}
		accounts = append(accounts, model)
	}

	return accounts
}
