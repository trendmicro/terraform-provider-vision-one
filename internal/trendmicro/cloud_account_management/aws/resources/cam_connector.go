package aws

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/aws/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/aws/resources/config"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                     = &CAMConnectorResource{}
	_ resource.ResourceWithConfigure        = &CAMConnectorResource{}
	_ resource.ResourceWithConfigValidators = &CAMConnectorResource{}
)

type AWSFeatureModel struct {
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
	// ── Required ──
	CloudAccountID types.String `tfsdk:"cloud_account_id"`
	RoleArn        types.String `tfsdk:"role_arn"`

	// ── Computed ──
	ID              types.String `tfsdk:"id"`
	State           types.String `tfsdk:"state"`
	CreatedDateTime types.String `tfsdk:"created_date_time"`
	UpdatedDateTime types.String `tfsdk:"updated_date_time"`

	// ── Optional ──
	Name                            types.String `tfsdk:"name"`
	Description                     types.String `tfsdk:"description"`
	OrganizationID                  types.String `tfsdk:"organization_id"`
	Features                        types.List   `tfsdk:"features"`
	FeaturesConfigFilePath          types.String `tfsdk:"features_config_file_path"`
	IsCremEnabled                   types.Bool   `tfsdk:"is_crem_enabled"`
	IsTFProviderDeployed            types.Bool   `tfsdk:"is_tf_provider_deployed"`
	IsAwsOrgMgmtAccount             types.Bool   `tfsdk:"is_aws_org_mgmt_account"`
	OrganizationExcludedAccounts      types.List   `tfsdk:"organization_excluded_accounts"`
	TargetOrganizationalUnitIDs       types.List   `tfsdk:"target_organizational_unit_ids"`
	CustomTags                        types.Map    `tfsdk:"custom_tags"`
	ConnectedSecurityServices       types.List   `tfsdk:"connected_security_services"`
	ServerWorkloadProtectionRegions types.List   `tfsdk:"server_workload_protection_regions"`
	CamDeployedRegion               types.String `tfsdk:"cam_deployed_region"`
	PreventDestroy                  types.Bool   `tfsdk:"prevent_destroy"`
}

func (r *CAMConnectorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_CONNECTOR_AWS
}

func (r *CAMConnectorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an AWS connector for Trend Micro Vision One CAM",
		Attributes: map[string]schema.Attribute{
			"cloud_account_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "AWS account ID (12-digit). Immutable — changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
				Validators: []validator.String{
					stringvalidator.LengthAtMost(254),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the connector (equals cloud_account_id)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"is_crem_enabled": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether Trend Vision One Cloud CREM (isCAMCloudASRMEnabled) is enabled for the connector",
			},
			"is_tf_provider_deployed": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Audit tag marking this account as onboarded via the Terraform provider. Defaults to `true`.",
				Default:             booldefault.StaticBool(true),
			},
			"is_aws_org_mgmt_account": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Marks this as the AWS Organization management account. Requires `organization_id`.",
			},
			"organization_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "AWS Organization/OU/root ID. Accepts bare org ID (`o-`), OU ID (`ou-`), or root ID (`r-`). Sent as `tmv1-organizationID` header.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(o-[a-z0-9]{10,32}|ou-[a-z0-9]+-[a-z0-9]{8,32}|r-[a-z0-9]{4,32})$`),
						"must be an AWS Organization ID (o-<alphanum10-32>), OU ID (ou-<id>-<alphanum8-32>), or root ID (r-<alphanum4-32>)",
					),
				},
			},
			"organization_excluded_accounts": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "AWS account IDs (12-digit) excluded from organization onboarding. Requires `organization_id`.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(
						stringvalidator.RegexMatches(
							regexp.MustCompile(`^\d{12}$`),
							"each entry must be a 12-digit AWS account ID",
						),
					),
				},
			},
			"target_organizational_unit_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "StackSet deployment-target OUs or root for AWS Organization onboarding. Each entry must be an OU ID (`ou-`) or root ID (`r-`). Requires `organization_id`.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(
						stringvalidator.RegexMatches(
							regexp.MustCompile(`^(ou-[a-z0-9]+-[a-z0-9]{8,32}|r-[a-z0-9]{4,32})$`),
							"each entry must be an OU ID (ou-<id>-<alphanum8-32>) or root ID (r-<alphanum4-32>)",
						),
					),
				},
			},
			"custom_tags": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Custom tags to apply to the connector (key-value pairs).",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name of the connector. The backend may normalize this (e.g. account alias takes precedence); the resolved value is stored in state.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtMost(254),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current state of the connector",
			},
			"updated_date_time": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the connector was last updated",
			},
			"role_arn": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "AWS IAM Role ARN used by Trend Micro Vision One to access the AWS account",
				Validators: []validator.String{
					stringvalidator.LengthBetween(20, 2048),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
			"connected_security_services": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Connected security services (e.g. workload/SWP). Required when the Vision One tenant has an active security service instance.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Name of the security service (e.g. `workload`)",
							Validators: []validator.String{
								stringvalidator.OneOf(
									"workload",
								),
							},
						},
						"instance_ids": schema.ListAttribute{
							ElementType:         types.StringType,
							Optional:            true,
							MarkdownDescription: "Exactly one workload instance UUID",
						},
						"regions": schema.ListAttribute{
							ElementType:         types.StringType,
							Optional:            true,
							MarkdownDescription: "List of AWS regions for the security service",
						},
					},
				},
			},
			"server_workload_protection_regions": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Legacy/fallback list of AWS regions for Server & Workload Protection. Honored only when `connected_security_services` is absent.",
			},
			"cam_deployed_region": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "AWS region where the CAM connector is deployed. Derived from `VisionOneBaseRegion` tag on the VisionOneRole; stored in state only — not sent to the API.",
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
		connectedSecurityServicesValidator{},
		cssWorkloadRequiresSWPRegionsValidator{},
		orgFieldsRequireOrganizationIDValidator{},
		targetOUIDsMixValidator{},
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

type connectedSecurityServicesValidator struct{}

func (v connectedSecurityServicesValidator) Description(_ context.Context) string {
	return "workload instance_ids must be exactly one valid UUID when provided"
}

func (v connectedSecurityServicesValidator) MarkdownDescription(_ context.Context) string {
	return "`workload` `instance_ids` must contain exactly one valid UUID when provided"
}

func (v connectedSecurityServicesValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data CAMConnectorResourceModel

	diags :=
		req.Config.Get(ctx, &data)

	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}
	if data.ConnectedSecurityServices.IsNull() || data.ConnectedSecurityServices.IsUnknown() {
		return
	}
	var services []ConnectedSecurityServiceInputModel

	diags =
		data.ConnectedSecurityServices.
			ElementsAs(ctx, &services, false)

	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	for _, svc := range services {
		if svc.Name.ValueString() != "workload" {
			continue
		}

		if svc.InstanceIDs.IsNull() || svc.InstanceIDs.IsUnknown() {
			continue
		}

		var ids []string

		diags = svc.InstanceIDs.ElementsAs(ctx, &ids, false)
		resp.Diagnostics.Append(diags...)

		if len(ids) != 1 {
			resp.Diagnostics.AddError(
				"Invalid instance_ids",
				"workload service requires exactly one instance_id",
			)
			return
		}

		if _, err := uuid.Parse(ids[0]); err != nil {
			resp.Diagnostics.AddError(
				"Invalid instance_id",
				fmt.Sprintf("%s is not a valid UUID", ids[0]),
			)
			return
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

	features, featureDiags := extractAWSFeatures(ctx, plan.Features)
	resp.Diagnostics.Append(featureDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	connectedServices, cssDiags := extractConnectedSecurityServices(ctx, plan.ConnectedSecurityServices)
	resp.Diagnostics.Append(cssDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudAccountID := plan.CloudAccountID.ValueString()
	roleArn := plan.RoleArn.ValueString()
	swpRegions := extractStringList(ctx, plan.ServerWorkloadProtectionRegions, &resp.Diagnostics)
	customTags := extractCustomTags(ctx, plan.CustomTags, &resp.Diagnostics)
	orgExcluded := extractStringList(ctx, plan.OrganizationExcludedAccounts, &resp.Diagnostics)
	targetOUIDs := extractStringList(ctx, plan.TargetOrganizationalUnitIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, readErr := r.client.ReadCloudAccount(cloudAccountID, true)
	if readErr != nil && !strings.Contains(readErr.Error(), "NotFound") {
		resp.Diagnostics.AddError(
			"[CAM Connector][Create] Error Checking Existing Account",
			fmt.Sprintf("Failed to check for existing account: %s", readErr),
		)
		return
	}
	isBridgeAccount := existing != nil && len(existing.Sources) > 0 && existing.RoleArn == ""

	if existing == nil || isBridgeAccount {
		if isBridgeAccount {
			tflog.Info(ctx, fmt.Sprintf("[CAM Connector][Create] Account %s is a bridge/legacy account (sources=%v), re-registering as common connector", cloudAccountID, existing.Sources))
		}
		postBody := &api.CreateCloudAccountRequest{
			RoleArn:                roleArn,
			Name:                   plan.Name.ValueString(),
			Description:            plan.Description.ValueString(),
			Features:               features,
			FeaturesConfigFilePath: plan.FeaturesConfigFilePath.ValueString(),

			OrganizationExcludedAccounts: orgExcluded,
			TargetOrganizationalUnitIDs:  targetOUIDs,
			CustomTags:                   customTags,
		}
		if len(connectedServices) > 0 {
			postBody.ConnectedSecurityServices = connectedServices
		}
		if len(swpRegions) > 0 {
			postBody.ServerWorkloadProtectionRegions = swpRegions
		}
		setBoolPtr(&postBody.IsCremEnabled, plan.IsCremEnabled)
		setBoolPtr(&postBody.IsTFProviderDeployed, plan.IsTFProviderDeployed)
		setBoolPtr(&postBody.IsAwsOrgMgmtAccount, plan.IsAwsOrgMgmtAccount)
		if _, err := r.client.CreateCloudAccount(ctx, plan.OrganizationID.ValueString(), postBody); err != nil {
			resp.Diagnostics.AddError(
				"[CAM Connector][Create] Error Adding AWS Account",
				fmt.Sprintf("[CAM Connector][Create] Failed to add AWS account: %s", err),
			)
			return
		}
	} else {
		// org fields (organization_id / is_aws_org_mgmt_account) are only accepted via the
		// tmv1-organizationID header on POST — the PATCH endpoint ignores that header entirely.
		// Re-registration (DELETE then POST) is required to apply them on an existing connector.
		// prevent_destroy is intentionally ignored here: this is an explicit apply, not a destroy.
		orgIDSet := !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() && plan.OrganizationID.ValueString() != ""
		orgMgmtSet := !plan.IsAwsOrgMgmtAccount.IsNull() && !plan.IsAwsOrgMgmtAccount.IsUnknown() && plan.IsAwsOrgMgmtAccount.ValueBool()
		if orgIDSet || orgMgmtSet {
			tflog.Info(ctx, fmt.Sprintf("[CAM Connector][Create] Account %s already registered; org fields require re-registration (DELETE + POST)", cloudAccountID))
			if err := r.client.DeleteCloudAccounts(cloudAccountID); err != nil {
				resp.Diagnostics.AddError(
					"[CAM Connector][Create] Error Re-registering AWS Account",
					fmt.Sprintf("[CAM Connector][Create] Failed to delete existing account for re-registration: %s", err),
				)
				return
			}
			postBody := &api.CreateCloudAccountRequest{
				RoleArn:                roleArn,
				Name:                   plan.Name.ValueString(),
				Description:            plan.Description.ValueString(),
				Features:               features,
				FeaturesConfigFilePath: plan.FeaturesConfigFilePath.ValueString(),

				OrganizationExcludedAccounts: orgExcluded,
				TargetOrganizationalUnitIDs:  targetOUIDs,
				CustomTags:                   customTags,
			}
			if len(connectedServices) > 0 {
				postBody.ConnectedSecurityServices = connectedServices
			}
			if len(swpRegions) > 0 {
				postBody.ServerWorkloadProtectionRegions = swpRegions
			}
			setBoolPtr(&postBody.IsCremEnabled, plan.IsCremEnabled)
			setBoolPtr(&postBody.IsTFProviderDeployed, plan.IsTFProviderDeployed)
			setBoolPtr(&postBody.IsAwsOrgMgmtAccount, plan.IsAwsOrgMgmtAccount)
			if _, err := r.client.CreateCloudAccount(ctx, plan.OrganizationID.ValueString(), postBody); err != nil {
				resp.Diagnostics.AddError(
					"[CAM Connector][Create] Error Re-registering AWS Account",
					fmt.Sprintf("[CAM Connector][Create] Failed to re-register account after deletion: %s", err),
				)
				return
			}
		} else {
			tflog.Info(ctx, fmt.Sprintf("[CAM Connector][Create] Account %s already exists as common connector, updating instead", cloudAccountID))
			updateBody := &api.ModifyCloudAccountRequest{
				RoleArn:                &roleArn,
				Features:               features,
				FeaturesConfigFilePath: plan.FeaturesConfigFilePath.ValueString(),

				OrganizationExcludedAccounts: orgExcluded,
				TargetOrganizationalUnitIDs:  targetOUIDs,
				CustomTags:                   customTags,
			}
			setStringPtr(&updateBody.Name, plan.Name)
			setStringPtr(&updateBody.Description, plan.Description)
			if len(connectedServices) > 0 {
				updateBody.ConnectedSecurityServices = connectedServices
			}
			if len(swpRegions) > 0 {
				updateBody.ServerWorkloadProtectionRegions = &swpRegions
			}
			setBoolPtr(&updateBody.IsCremEnabled, plan.IsCremEnabled)
			setBoolPtr(&updateBody.IsTFProviderDeployed, plan.IsTFProviderDeployed)
			setBoolPtr(&updateBody.IsAwsOrgMgmtAccount, plan.IsAwsOrgMgmtAccount)
			if err := r.client.UpdateCloudAccounts(cloudAccountID, plan.OrganizationID.ValueString(), updateBody); err != nil {
				resp.Diagnostics.AddError(
					"[CAM Connector][Create] Error Updating Existing AWS Account",
					fmt.Sprintf("[CAM Connector][Create] Failed to update existing account: %s", err),
				)
				return
			}
		}
	}

	res, err := r.client.ReadCloudAccount(cloudAccountID, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Create] Error Reading AWS Account",
			fmt.Sprintf("[CAM Connector][Create] Failed to read AWS account: %s", err),
		)
		return
	}

	plan.ID = types.StringValue(cloudAccountID)
	plan.UpdatedDateTime = types.StringValue("")
	if res != nil {
		plan.State = types.StringValue(res.State)
		plan.CreatedDateTime = types.StringValue(res.CreatedTime)
		updatedTime := res.UpdatedTime
		if updatedTime == "" {
			updatedTime = res.CreatedTime
		}
		plan.UpdatedDateTime = types.StringValue(updatedTime)
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

	res, err := r.client.ReadCloudAccount(state.CloudAccountID.ValueString(), true)
	if err != nil {
		// 401/403/500 are NOT deletion — surface the error
		if strings.Contains(err.Error(), "NotFound") {
			tflog.Info(ctx, "[CAM Connector][Read] Account not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"[CAM Connector][Read] Error Reading AWS Account",
			fmt.Sprintf("[CAM Connector][Read] Failed to read account: %s", err),
		)
		return
	}

	if res != nil {
		state.ID = types.StringValue(state.CloudAccountID.ValueString())
		state.State = types.StringValue(res.State)
		state.CreatedDateTime = types.StringValue(res.CreatedTime)
		state.UpdatedDateTime = types.StringValue(res.UpdatedTime)
		if res.Name != "" {
			state.Name = types.StringValue(res.Name)
		}
		if res.Description != "" {
			state.Description = types.StringValue(res.Description)
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *CAMConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state CAMConnectorResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[CAM Connector][Update] Cluster plan: %+v", plan))

	roleArn := plan.RoleArn.ValueString()

	features, featureDiags := extractAWSFeatures(ctx, plan.Features)
	resp.Diagnostics.Append(featureDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := validateCSSUpdate(
		ctx,
		state.ConnectedSecurityServices,
		plan.ConnectedSecurityServices,
	); err != nil {
		resp.Diagnostics.AddError(
			"Invalid connected_security_services update",
			err.Error(),
		)

		return
	}
	connectedServices, cssDiags := extractConnectedSecurityServices(ctx, plan.ConnectedSecurityServices)
	resp.Diagnostics.Append(cssDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	swpRegions := extractStringList(ctx, plan.ServerWorkloadProtectionRegions, &resp.Diagnostics)
	if len(swpRegions) == 0 && len(connectedServices) > 0 {
		swpRegions = extractStringList(ctx, state.ServerWorkloadProtectionRegions, &resp.Diagnostics)
	}
	customTags := extractCustomTags(ctx, plan.CustomTags, &resp.Diagnostics)
	orgExcluded := extractStringList(ctx, plan.OrganizationExcludedAccounts, &resp.Diagnostics)
	targetOUIDs := extractStringList(ctx, plan.TargetOrganizationalUnitIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	body := &api.ModifyCloudAccountRequest{
		RoleArn:                &roleArn,
		Features:               features,
		FeaturesConfigFilePath: plan.FeaturesConfigFilePath.ValueString(),

		OrganizationExcludedAccounts: orgExcluded,
		TargetOrganizationalUnitIDs:  targetOUIDs,
		CustomTags:                   customTags,
	}
	setStringPtr(&body.Name, plan.Name)
	setStringPtr(&body.Description, plan.Description)
	if len(connectedServices) > 0 {
		body.ConnectedSecurityServices = connectedServices
	}
	if len(swpRegions) > 0 {
		body.ServerWorkloadProtectionRegions = &swpRegions
	}
	setBoolPtr(&body.IsCremEnabled, plan.IsCremEnabled)
	setBoolPtr(&body.IsTFProviderDeployed, plan.IsTFProviderDeployed)
	setBoolPtr(&body.IsAwsOrgMgmtAccount, plan.IsAwsOrgMgmtAccount)

	cloudAccountID := state.CloudAccountID.ValueString()

	err := r.client.UpdateCloudAccounts(cloudAccountID, plan.OrganizationID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Update] Error Updating AWS Account",
			fmt.Sprintf("[CAM Connector][Update] Failed to update account: %s", err),
		)
		return
	}

	res, err := r.client.ReadCloudAccount(cloudAccountID, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Update] Error Describing AWS Account",
			fmt.Sprintf("[CAM Connector][Update] Failed to describe account: %s", err),
		)
		return
	}

	if res != nil {
		state.ID = types.StringValue(state.CloudAccountID.ValueString())
		state.State = types.StringValue(res.State)
		state.CreatedDateTime = types.StringValue(res.CreatedTime)
		state.UpdatedDateTime = types.StringValue(res.UpdatedTime)
		state.Name = plan.Name
		state.Description = plan.Description
		state.Features = plan.Features
		state.FeaturesConfigFilePath = plan.FeaturesConfigFilePath
		state.ConnectedSecurityServices = plan.ConnectedSecurityServices
		state.ServerWorkloadProtectionRegions = plan.ServerWorkloadProtectionRegions
		state.CamDeployedRegion = plan.CamDeployedRegion
		state.PreventDestroy = plan.PreventDestroy
		state.IsCremEnabled = plan.IsCremEnabled
		state.IsTFProviderDeployed = plan.IsTFProviderDeployed
		state.IsAwsOrgMgmtAccount = plan.IsAwsOrgMgmtAccount
		state.OrganizationID = plan.OrganizationID
		state.OrganizationExcludedAccounts = plan.OrganizationExcludedAccounts
		state.TargetOrganizationalUnitIDs = plan.TargetOrganizationalUnitIDs
		state.CustomTags = plan.CustomTags
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *CAMConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CAMConnectorResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.PreventDestroy.IsNull() || state.PreventDestroy.IsUnknown() || state.PreventDestroy.ValueBool() {
		tflog.Info(ctx, fmt.Sprintf("[CAM Connector][Delete] prevent_destroy=true (or unset), skipping CAM DELETE for account %s", state.CloudAccountID.ValueString()))
		return
	}

	err := r.client.DeleteCloudAccounts(state.CloudAccountID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[CAM Connector][Delete] Error Removing Subscription",
			fmt.Sprintf("[CAM Connector][Delete] Failed to remove subscription: %s", err),
		)
		return
	}
}

func extractAWSFeatures(ctx context.Context, featuresList types.List) ([]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	// null means the user did not specify features — omit the field entirely from the request.
	if featuresList.IsNull() || featuresList.IsUnknown() {
		return nil, diags
	}

	var featureModels []AWSFeatureModel
	extractDiags := featuresList.ElementsAs(ctx, &featureModels, false)
	diags.Append(extractDiags...)
	if diags.HasError() {
		return nil, diags
	}

	// Empty list means the user explicitly cleared all features — send [] to the backend.
	features := make([]interface{}, 0, len(featureModels))
	for _, model := range featureModels {
		var regions []string
		if !model.Regions.IsNull() && !model.Regions.IsUnknown() {
			regionDiags := model.Regions.ElementsAs(ctx, &regions, false)
			diags.Append(regionDiags...)
			if diags.HasError() {
				return nil, diags
			}
		}
		entry := map[string]interface{}{"id": model.ID.ValueString()}
		if len(regions) > 0 {
			entry["regions"] = regions
		}
		features = append(features, entry)
	}

	return features, diags
}

type ConnectedSecurityServiceInputModel struct {
	Name        types.String `tfsdk:"name"`
	InstanceIDs types.List   `tfsdk:"instance_ids"`
	Regions     types.List   `tfsdk:"regions"`
}

func extractConnectedSecurityServices(ctx context.Context, list types.List) ([]api.SecurityService, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var models []ConnectedSecurityServiceInputModel
	diags.Append(list.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}

	services := make([]api.SecurityService, 0, len(models))
	for _, m := range models {
		svc := api.SecurityService{Name: m.Name.ValueString()}
		if !m.InstanceIDs.IsNull() && !m.InstanceIDs.IsUnknown() {
			diags.Append(m.InstanceIDs.ElementsAs(ctx, &svc.InstanceIDs, false)...)
		}
		if !m.Regions.IsNull() && !m.Regions.IsUnknown() {
			diags.Append(m.Regions.ElementsAs(ctx, &svc.Regions, false)...)
		}
		if diags.HasError() {
			return nil, diags
		}
		services = append(services, svc)
	}
	return services, diags
}

// setBoolPtr sets a *bool field on a request struct from a types.Bool plan value.
func setBoolPtr(target **bool, v types.Bool) {
	if !v.IsNull() && !v.IsUnknown() {
		val := v.ValueBool()
		*target = &val
	}
}

// setStringPtr sets a *string field only when the value is known and non-empty,
// preventing omitempty from being bypassed by a non-nil pointer to "".
func setStringPtr(target **string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		val := v.ValueString()
		*target = &val
	}
}

// extractCustomTags converts a types.Map (string→string) into []cam.CustomTag.
func extractCustomTags(ctx context.Context, m types.Map, diags *diag.Diagnostics) []cam.CustomTag {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	var raw map[string]string
	diags.Append(m.ElementsAs(ctx, &raw, false)...)
	if diags.HasError() {
		return nil
	}
	tags := make([]cam.CustomTag, 0, len(raw))
	for k, v := range raw {
		tags = append(tags, cam.CustomTag{Key: k, Value: v})
	}
	return tags
}

// extractStringList converts a types.List of strings into []string.
func extractStringList(ctx context.Context, list types.List, diags *diag.Diagnostics) []string {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var result []string
	diags.Append(list.ElementsAs(ctx, &result, false)...)
	return result
}

var supportedSWPRegions = map[string]struct{}{
	"us-east-2":      {},
	"us-east-1":      {},
	"us-west-1":      {},
	"us-west-2":      {},
	"af-south-1":     {},
	"ap-east-1":      {},
	"ap-south-2":     {},
	"ap-southeast-3": {},
	"ap-southeast-5": {},
	"ap-southeast-4": {},
	"ap-south-1":     {},
	"ap-southeast-6": {},
	"ap-northeast-3": {},
	"ap-northeast-2": {},
	"ap-southeast-1": {},
	"ap-southeast-2": {},
	"ap-east-2":      {},
	"ap-southeast-7": {},
	"ap-northeast-1": {},
	"ca-central-1":   {},
	"ca-west-1":      {},
	"eu-central-1":   {},
	"eu-west-1":      {},
	"eu-west-2":      {},
	"eu-south-1":     {},
	"eu-west-3":      {},
	"eu-south-2":     {},
	"eu-north-1":     {},
	"eu-central-2":   {},
	"il-central-1":   {},
	"mx-central-1":   {},
	"me-south-1":     {},
	"me-central-1":   {},
	"sa-east-1":      {},
}

type cssWorkloadRequiresSWPRegionsValidator struct{}

func (v cssWorkloadRequiresSWPRegionsValidator) Description(_ context.Context) string {
	return "server_workload_protection_regions is required when connected_security_services contains a workload entry with instance_ids"
}

func (v cssWorkloadRequiresSWPRegionsValidator) MarkdownDescription(_ context.Context) string {
	return "`server_workload_protection_regions` is required when `connected_security_services` contains a `workload` entry with `instance_ids`"
}

func (v cssWorkloadRequiresSWPRegionsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data CAMConnectorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ConnectedSecurityServices.IsNull() || data.ConnectedSecurityServices.IsUnknown() {
		return
	}

	var services []ConnectedSecurityServiceInputModel
	resp.Diagnostics.Append(data.ConnectedSecurityServices.ElementsAs(ctx, &services, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, svc := range services {
		if svc.Name.ValueString() != "workload" {
			continue
		}
		if svc.InstanceIDs.IsNull() || svc.InstanceIDs.IsUnknown() {
			continue
		}
		var ids []string
		resp.Diagnostics.Append(svc.InstanceIDs.ElementsAs(ctx, &ids, false)...)
		if len(ids) == 0 {
			continue
		}
		if data.ServerWorkloadProtectionRegions.IsNull() || data.ServerWorkloadProtectionRegions.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("server_workload_protection_regions"),
				"Missing Required Attribute",
				"server_workload_protection_regions must be set when connected_security_services contains a workload entry with instance_ids.",
			)
			return
		}
		var regions []string
		resp.Diagnostics.Append(data.ServerWorkloadProtectionRegions.ElementsAs(ctx, &regions, false)...)
		if len(regions) == 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("server_workload_protection_regions"),
				"Missing Required Attribute",
				"server_workload_protection_regions must be non-empty when connected_security_services contains a workload entry with instance_ids.",
			)
			return
		}
		for _, r := range regions {
			if _, ok := supportedSWPRegions[r]; !ok {
				resp.Diagnostics.AddAttributeError(
					path.Root("server_workload_protection_regions"),
					"Unsupported SWP Region",
					fmt.Sprintf("%q is not a supported Server & Workload Protection region.", r),
				)
			}
		}
	}
}

type orgFieldsRequireOrganizationIDValidator struct{}

func (v orgFieldsRequireOrganizationIDValidator) Description(_ context.Context) string {
	return "is_aws_org_mgmt_account, organization_excluded_accounts, and target_organizational_unit_ids require organization_id"
}

func (v orgFieldsRequireOrganizationIDValidator) MarkdownDescription(_ context.Context) string {
	return "`is_aws_org_mgmt_account`, `organization_excluded_accounts`, and `target_organizational_unit_ids` require `organization_id` to be set"
}

func (v orgFieldsRequireOrganizationIDValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data CAMConnectorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgIDSet := !data.OrganizationID.IsNull() && !data.OrganizationID.IsUnknown() && data.OrganizationID.ValueString() != ""

	if !data.IsAwsOrgMgmtAccount.IsNull() && !data.IsAwsOrgMgmtAccount.IsUnknown() && !orgIDSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("is_aws_org_mgmt_account"),
			"Missing Required Attribute",
			"is_aws_org_mgmt_account requires organization_id to be set.",
		)
	}

	if !data.OrganizationExcludedAccounts.IsNull() && !data.OrganizationExcludedAccounts.IsUnknown() && !orgIDSet {
		var accounts []string
		resp.Diagnostics.Append(data.OrganizationExcludedAccounts.ElementsAs(ctx, &accounts, false)...)
		if len(accounts) > 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("organization_excluded_accounts"),
				"Missing Required Attribute",
				"organization_excluded_accounts requires organization_id to be set.",
			)
		}
	}

	if !data.TargetOrganizationalUnitIDs.IsNull() && !data.TargetOrganizationalUnitIDs.IsUnknown() {
		var ouIDs []string
		resp.Diagnostics.Append(data.TargetOrganizationalUnitIDs.ElementsAs(ctx, &ouIDs, false)...)
		if len(ouIDs) > 0 {
			if !orgIDSet {
				resp.Diagnostics.AddAttributeError(
					path.Root("target_organizational_unit_ids"),
					"Missing Required Attribute",
					"target_organizational_unit_ids requires organization_id to be set.",
				)
			}
			if data.IsAwsOrgMgmtAccount.IsNull() || data.IsAwsOrgMgmtAccount.IsUnknown() || !data.IsAwsOrgMgmtAccount.ValueBool() {
				resp.Diagnostics.AddAttributeError(
					path.Root("target_organizational_unit_ids"),
					"Invalid Attribute Combination",
					"target_organizational_unit_ids can only be set on the AWS Organization management account (is_aws_org_mgmt_account = true).",
				)
			}
		}
	}
}

type targetOUIDsMixValidator struct{}

func (v targetOUIDsMixValidator) Description(_ context.Context) string {
	return "target_organizational_unit_ids cannot mix a root (r-) with organizational units (ou-)"
}

func (v targetOUIDsMixValidator) MarkdownDescription(_ context.Context) string {
	return "`target_organizational_unit_ids` cannot mix a root (`r-`) with organizational units (`ou-`) — root already covers all OUs"
}

func (v targetOUIDsMixValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data CAMConnectorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.TargetOrganizationalUnitIDs.IsNull() || data.TargetOrganizationalUnitIDs.IsUnknown() {
		return
	}
	var ids []string
	resp.Diagnostics.Append(data.TargetOrganizationalUnitIDs.ElementsAs(ctx, &ids, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hasRoot, hasOU := false, false
	for _, id := range ids {
		if strings.HasPrefix(id, "r-") {
			hasRoot = true
		} else {
			hasOU = true
		}
	}
	if hasRoot && hasOU {
		resp.Diagnostics.AddAttributeError(
			path.Root("target_organizational_unit_ids"),
			"Invalid Attribute Combination",
			"cannot mix a root (r-xxxx) with organizational units (ou-xxxx) — root already covers all OUs",
		)
	}
}

func validateCSSUpdate(
	ctx context.Context,
	state types.List,
	plan types.List,
) error {
	oldServices, diags :=
		extractConnectedSecurityServices(ctx, state)

	if diags.HasError() {
		return fmt.Errorf("failed to parse existing connected_security_services")
	}

	newServices, diags :=
		extractConnectedSecurityServices(ctx, plan)

	if diags.HasError() {
		return fmt.Errorf("failed to parse planned connected_security_services")
	}

	if len(newServices) != len(oldServices) {
		return fmt.Errorf(
			"cannot add or remove connected_security_services entries; only regions may be modified",
		)
	}

	for i := range oldServices {
		if oldServices[i].Name != newServices[i].Name {
			return fmt.Errorf("service name is immutable")
		}

		if !reflect.DeepEqual(oldServices[i].InstanceIDs, newServices[i].InstanceIDs) {
			return fmt.Errorf("instance_ids is immutable; only regions may be modified")
		}
	}

	return nil
}
