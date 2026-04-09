package azure

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/azure/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                     = &CAMConnectorResource{}
	_ resource.ResourceWithConfigure        = &CAMConnectorResource{}
	_ resource.ResourceWithConfigValidators = &CAMConnectorResource{}
)

type SecurityServiceModel struct {
	Name        types.String `tfsdk:"name"`
	InstanceIds types.List   `tfsdk:"instance_ids"`
}

type AzureFeatureModel struct {
	ID      types.String `tfsdk:"id"`
	Regions types.List   `tfsdk:"regions"`
}

func NewCAMConnectorResource() resource.Resource {
	return &CAMConnectorResource{}
}

type CAMConnectorResource struct {
	client *api.CamClient
}

// CAMConnectorResourceModel describes the resource data model.
type CAMConnectorResourceModel struct {
	ApplicationID             types.String                `tfsdk:"application_id"`
	ConnectedSecurityServices types.List                  `tfsdk:"connected_security_services"`
	CreatedDateTime           types.String                `tfsdk:"created_date_time"`
	Description               types.String                `tfsdk:"description"`
	ID                        types.String                `tfsdk:"id"`
	IsCAMCloudASRMEnabled     types.Bool                  `tfsdk:"is_cam_cloud_asrm_enabled"`
	Name                      types.String                `tfsdk:"name"`
	SubscriptionID            types.String                `tfsdk:"subscription_id"`
	State                     types.String                `tfsdk:"state"`
	TenantID                  types.String                `tfsdk:"tenant_id"`
	UpdatedDateTime           types.String                `tfsdk:"updated_date_time"`
	ManagementGroupDetails    *ManagementGroupDetailsModel `tfsdk:"management_group_details"`
	IsSharedApplication       types.Bool                  `tfsdk:"is_shared_application"`
	CamDeployedRegion         types.String                `tfsdk:"cam_deployed_region"`
	Features                  types.List                  `tfsdk:"features"`
	FeaturesConfigFilePath    types.String                `tfsdk:"features_config_file_path"`
	PreventDestroy            types.Bool                  `tfsdk:"prevent_destroy"`
}

type ManagementGroupDetailsModel struct {
	ID                    types.String `tfsdk:"id"`
	DisplayName           types.String `tfsdk:"display_name"`
	ExcludedSubscriptions types.List   `tfsdk:"excluded_subscriptions"`
}

func (r *CAMConnectorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_CONNECTOR_AZURE
}

func (r *CAMConnectorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Azure connector for Trend Micro Vision One CAM",
		Attributes: map[string]schema.Attribute{
			"application_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Azure application ID which is used to connect to the Azure subscription",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"connected_security_services": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of connected security services for the connector",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Name of the security service",
						},
						"instance_ids": schema.ListAttribute{
							ElementType:         types.StringType,
							Required:            true,
							MarkdownDescription: "List of instance IDs for the security service",
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
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the connector",
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
			"subscription_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Azure subscription ID for the connector",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current state of the connector",
			},
			"tenant_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Azure tenant ID for the connector",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"updated_date_time": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the connector was last updated",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"management_group_details": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Azure management group details for the connector",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Azure management group ID",
					},
					"display_name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Display name of the management group",
					},
					"excluded_subscriptions": schema.ListAttribute{
						ElementType:         types.StringType,
						Optional:            true,
						MarkdownDescription: "List of subscription IDs to exclude from the management group",
					},
				},
			},
			"is_shared_application": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the application is shared across multiple connectors",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"cam_deployed_region": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Region where CAM is deployed for this connector",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"features": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of features to enable for the connector",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Feature identifier",
						},
						"regions": schema.ListAttribute{
							ElementType:         types.StringType,
							Optional:            true,
							MarkdownDescription: "List of regions to enable the feature in",
						},
					},
				},
			},
			"features_config_file_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to the features configuration file",
			},
			"prevent_destroy": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When `true` (default), Terraform destroy will not call the CAM DELETE API, preserving the subscription in CAM. Set to `false` to allow the subscription to be removed from CAM on destroy.",
				Default:             booldefault.StaticBool(true),
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
		Client: client.WithTimeout(cam.CAMAPITimeout),
	}
	tflog.Debug(ctx, "[CAM Connector] CAM Connector resource configured successfully")
}

func (r *CAMConnectorResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		featuresConfigFilePathRequiresFeaturesValidator{},
	}
}

type featuresConfigFilePathRequiresFeaturesValidator struct{}

func (v featuresConfigFilePathRequiresFeaturesValidator) Description(_ context.Context) string {
	return "features_config_file_path requires features to also be set"
}

func (v featuresConfigFilePathRequiresFeaturesValidator) MarkdownDescription(_ context.Context) string {
	return "`features_config_file_path` requires `features` to also be set"
}

func (v featuresConfigFilePathRequiresFeaturesValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data CAMConnectorResourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.FeaturesConfigFilePath.IsNull() && !data.FeaturesConfigFilePath.IsUnknown() && data.FeaturesConfigFilePath.ValueString() != "" {
		if data.Features.IsNull() || data.Features.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("features_config_file_path"),
				"Invalid Attribute Combination",
				"features_config_file_path cannot be set without features.",
			)
		}
	}
}

func (r *CAMConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CAMConnectorResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var connectedServices []cam.ConnectedSecurityService
	if !plan.ConnectedSecurityServices.IsNull() {
		var securityServiceModels []SecurityServiceModel
		diags = plan.ConnectedSecurityServices.ElementsAs(ctx, &securityServiceModels, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, model := range securityServiceModels {
			var instanceIds []string
			diags = model.InstanceIds.ElementsAs(ctx, &instanceIds, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			connectedServices = append(connectedServices, cam.ConnectedSecurityService{
				Name:        model.Name.ValueString(),
				InstanceIds: instanceIds,
			})
		}
	}

	tflog.Debug(ctx, fmt.Sprintf("[CAM Connector][Create] Creating Azure connector with name: %s, subscription ID: %s, tenant ID: %s",
		plan.Name.ValueString(), plan.SubscriptionID.ValueString(), plan.TenantID.ValueString()))

	// Convert management group details if provided
	managementGroup, convertDiags := convertManagementGroupDetailsToAPI(ctx, plan.ManagementGroupDetails)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	features, featureDiags := extractFeatures(ctx, plan.Features)
	resp.Diagnostics.Append(featureDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := &api.CreateSubscriptionRequest{
		ApplicationID:             plan.ApplicationID.ValueString(),
		ConnectedSecurityServices: connectedServices,
		Description:               plan.Description.ValueString(),
		IsCAMCloudASRMEnabled:     plan.IsCAMCloudASRMEnabled.ValueBool(),
		Name:                      plan.Name.ValueString(),
		SubscriptionID:            plan.SubscriptionID.ValueString(),
		TenantID:                  plan.TenantID.ValueString(),
		ManagementGroup:           managementGroup,
		IsSharedApplication:       plan.IsSharedApplication.ValueBool(),
		CamDeployedRegion:         plan.CamDeployedRegion.ValueString(),
		IsTFProviderDeployed:      true,
		Features:                  features,
		FeaturesConfigFilePath:    plan.FeaturesConfigFilePath.ValueString(),
	}

	createErr := r.client.CreateSubscription(body)
	if createErr != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Create] Error Adding Subscription",
			fmt.Sprintf("[CAM Connector][Create] Failed to add subscription: %s", createErr),
		)
		return
	}
	res, err := r.client.ReadSubscription(plan.SubscriptionID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Create] Error Describing Subscription",
			fmt.Sprintf("[CAM Connector][Create] Failed to describe subscription: %s", err),
		)
		return
	}
	if res != nil {
		plan.ID = types.StringValue(res.SubscriptionID)
		plan.ApplicationID = types.StringValue(res.ApplicationID)
		plan.State = types.StringValue(res.State)
		if res.Description != "" {
			plan.Description = types.StringValue(res.Description)
		}
		plan.IsCAMCloudASRMEnabled = types.BoolValue(res.IsCAMCloudASRMEnabled)
		plan.Name = types.StringValue(res.Name)
		plan.TenantID = types.StringValue(res.TenantID)
		plan.CreatedDateTime = types.StringValue(res.CreatedDateTime)
		plan.UpdatedDateTime = types.StringValue(res.UpdatedDateTime)

		// Set cam_deployed_region from API response
		if res.CamDeployedRegion != "" {
			plan.CamDeployedRegion = types.StringValue(res.CamDeployedRegion)
		}

		// Preserve management_group_details and is_shared_application from plan
		// since API response (SubscriptionResponse) doesn't return these fields

		if !plan.ConnectedSecurityServices.IsNull() {
			connectedServicesList, convertDiags := convertAPISecurityServicesToTerraform(ctx, res.ConnectedSecurityServices)
			resp.Diagnostics.Append(convertDiags...)
			if resp.Diagnostics.HasError() {
				return
			}
			plan.ConnectedSecurityServices = connectedServicesList
		} else {
			plan.ConnectedSecurityServices = types.ListNull(types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"name":         types.StringType,
					"instance_ids": types.ListType{ElemType: types.StringType},
				},
			})
		}

		// Do not auto-populate features from API response when the user did not specify them.
		// features has omitempty on the backend; keeping it null avoids sending stale or
		// unsupported feature IDs on subsequent re-create operations in Read.
		// Preserve FeaturesConfigFilePath from plan since API does not return it
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

	var connectedServices []cam.ConnectedSecurityService
	if !state.ConnectedSecurityServices.IsNull() {
		var securityServiceModels []SecurityServiceModel
		diags = state.ConnectedSecurityServices.ElementsAs(ctx, &securityServiceModels, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, model := range securityServiceModels {
			var instanceIds []string
			diags = model.InstanceIds.ElementsAs(ctx, &instanceIds, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			connectedServices = append(connectedServices, cam.ConnectedSecurityService{
				Name:        model.Name.ValueString(),
				InstanceIds: instanceIds,
			})
		}
	}

	res, err := r.client.ReadSubscription(state.SubscriptionID.ValueString())
	if err != nil {
		tflog.Warn(ctx, "[CAM Connector][Read] Failed to describe subscription, will attempt to create it", map[string]any{
			"error": err.Error(),
		})

		// Convert management group details if provided
		managementGroup, convertDiags := convertManagementGroupDetailsToAPI(ctx, state.ManagementGroupDetails)
		resp.Diagnostics.Append(convertDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		stateFeatures, featureDiags := extractFeatures(ctx, state.Features)
		resp.Diagnostics.Append(featureDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		body := &api.CreateSubscriptionRequest{
			ApplicationID:             state.ApplicationID.ValueString(),
			ConnectedSecurityServices: connectedServices,
			Description:               state.Description.ValueString(),
			IsCAMCloudASRMEnabled:     state.IsCAMCloudASRMEnabled.ValueBool(),
			Name:                      state.Name.ValueString(),
			SubscriptionID:            state.SubscriptionID.ValueString(),
			TenantID:                  state.TenantID.ValueString(),
			ManagementGroup:           managementGroup,
			IsSharedApplication:       state.IsSharedApplication.ValueBool(),
			CamDeployedRegion:         state.CamDeployedRegion.ValueString(),
			IsTFProviderDeployed:      true,
			Features:                  stateFeatures,
			FeaturesConfigFilePath:    state.FeaturesConfigFilePath.ValueString(),
		}

		err = r.client.CreateSubscription(body)
		if err != nil {
			resp.Diagnostics.AddError(
				"[CAM Connector][Read] Error Adding Subscription",
				fmt.Sprintf("[CAM Connector][Read] Failed to add subscription: %s", err),
			)
			return
		}
	} else {
		// Convert management group details if provided
		managementGroup, convertDiags := convertManagementGroupDetailsToAPI(ctx, state.ManagementGroupDetails)
		resp.Diagnostics.Append(convertDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		stateFeatures, featureDiags := extractFeatures(ctx, state.Features)
		resp.Diagnostics.Append(featureDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		targetName := state.Name.ValueString()
		if res.Name != "" && res.Name != state.Name.ValueString() {
			tflog.Info(ctx, fmt.Sprintf("[CAM Connector][Read] Backend name %q differs from state name %q, using backend name as target", res.Name, state.Name.ValueString()))
			targetName = res.Name
		}

		body := &api.ModifySubscriptionRequest{
			ApplicationID:             state.ApplicationID.ValueString(),
			ConnectedSecurityServices: connectedServices,
			Description:               state.Description.ValueString(),
			IsCAMCloudASRMEnabled:     state.IsCAMCloudASRMEnabled.ValueBool(),
			Name:                      targetName,
			SubscriptionID:            res.SubscriptionID,
			TenantID:                  state.TenantID.ValueString(),
			ManagementGroup:           managementGroup,
			IsSharedApplication:       state.IsSharedApplication.ValueBool(),
			CamDeployedRegion:         state.CamDeployedRegion.ValueString(),
			IsTFProviderDeployed:      true,
			Features:                  stateFeatures,
			FeaturesConfigFilePath:    state.FeaturesConfigFilePath.ValueString(),
		}

		err = r.client.UpdateSubscription(res.SubscriptionID, body)
		if err != nil {
			// Check if the error indicates the application ID is not found in the tenant
			if strings.Contains(err.Error(), "application ID provided within your request was not found") ||
				strings.Contains(err.Error(), "assume-identity-failed") ||
				strings.Contains(err.Error(), "BadRequest") {
				tflog.Info(ctx, fmt.Sprintf("[CAM Connector][Read] Application ID %s no longer exists in tenant, removing from state", state.ApplicationID.ValueString()))
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError(
				"[CAM Connector][Read] Error Updating Subscription",
				fmt.Sprintf("[CAM Connector][Read] Failed to update subscription: %s", err),
			)
			return
		}
	}

	res, err = r.client.ReadSubscription(state.SubscriptionID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Read] Error Describing Subscription",
			fmt.Sprintf("[CAM Connector][Read] Failed to describe subscription: %s", err),
		)
		return
	}

	if res != nil {
		state.ID = types.StringValue(res.SubscriptionID)
		state.ApplicationID = types.StringValue(res.ApplicationID)
		state.State = types.StringValue(res.State)
		if res.Description != "" {
			state.Description = types.StringValue(res.Description)
		}
		state.IsCAMCloudASRMEnabled = types.BoolValue(res.IsCAMCloudASRMEnabled)
		// Preserve state name: if backend name differs from state, we already sent PATCH
		// with backend name to avoid overwriting UI changes. Keep state name as-is so
		// Terraform does not see drift and force replacement.
		if res.Name != "" && res.Name == state.Name.ValueString() {
			state.Name = types.StringValue(res.Name)
		}
		state.TenantID = types.StringValue(res.TenantID)
		state.CreatedDateTime = types.StringValue(res.CreatedDateTime)
		state.UpdatedDateTime = types.StringValue(res.UpdatedDateTime)

		// Set cam_deployed_region from API response
		if res.CamDeployedRegion != "" {
			state.CamDeployedRegion = types.StringValue(res.CamDeployedRegion)
		}

		// Preserve management_group_details and is_shared_application from state
		// since API response (SubscriptionResponse) doesn't return these fields

		if !state.ConnectedSecurityServices.IsNull() {
			connectedServicesList, convertDiags := convertAPISecurityServicesToTerraform(ctx, res.ConnectedSecurityServices)
			resp.Diagnostics.Append(convertDiags...)
			if resp.Diagnostics.HasError() {
				return
			}
			state.ConnectedSecurityServices = connectedServicesList
		} else {
			state.ConnectedSecurityServices = types.ListNull(types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"name":         types.StringType,
					"instance_ids": types.ListType{ElemType: types.StringType},
				},
			})
		}

		// Do not auto-populate features from API response when the user did not specify them.
		// features has omitempty on the backend; keeping it null avoids sending stale or
		// unsupported feature IDs on subsequent re-create operations in Read.
		// Preserve FeaturesConfigFilePath from state since API does not return it
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

	tflog.Debug(ctx, fmt.Sprintf("[CAM Connector][Update] Cluster plan: %+v", plan))

	var applicationID string
	if !plan.ApplicationID.IsNull() && !plan.ApplicationID.IsUnknown() {
		applicationID = plan.ApplicationID.ValueString()
	} else {
		applicationID = state.ApplicationID.ValueString()
	}

	var connectedServices []cam.ConnectedSecurityService
	if !plan.ConnectedSecurityServices.IsNull() {
		var securityServiceModels []SecurityServiceModel
		diags := plan.ConnectedSecurityServices.ElementsAs(ctx, &securityServiceModels, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, model := range securityServiceModels {
			var instanceIds []string
			diags := model.InstanceIds.ElementsAs(ctx, &instanceIds, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			connectedServices = append(connectedServices, cam.ConnectedSecurityService{
				Name:        model.Name.ValueString(),
				InstanceIds: instanceIds,
			})
		}
	}

	var isCAMCloudASRMEnabled bool
	if !plan.IsCAMCloudASRMEnabled.IsNull() && !plan.IsCAMCloudASRMEnabled.IsUnknown() {
		isCAMCloudASRMEnabled = plan.IsCAMCloudASRMEnabled.ValueBool()
	} else if !state.IsCAMCloudASRMEnabled.IsNull() && !state.IsCAMCloudASRMEnabled.IsUnknown() {
		isCAMCloudASRMEnabled = state.IsCAMCloudASRMEnabled.ValueBool()
	}

	var subscriptionID string
	if !plan.SubscriptionID.IsNull() && !plan.SubscriptionID.IsUnknown() {
		subscriptionID = plan.SubscriptionID.ValueString()
	} else {
		subscriptionID = state.SubscriptionID.ValueString()
	}

	var tenantID string
	if !plan.TenantID.IsNull() && !plan.TenantID.IsUnknown() {
		tenantID = plan.TenantID.ValueString()
	} else {
		tenantID = state.TenantID.ValueString()
	}

	// Convert management group details if provided
	managementGroup, convertDiags := convertManagementGroupDetailsToAPI(ctx, plan.ManagementGroupDetails)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	features, featureDiags := extractFeatures(ctx, plan.Features)
	resp.Diagnostics.Append(featureDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := &api.ModifySubscriptionRequest{
		ApplicationID:             applicationID,
		ConnectedSecurityServices: connectedServices,
		Description:               plan.Description.ValueString(),
		IsCAMCloudASRMEnabled:     isCAMCloudASRMEnabled,
		Name:                      plan.Name.ValueString(),
		SubscriptionID:            subscriptionID,
		TenantID:                  tenantID,
		ManagementGroup:           managementGroup,
		IsSharedApplication:       plan.IsSharedApplication.ValueBool(),
		CamDeployedRegion:         plan.CamDeployedRegion.ValueString(),
		IsTFProviderDeployed:      true,
		Features:                  features,
		FeaturesConfigFilePath:    plan.FeaturesConfigFilePath.ValueString(),
	}

	err := r.client.UpdateSubscription(subscriptionID, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Update] Error Updating Subscription",
			fmt.Sprintf("[CAM Connector][Update] Failed to update subscription: %s", err),
		)
		return
	}

	res, err := r.client.ReadSubscription(plan.SubscriptionID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Update] Error Describing Subscription",
			fmt.Sprintf("[CAM Connector][Update] Failed to describe subscription: %s", err),
		)
		return
	}

	if res != nil {
		state.ID = types.StringValue(plan.ID.ValueString())
		state.ApplicationID = types.StringValue(res.ApplicationID)
		state.State = types.StringValue(res.State)
		state.Description = types.StringValue(res.Description)
		state.IsCAMCloudASRMEnabled = types.BoolValue(res.IsCAMCloudASRMEnabled)
		state.Name = types.StringValue(res.Name)
		state.TenantID = types.StringValue(res.TenantID)
		state.CreatedDateTime = types.StringValue(res.CreatedDateTime)
		state.UpdatedDateTime = types.StringValue(res.UpdatedDateTime)

		// Set cam_deployed_region from API response
		if res.CamDeployedRegion != "" {
			state.CamDeployedRegion = types.StringValue(res.CamDeployedRegion)
		}

		// Preserve management_group_details and is_shared_application from plan
		// since API response doesn't return these fields
		state.ManagementGroupDetails = plan.ManagementGroupDetails
		state.IsSharedApplication = plan.IsSharedApplication

		if !plan.ConnectedSecurityServices.IsNull() {
			connectedServicesList, convertDiags := convertAPISecurityServicesToTerraform(ctx, res.ConnectedSecurityServices)
			resp.Diagnostics.Append(convertDiags...)
			if resp.Diagnostics.HasError() {
				return
			}
			state.ConnectedSecurityServices = connectedServicesList
		} else {
			state.ConnectedSecurityServices = types.ListNull(types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"name":         types.StringType,
					"instance_ids": types.ListType{ElemType: types.StringType},
				},
			})
		}

		// Use plan features (null when user did not specify them).
		// Do not fall back to API response features: features has omitempty on the backend
		// and auto-populating from API can persist unsupported feature IDs into state.
		state.Features = plan.Features
		// Preserve FeaturesConfigFilePath from plan since API does not return it
		state.FeaturesConfigFilePath = plan.FeaturesConfigFilePath
		// Preserve prevent_destroy from plan since API does not return it
		state.PreventDestroy = plan.PreventDestroy
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

	if state.PreventDestroy.IsNull() || state.PreventDestroy.IsUnknown() || state.PreventDestroy.ValueBool() {
		tflog.Info(ctx, fmt.Sprintf("[CAM Connector][Delete] prevent_destroy=true (or unset), skipping CAM DELETE for subscription %s", state.SubscriptionID.ValueString()))
		return
	}

	err := r.client.DeleteSubscription(state.SubscriptionID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Delete] Error Removing Subscription",
			fmt.Sprintf("[CAM Connector][Delete] Failed to remove subscription: %s", err),
		)
		return
	}
}

func convertManagementGroupDetailsToAPI(ctx context.Context, mgmtGroup *ManagementGroupDetailsModel) (api.ManagementGroupDetails, diag.Diagnostics) {
	var diags diag.Diagnostics
	var managementGroup api.ManagementGroupDetails

	if mgmtGroup == nil {
		return managementGroup, diags
	}

	if !mgmtGroup.ID.IsNull() {
		var excludedSubsStr string
		if !mgmtGroup.ExcludedSubscriptions.IsNull() {
			var excludedSubs []string
			convertDiags := mgmtGroup.ExcludedSubscriptions.ElementsAs(ctx, &excludedSubs, false)
			diags.Append(convertDiags...)
			if diags.HasError() {
				return managementGroup, diags
			}
			// Convert array to comma-separated string for backend API
			excludedSubsStr = strings.Join(excludedSubs, ",")
		}
		managementGroup = api.ManagementGroupDetails{
			ID:                    mgmtGroup.ID.ValueString(),
			DisplayName:           mgmtGroup.DisplayName.ValueString(),
			ExcludedSubscriptions: excludedSubsStr,
		}
	}

	return managementGroup, diags
}

func extractFeatures(ctx context.Context, featuresList types.List) (*[]api.Feature, diag.Diagnostics) {
	var diags diag.Diagnostics

	// null means the user did not specify features — omit the field entirely from the request.
	if featuresList.IsNull() || featuresList.IsUnknown() {
		return nil, diags
	}

	var featureModels []AzureFeatureModel
	extractDiags := featuresList.ElementsAs(ctx, &featureModels, false)
	diags.Append(extractDiags...)
	if diags.HasError() {
		return nil, diags
	}

	// Empty list means the user explicitly cleared all features — send [] to the backend.
	features := make([]api.Feature, 0, len(featureModels))
	for _, model := range featureModels {
		var regions []string
		if !model.Regions.IsNull() && !model.Regions.IsUnknown() {
			regionDiags := model.Regions.ElementsAs(ctx, &regions, false)
			diags.Append(regionDiags...)
			if diags.HasError() {
				return nil, diags
			}
		}
		features = append(features, api.Feature{
			ID:      model.ID.ValueString(),
			Regions: regions,
		})
	}

	return &features, diags
}


func convertAPISecurityServicesToTerraform(ctx context.Context, apiServices []cam.ConnectedSecurityService) (types.List, diag.Diagnostics) {
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
