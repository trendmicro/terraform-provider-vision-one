package resources

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource              = &CAMConnectorResource{}
	_ resource.ResourceWithConfigure = &CAMConnectorResource{}
)

// SecurityServiceModel represents a connected security service in Terraform state
type SecurityServiceModel struct {
	Name        types.String `tfsdk:"name"`
	InstanceIds types.List   `tfsdk:"instance_ids"`
}

func NewCAMConnectorResource() resource.Resource {
	return &CAMConnectorResource{}
}

type CAMConnectorResource struct {
	client *api.CamClient
}

// CAMConnectorResourceModel describes the resource data model for GCP connector.
type CAMConnectorResourceModel struct {
	// Required fields
	Name              types.String `tfsdk:"name"`
	ProjectNumber     types.String `tfsdk:"project_number"`
	ServiceAccountID  types.String `tfsdk:"service_account_id"`
	ServiceAccountKey types.String `tfsdk:"service_account_key"`

	// Optional fields
	CamDeployedRegion         types.String              `tfsdk:"cam_deployed_region"`
	ConnectedSecurityServices types.List                `tfsdk:"connected_security_services"`
	Description               types.String              `tfsdk:"description"`
	IsCAMCloudASRMEnabled     types.Bool                `tfsdk:"is_cam_cloud_asrm_enabled"`
	Folder                    *FolderDetailsModel       `tfsdk:"folder"`
	Organization              *OrganizationDetailsModel `tfsdk:"organization"`

	// Computed fields
	CreatedDateTime     types.String `tfsdk:"created_date_time"`
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	ProjectName         types.String `tfsdk:"project_name"`
	ServiceAccountEmail types.String `tfsdk:"service_account_email"`
	State               types.String `tfsdk:"state"`
	UpdatedDateTime     types.String `tfsdk:"updated_date_time"`
}

// OrganizationDetailsModel represents the GCP organization details in Terraform state.
type OrganizationDetailsModel struct {
	ID               types.String `tfsdk:"id"`
	DisplayName      types.String `tfsdk:"display_name"`
	ExcludedProjects types.List   `tfsdk:"excluded_projects"`
}

// FolderDetailsModel represents the GCP folder details in Terraform state.
type FolderDetailsModel struct {
	ID               types.String `tfsdk:"id"`
	DisplayName      types.String `tfsdk:"display_name"`
	ExcludedProjects types.List   `tfsdk:"excluded_projects"`
}

func (r *CAMConnectorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_CONNECTOR_GCP
}

func (r *CAMConnectorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GCP connector for Trend Micro Vision One CAM",
		Attributes: map[string]schema.Attribute{
			"cam_deployed_region": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Region where CAM is deployed for this connector",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"connected_security_services": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of connected security services for the connector",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"instance_ids": schema.ListAttribute{
							ElementType:         types.StringType,
							Required:            true,
							MarkdownDescription: "List of instance IDs for the security service",
						},
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Name of the security service",
						},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"created_date_time": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the connector was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the connector",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"folder": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "GCP folder details for the connector",
				Attributes: map[string]schema.Attribute{
					"display_name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Display name of the folder",
					},
					"excluded_projects": schema.ListAttribute{
						ElementType:         types.StringType,
						Optional:            true,
						MarkdownDescription: "List of project numbers to exclude from the folder",
					},
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "GCP folder ID",
					},
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the connector (same as project_number)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"is_cam_cloud_asrm_enabled": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether Trend Vision One Cloud CREM is enabled for the connector",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the connector",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "GCP organization details for the connector",
				Attributes: map[string]schema.Attribute{
					"display_name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Display name of the organization",
					},
					"excluded_projects": schema.ListAttribute{
						ElementType:         types.StringType,
						Optional:            true,
						MarkdownDescription: "List of project numbers to exclude from the organization",
					},
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "GCP organization ID",
					},
				},
			},
			"project_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "GCP project ID",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "GCP project name",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_number": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "GCP project number for the connector",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_email": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "GCP service account email",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_account_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "GCP service account unique ID used to connect to the GCP project",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_key": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "GCP service account key (JSON credentials) used to authenticate with the GCP project. Must be provided as a base64-encoded string.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current state of the connector",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_date_time": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the connector was last updated",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *CAMConnectorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			"Expected *trendmicro.Client, but received a different type.",
		)
		return
	}

	r.client = &api.CamClient{
		Client: client,
	}
	tflog.Debug(ctx, "[CAM Connector GCP] CAM Connector resource configured successfully")
}

func (r *CAMConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CAMConnectorResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate ServiceAccountKey is provided
	if plan.ServiceAccountKey.IsNull() || plan.ServiceAccountKey.IsUnknown() || plan.ServiceAccountKey.ValueString() == "" {
		resp.Diagnostics.AddError(
			"[CAM Connector GCP][Create] Missing Service Account Key",
			"The service_account_key is required. Please provide the GCP service account key as a base64-encoded string.",
		)
		return
	}

	// Validate ServiceAccountKey is valid base64
	if err := r.validateBase64ServiceAccountKey(plan.ServiceAccountKey.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector GCP][Create] Invalid Service Account Key Format",
			fmt.Sprintf("The service_account_key must be a valid base64-encoded string. Error: %s", err.Error()),
		)
		return
	}

	connectedServices, serviceDiags := r.extractConnectedServices(ctx, plan.ConnectedSecurityServices)
	resp.Diagnostics.Append(serviceDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[CAM Connector GCP][Create] Creating GCP connector with name: %s, project number: %s",
		plan.Name.ValueString(), plan.ProjectNumber.ValueString()))

	// Convert organization details if provided
	organization, convertDiags := r.convertOrganizationDetailsToAPI(ctx, plan.Organization)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert folder details if provided
	folder, folderDiags := r.convertFolderDetailsToAPI(ctx, plan.Folder)
	resp.Diagnostics.Append(folderDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := &api.CreateProjectRequest{
		CamDeployedRegion:         plan.CamDeployedRegion.ValueString(),
		ConnectedSecurityServices: connectedServices,
		Description:               plan.Description.ValueString(),
		Folder:                    folder,
		IsCAMCloudASRMEnabled:     plan.IsCAMCloudASRMEnabled.ValueBool(),
		IsTFProviderDeployed:      true,
		Name:                      plan.Name.ValueString(),
		Organization:              organization,
		ProjectNumber:             plan.ProjectNumber.ValueString(),
		ServiceAccountId:          plan.ServiceAccountID.ValueString(),
		ServiceAccountKey:         plan.ServiceAccountKey.ValueString(),
	}

	createErr := r.client.CreateProject(body)
	if createErr != nil {
		// If the project already exists, adopt it instead of failing
		if strings.Contains(createErr.Error(), "account-exist") {
			tflog.Info(ctx, fmt.Sprintf("[CAM Connector GCP][Create] Project %s already exists, adopting existing resource",
				plan.ProjectNumber.ValueString()))
		} else {
			resp.Diagnostics.AddError(
				"[CAM Connector GCP][Create] Error Adding Project",
				fmt.Sprintf("[CAM Connector GCP][Create] Failed to add project: %s", createErr),
			)
			return
		}
	}

	res, err := r.client.ReadProject(plan.ProjectNumber.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector GCP][Create] Error Describing Project",
			fmt.Sprintf("[CAM Connector GCP][Create] Failed to describe project: %s", err),
		)
		return
	}

	if res != nil {
		r.mapResponseToModel(ctx, res, &plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *CAMConnectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CAMConnectorResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	connectedServices, serviceDiags := r.extractConnectedServices(ctx, state.ConnectedSecurityServices)
	resp.Diagnostics.Append(serviceDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.ReadProject(state.ProjectNumber.ValueString())
	if err != nil {
		tflog.Warn(ctx, "[CAM Connector GCP][Read] Failed to describe project, will attempt to create it", map[string]any{
			"error": err.Error(),
		})

		// Convert organization details if provided
		organization, convertDiags := r.convertOrganizationDetailsToAPI(ctx, state.Organization)
		resp.Diagnostics.Append(convertDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Convert folder details if provided
		folder, folderDiags := r.convertFolderDetailsToAPI(ctx, state.Folder)
		resp.Diagnostics.Append(folderDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		body := &api.CreateProjectRequest{
			CamDeployedRegion:         state.CamDeployedRegion.ValueString(),
			ConnectedSecurityServices: connectedServices,
			Description:               state.Description.ValueString(),
			Folder:                    folder,
			IsCAMCloudASRMEnabled:     state.IsCAMCloudASRMEnabled.ValueBool(),
			IsTFProviderDeployed:      true,
			Name:                      state.Name.ValueString(),
			Organization:              organization,
			ProjectNumber:             state.ProjectNumber.ValueString(),
			ServiceAccountId:          state.ServiceAccountID.ValueString(),
			ServiceAccountKey:         state.ServiceAccountKey.ValueString(),
		}

		err = r.client.CreateProject(body)
		if err != nil {
			resp.Diagnostics.AddError(
				"[CAM Connector GCP][Read] Error Adding Project",
				fmt.Sprintf("[CAM Connector GCP][Read] Failed to add project: %s", err),
			)
			return
		}
	} else {
		// Convert organization details if provided
		organization, convertDiags := r.convertOrganizationDetailsToAPI(ctx, state.Organization)
		resp.Diagnostics.Append(convertDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Convert folder details if provided
		folder, folderDiags := r.convertFolderDetailsToAPI(ctx, state.Folder)
		resp.Diagnostics.Append(folderDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		body := &api.ModifyProjectRequest{
			CamDeployedRegion:         state.CamDeployedRegion.ValueString(),
			ConnectedSecurityServices: connectedServices,
			Description:               state.Description.ValueString(),
			Folder:                    folder,
			IsCAMCloudASRMEnabled:     state.IsCAMCloudASRMEnabled.ValueBool(),
			IsTFProviderDeployed:      true,
			Name:                      state.Name.ValueString(),
			Organization:              organization,
			ProjectNumber:             res.ProjectNumber,
			ServiceAccountId:          state.ServiceAccountID.ValueString(),
			ServiceAccountKey:         state.ServiceAccountKey.ValueString(),
		}

		err = r.client.UpdateProject(res.ProjectNumber, body)
		if err != nil {
			// Check if the error indicates the service account is not found
			if strings.Contains(err.Error(), "service account") ||
				strings.Contains(err.Error(), "assume-identity-failed") ||
				strings.Contains(err.Error(), "BadRequest") {
				tflog.Info(ctx, fmt.Sprintf("[CAM Connector GCP][Read] Service Account %s no longer exists, removing from state", state.ServiceAccountID.ValueString()))
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError(
				"[CAM Connector GCP][Read] Error Updating Project",
				fmt.Sprintf("[CAM Connector GCP][Read] Failed to update project: %s", err),
			)
			return
		}
	}

	res, err = r.client.ReadProject(state.ProjectNumber.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector GCP][Read] Error Describing Project",
			fmt.Sprintf("[CAM Connector GCP][Read] Failed to describe project: %s", err),
		)
		return
	}

	if res != nil {
		r.mapResponseToModel(ctx, res, &state, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *CAMConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CAMConnectorResourceModel
	var state CAMConnectorResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[CAM Connector GCP][Update] Project plan: %+v", plan))

	connectedServices, serviceDiags := r.extractConnectedServices(ctx, plan.ConnectedSecurityServices)
	resp.Diagnostics.Append(serviceDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var serviceAccountID string
	if !plan.ServiceAccountID.IsNull() && !plan.ServiceAccountID.IsUnknown() {
		serviceAccountID = plan.ServiceAccountID.ValueString()
	} else {
		serviceAccountID = state.ServiceAccountID.ValueString()
	}

	var isCAMCloudASRMEnabled bool
	if !plan.IsCAMCloudASRMEnabled.IsNull() && !plan.IsCAMCloudASRMEnabled.IsUnknown() {
		isCAMCloudASRMEnabled = plan.IsCAMCloudASRMEnabled.ValueBool()
	} else if !state.IsCAMCloudASRMEnabled.IsNull() && !state.IsCAMCloudASRMEnabled.IsUnknown() {
		isCAMCloudASRMEnabled = state.IsCAMCloudASRMEnabled.ValueBool()
	}

	var projectNumber string
	if !plan.ProjectNumber.IsNull() && !plan.ProjectNumber.IsUnknown() {
		projectNumber = plan.ProjectNumber.ValueString()
	} else {
		projectNumber = state.ProjectNumber.ValueString()
	}

	organization, convertDiags := r.convertOrganizationDetailsToAPI(ctx, plan.Organization)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	folder, folderDiags := r.convertFolderDetailsToAPI(ctx, plan.Folder)
	resp.Diagnostics.Append(folderDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var serviceAccountKey string
	if !plan.ServiceAccountKey.IsNull() && !plan.ServiceAccountKey.IsUnknown() {
		serviceAccountKey = plan.ServiceAccountKey.ValueString()
	} else {
		serviceAccountKey = state.ServiceAccountKey.ValueString()
	}

	body := &api.ModifyProjectRequest{
		CamDeployedRegion:         plan.CamDeployedRegion.ValueString(),
		ConnectedSecurityServices: connectedServices,
		Description:               plan.Description.ValueString(),
		Folder:                    folder,
		IsCAMCloudASRMEnabled:     isCAMCloudASRMEnabled,
		IsTFProviderDeployed:      true,
		Name:                      plan.Name.ValueString(),
		Organization:              organization,
		ProjectNumber:             projectNumber,
		ServiceAccountId:          serviceAccountID,
		ServiceAccountKey:         serviceAccountKey,
	}

	err := r.client.UpdateProject(projectNumber, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector GCP][Update] Error Updating Project",
			fmt.Sprintf("[CAM Connector GCP][Update] Failed to update project: %s", err),
		)
		return
	}

	res, err := r.client.ReadProject(plan.ProjectNumber.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector GCP][Update] Error Describing Project",
			fmt.Sprintf("[CAM Connector GCP][Update] Failed to describe project: %s", err),
		)
		return
	}

	if res != nil {
		state.ID = types.StringValue(plan.ID.ValueString())
		r.mapResponseToModel(ctx, res, &state, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		// Preserve organization from plan since API response might not include all details
		state.Organization = plan.Organization
		state.Folder = plan.Folder
	}

	resp.State.Set(ctx, &state)
}

func (r *CAMConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CAMConnectorResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteProject(state.ProjectNumber.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector GCP][Delete] Error Removing Project",
			fmt.Sprintf("[CAM Connector GCP][Delete] Failed to remove project: %s", err),
		)
		return
	}
}

// ===== Helper Functions =====
func (r *CAMConnectorResource) extractConnectedServices(ctx context.Context, servicesList types.List) ([]cam.ConnectedSecurityService, diag.Diagnostics) {
	var diags diag.Diagnostics
	var connectedServices []cam.ConnectedSecurityService

	if servicesList.IsNull() || servicesList.IsUnknown() {
		return []cam.ConnectedSecurityService{}, diags
	}

	var securityServiceModels []SecurityServiceModel
	extractDiags := servicesList.ElementsAs(ctx, &securityServiceModels, false)
	diags.Append(extractDiags...)
	if diags.HasError() {
		return connectedServices, diags
	}

	if len(securityServiceModels) == 0 {
		return []cam.ConnectedSecurityService{}, diags
	}

	for _, model := range securityServiceModels {
		var instanceIds []string
		if model.InstanceIds.IsNull() || model.InstanceIds.IsUnknown() {
			instanceIds = []string{}
		} else {
			extractDiags := model.InstanceIds.ElementsAs(ctx, &instanceIds, false)
			diags.Append(extractDiags...)
			if diags.HasError() {
				return connectedServices, diags
			}
		}

		connectedServices = append(connectedServices, cam.ConnectedSecurityService{
			Name:        model.Name.ValueString(),
			InstanceIds: instanceIds,
		})
	}

	return connectedServices, diags
}

// convertOrganizationDetailsToAPI converts Terraform organization model to API format
func (r *CAMConnectorResource) convertOrganizationDetailsToAPI(ctx context.Context, org *OrganizationDetailsModel) (*api.OrganizationDetails, diag.Diagnostics) {
	var diags diag.Diagnostics

	if org == nil || org.ID.IsNull() {
		return nil, diags
	}

	var excludedProjectsStr string
	if !org.ExcludedProjects.IsNull() {
		var excludedProjects []string
		convertDiags := org.ExcludedProjects.ElementsAs(ctx, &excludedProjects, false)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return nil, diags
		}
		// Convert array to comma-separated string for backend API
		excludedProjectsStr = strings.Join(excludedProjects, ",")
	}

	organization := &api.OrganizationDetails{
		ID:               org.ID.ValueString(),
		DisplayName:      org.DisplayName.ValueString(),
		ExcludedProjects: excludedProjectsStr,
	}

	return organization, diags
}

// convertFolderDetailsToAPI converts Terraform folder model to API format
func (r *CAMConnectorResource) convertFolderDetailsToAPI(ctx context.Context, folder *FolderDetailsModel) (*api.FolderDetails, diag.Diagnostics) {
	var diags diag.Diagnostics

	if folder == nil || folder.ID.IsNull() {
		return nil, diags
	}

	var excludedProjectsStr string
	if !folder.ExcludedProjects.IsNull() {
		var excludedProjects []string
		convertDiags := folder.ExcludedProjects.ElementsAs(ctx, &excludedProjects, false)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return nil, diags
		}
		excludedProjectsStr = strings.Join(excludedProjects, ",")
	}

	f := &api.FolderDetails{
		ID:               folder.ID.ValueString(),
		DisplayName:      folder.DisplayName.ValueString(),
		ExcludedProjects: excludedProjectsStr,
	}

	return f, diags
}

// convertAPISecurityServicesToTerraform converts API security services to Terraform list
func (r *CAMConnectorResource) convertAPISecurityServicesToTerraform(ctx context.Context, apiServices []cam.ConnectedSecurityService) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	objectType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":         types.StringType,
			"instance_ids": types.ListType{ElemType: types.StringType},
		},
	}

	if len(apiServices) == 0 {
		emptyList, listDiags := types.ListValue(objectType, []attr.Value{})
		diags.Append(listDiags...)
		return emptyList, diags
	}

	var serviceModels []SecurityServiceModel
	for _, apiService := range apiServices {
		instanceIdsList, instanceDiags := types.ListValueFrom(ctx, types.StringType, apiService.InstanceIds)
		diags.Append(instanceDiags...)
		if diags.HasError() {
			return types.List{}, diags
		}

		serviceModels = append(serviceModels, SecurityServiceModel{
			Name:        types.StringValue(apiService.Name),
			InstanceIds: instanceIdsList,
		})
	}

	resultList, listDiags := types.ListValueFrom(ctx, objectType, serviceModels)
	diags.Append(listDiags...)
	return resultList, diags
}

// validateBase64ServiceAccountKey validates that the service account key is a valid base64-encoded string
func (r *CAMConnectorResource) validateBase64ServiceAccountKey(key string) error {
	if key == "" {
		return fmt.Errorf("service account key cannot be empty")
	}

	// Attempt to decode the base64 string
	_, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("invalid base64 encoding: %w", err)
	}

	return nil
}

// mapResponseToModel maps the API response to the Terraform model
func (r *CAMConnectorResource) mapResponseToModel(ctx context.Context, res *api.ProjectResponse, model *CAMConnectorResourceModel, diags *diag.Diagnostics) {
	model.ID = types.StringValue(res.ProjectNumber)
	model.ProjectNumber = types.StringValue(res.ProjectNumber)
	model.State = types.StringValue(res.State)

	if res.Description != "" {
		model.Description = types.StringValue(res.Description)
	}

	// Note: IsCAMCloudASRMEnabled is a user-provided required field.
	// We preserve it from the plan/state and do NOT overwrite from API response
	// because the API may not reflect the value immediately after creation.

	model.Name = types.StringValue(res.Name)
	model.CreatedDateTime = types.StringValue(res.CreatedTime)
	model.UpdatedDateTime = types.StringValue(res.UpdatedDateTime)

	// Set computed GCP-specific fields
	if res.ProjectID != "" {
		model.ProjectID = types.StringValue(res.ProjectID)
	}
	if res.ProjectName != "" {
		model.ProjectName = types.StringValue(res.ProjectName)
	}
	if res.ServiceAccountEmail != "" {
		model.ServiceAccountEmail = types.StringValue(res.ServiceAccountEmail)
	}
	// Note: ServiceAccountID and ServiceAccountKey are user-provided required fields.
	// We preserve them from the plan/state and do NOT overwrite from API response
	// because the API may not return these values or may return different formats.
	if res.CamDeployedRegion != "" {
		model.CamDeployedRegion = types.StringValue(res.CamDeployedRegion)
	}

	// Handle connected security services
	// Always update from API response if available, otherwise preserve from plan/state
	if len(res.ConnectedSecurityServices) > 0 {
		connectedServicesList, convertDiags := r.convertAPISecurityServicesToTerraform(ctx, res.ConnectedSecurityServices)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return
		}
		model.ConnectedSecurityServices = connectedServicesList
	} else if model.ConnectedSecurityServices.IsNull() || model.ConnectedSecurityServices.IsUnknown() {
		// Only set to null if it was already null/unknown and API returned empty
		model.ConnectedSecurityServices = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"name":         types.StringType,
				"instance_ids": types.ListType{ElemType: types.StringType},
			},
		})
	}
}
