package data_sources

import (
	"context"
	"fmt"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/data-sources/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/data-sources/config"

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

	// GCP-specific fields
	OidcProviderID         types.String              `tfsdk:"oidc_provider_id"`          // GCP Workload Identity Provider ID
	ProjectID              types.String              `tfsdk:"project_id"`                // GCP project ID
	ProjectNumber          types.String              `tfsdk:"project_number"`            // GCP project number
	ProjectName            types.String              `tfsdk:"project_name"`              // GCP project name
	ServiceAccountEmail    types.String              `tfsdk:"service_account_email"`     // GCP Service Account Email
	ServiceAccountID       types.String              `tfsdk:"service_account_id"`        // GCP Service Account Unique ID
	WorkloadIdentityPoolID types.String              `tfsdk:"workload_identity_pool_id"` // GCP Workload Identity Pool ID
	Organization           *OrganizationDetailsModel `tfsdk:"organization"`              // GCP Organization details

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

type CAMGCPProjectDataSourceModel struct {
	CloudAccounts []CAMCloudAccountModel `tfsdk:"cloud_accounts"`
	ProjectIds    []types.String         `tfsdk:"project_ids"`
	State         types.String           `tfsdk:"state"`
	Top           types.Int64            `tfsdk:"top"`
}

type OrganizationDetailsModel struct {
	ID               types.String   `tfsdk:"id"`
	DisplayName      types.String   `tfsdk:"display_name"`
	ExcludedProjects []types.String `tfsdk:"excluded_projects"`
}

type CAMCloudAccountsDataSource struct {
	client *api.CamClient
}

// Metadata sets the data source type name for CAM Cloud Accounts
func (d *CAMCloudAccountsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.DATA_SOURCE_TYPE_CAM_CONNECT_GCP_PROJECTS
}

// Schema defines the data source schema for CAM Cloud Accounts
func (d *CAMCloudAccountsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for retrieving GCP projects from Trend Micro Vision One Cloud Account Management.",
		Attributes: map[string]schema.Attribute{
			"cloud_accounts": schema.ListNestedAttribute{
				MarkdownDescription: "List of GCP projects managed by Trend Micro Vision One Cloud Account Management.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: d.getCloudAccountAttributes(),
				},
			},
			"project_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "List of GCP project IDs to filter the cloud accounts.",
			},
			"state": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Current state of the cloud account. Valid values: managed, outdated, failed.",
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
		// GCP-specific fields
		"oidc_provider_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "GCP Workload Identity Provider ID.",
		},
		"project_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "GCP project ID.",
		},
		"project_number": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "GCP project number.",
		},
		"project_name": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "GCP project name.",
		},
		"service_account_email": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "GCP service account email for the cloud account.",
		},
		"service_account_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "GCP Service Account unique ID.",
		},
		"workload_identity_pool_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "GCP Workload Identity Pool ID.",
		},
		"organization": schema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "GCP Organization details.",
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "Organization ID.",
				},
				"display_name": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "Organization display name.",
				},
				"excluded_projects": schema.ListAttribute{
					ElementType:         types.StringType,
					Computed:            true,
					MarkdownDescription: "List of excluded project IDs.",
				},
			},
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
	var data CAMGCPProjectDataSourceModel

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectIDs := cam.ConvertTypesStringSliceToStringSlice(data.ProjectIds)

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

	response, err := d.client.ListGCPProjects(projectIDs, top, state)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Read CAM Cloud Accounts",
			fmt.Sprintf("Unable to retrieve GCP project data from CAM API. Project IDs: %v. Error: %s", projectIDs, err.Error()),
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
	tflog.Debug(ctx, "[CAM Cloud Accounts] CAM Cloud Accounts GCP data source configured successfully")
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
			ID:          cam.GetStringValue(account.ProjectNumber),
			Name:        cam.GetStringValue(account.Name),
			Description: cam.GetStringValue(account.Description),
			State:       cam.GetStringValue(account.State),

			// GCP-specific fields
			OidcProviderID:         cam.GetStringValue(account.OidcProviderID),
			ProjectID:              cam.GetStringValue(account.ProjectID),
			ProjectName:            cam.GetStringValue(account.ProjectName),
			ProjectNumber:          cam.GetStringValue(account.ProjectNumber),
			ServiceAccountEmail:    cam.GetStringValue(account.ServiceAccountEmail),
			ServiceAccountID:       cam.GetStringValue(account.ServiceAccountID),
			WorkloadIdentityPoolID: cam.GetStringValue(account.WorkloadIdentityPoolID),
			CamDeployedRegion:      cam.GetStringValue(account.CamDeployedRegion),
			Organization:           convertOrganizationDetails(account.Organization),

			// Trend Micro security features
			IsCAMCloudASRMEnabled:     cam.GetBoolPointerValue(account.IsCAMCloudASRMEnabled),
			IsCloudASRMEditable:       cam.GetBoolPointerValue(account.IsCloudASRMEditable),
			IsCloudASRMEnabled:        cam.GetBoolPointerValue(account.IsCloudASRMEnabled),
			ConnectedSecurityServices: cam.ConvertConnectedSecurityServices(account.ConnectedSecurityServices),
			Features:                  cam.ConvertFeatures(account.Features),

			// Metadata and tracking
			CloudAssetCount:    cam.GetInt64Value(account.CloudAssetCount),
			Sources:            cam.ConvertStringSlice(account.Sources),
			CreatedDateTime:    cam.GetStringValue(account.CreatedTime),
			UpdatedDateTime:    cam.GetStringValue(account.UpdatedDateTime),
			LastSyncedDateTime: cam.GetStringValue(account.LastSyncedDateTime),
		}
		accounts = append(accounts, model)
	}

	return accounts
}

// convertOrganizationDetails converts API organization details to Terraform model
func convertOrganizationDetails(org *api.OrganizationDetailsResponse) *OrganizationDetailsModel {
	if org == nil {
		return nil
	}

	return &OrganizationDetailsModel{
		ID:               cam.GetStringValue(org.ID),
		DisplayName:      cam.GetStringValue(org.DisplayName),
		ExcludedProjects: cam.ConvertStringSlice(org.ExcludedProjects),
	}
}
