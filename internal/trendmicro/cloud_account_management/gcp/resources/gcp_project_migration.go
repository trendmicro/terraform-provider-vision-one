package resources

import (
	"context"
	"fmt"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	migrationStatusMigrated = "migrated"
	migrationStatusFailed   = "failed"
)

var (
	_ resource.Resource              = &GCPProjectMigrationResource{}
	_ resource.ResourceWithConfigure = &GCPProjectMigrationResource{}
)

func NewGCPProjectMigrationResource() resource.Resource {
	return &GCPProjectMigrationResource{}
}

type GCPProjectMigrationResource struct {
	client *api.CamClient
}

type gcpProjectMigrationModel struct {
	ID                   types.String `tfsdk:"id"`
	ProjectNumber        types.String `tfsdk:"project_number"`
	Name                 types.String `tfsdk:"name"`
	NewServiceAccountID  types.String `tfsdk:"new_service_account_id"`
	NewServiceAccountKey types.String `tfsdk:"new_service_account_key"`
	MigratedAt           types.String `tfsdk:"migrated_at"`
	MigrationStatus      types.String `tfsdk:"migration_status"`
	MigrationError       types.String `tfsdk:"migration_error"`
}

func (r *GCPProjectMigrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_GCP_PROJECT_MIGRATION
}

func (r *GCPProjectMigrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Migrates an existing GCP project record in the CAM database from the legacy Terraform Package Solution to the new Terraform Provider Solution. " +
			"This updates the service account key information in the DB record so that downstream apps remain connected. " +
			"Migration is a one-way operation: Update and Delete are no-ops.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_number": schema.StringAttribute{
				MarkdownDescription: "The GCP project number identifying the existing CAM record to migrate.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name for the connector (preserved during migration).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"new_service_account_id": schema.StringAttribute{
				MarkdownDescription: "The unique ID of the new service account created by the Terraform Provider Solution.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"new_service_account_key": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded JSON service account key for the new Terraform Provider Solution service account.",
				Required:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"migrated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the migration was performed.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"migration_status": schema.StringAttribute{
				MarkdownDescription: "Status of the migration: `migrated` or `failed`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"migration_error": schema.StringAttribute{
				MarkdownDescription: "Error message if the migration failed.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *GCPProjectMigrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = &api.CamClient{
		Client: client.WithTimeout(cam.CAMAPITimeout),
	}
}

func (r *GCPProjectMigrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gcpProjectMigrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectNumber := plan.ProjectNumber.ValueString()
	plan.ID = types.StringValue(projectNumber)
	plan.MigrationError = types.StringValue("")

	serviceAccountEmail, err := waitForGCPServiceAccountIAMReady(ctx, plan.NewServiceAccountKey.ValueString(), plan.ProjectNumber.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Project Migration] Service Account IAM Not Ready",
			fmt.Sprintf("The service account key is valid but its IAM bindings have not yet propagated in GCP. Error: %s", err.Error()),
		)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf(
		"[GCP Project Migration] Serializing CAM migration for project %s with service account email %s",
		projectNumber,
		serviceAccountEmail,
	))

	unlock := lockGCPCAMProjectMutation(projectNumber)
	defer unlock()

	// Read existing project to preserve fields that must not be overwritten (e.g. IsCAMCloudASRMEnabled).
	// UpdateProject returns a clear "code": "NotFound" error if the project is not registered.
	existing, readExistingErr := r.client.ReadProject(projectNumber)
	if readExistingErr != nil {
		if strings.Contains(readExistingErr.Error(), `"code": "NotFound"`) {
			resp.Diagnostics.AddError(
				"[GCP Project Migration] Project Not Found",
				fmt.Sprintf("project %s not found in CAM database; ensure the project was registered via the legacy Terraform Package Solution", projectNumber),
			)
		} else {
			resp.Diagnostics.AddError(
				"[GCP Project Migration] Failed to Read Existing Project",
				fmt.Sprintf("failed to read existing project before migration: %s", readExistingErr),
			)
		}
		return
	}

	emptyString := ""
	updateReq := buildGCPProjectMigrationUpdateRequest(plan, existing, &emptyString)

	err = r.client.UpdateProject(projectNumber, updateReq)
	if err != nil {
		errMsg := fmt.Sprintf("failed to update project with new service account: %s", err)
		if strings.Contains(err.Error(), `"code": "NotFound"`) {
			errMsg = fmt.Sprintf("project %s not found in CAM database; ensure the project was registered via the legacy Terraform Package Solution", projectNumber)
		}
		plan.MigratedAt = types.StringValue("")
		plan.MigrationStatus = types.StringValue(migrationStatusFailed)
		plan.MigrationError = types.StringValue(errMsg)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	// Wait for CAM backend to verify the new service account and reach "connected" state.
	_, connectErr := waitForGCPProjectConnected(ctx, r.client, projectNumber)
	if connectErr != nil {
		plan.MigratedAt = types.StringValue("")
		plan.MigrationStatus = types.StringValue(migrationStatusFailed)
		plan.MigrationError = types.StringValue(fmt.Sprintf("PATCH succeeded but project did not reach connected state: %s", connectErr))
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	plan.MigratedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))
	plan.MigrationStatus = types.StringValue(migrationStatusMigrated)
	tflog.Info(ctx, fmt.Sprintf("[GCP Project Migration] Successfully migrated project %s to new service account %s",
		projectNumber, plan.NewServiceAccountID.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GCPProjectMigrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Migration is one-way: preserve state as-is on every refresh.
	var state gcpProjectMigrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *GCPProjectMigrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state gcpProjectMigrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *GCPProjectMigrationResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op: migration is one-way, removing from state only
}

func buildGCPProjectMigrationUpdateRequest(
	plan gcpProjectMigrationModel,
	existing *api.ProjectResponse,
	workloadIdentityPoolID *string,
) *api.ModifyProjectRequest {
	return &api.ModifyProjectRequest{
		CamDeployedRegion:         existing.CamDeployedRegion,
		ConnectedSecurityServices: existing.ConnectedSecurityServices,
		Description:               existing.Description,
		Folder:                    existing.Folder,
		IsCAMCloudASRMEnabled:     existing.IsCAMCloudASRMEnabled,
		IsTFProviderDeployed:      true,
		Name:                      plan.Name.ValueString(),
		Organization:              existing.Organization,
		ProjectNumber:             plan.ProjectNumber.ValueString(),
		ServiceAccountId:          plan.NewServiceAccountID.ValueString(),
		ServiceAccountKey:         plan.NewServiceAccountKey.ValueString(),
		WorkloadIdentityPoolId:    workloadIdentityPoolID,
	}
}
