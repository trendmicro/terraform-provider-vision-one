package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

var _ resource.Resource = &LegacyCleanupIAMCustomRole{}

type LegacyCleanupIAMCustomRole struct{}

type legacyCleanupIAMCustomRoleModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	ServiceAccountKey types.String `tfsdk:"service_account_key"`
	CustomRoleID      types.String `tfsdk:"custom_role_id"`
	Deleted           types.Bool   `tfsdk:"deleted"`
	RoleName          types.String `tfsdk:"role_name"`
	DeletionTimestamp types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus     types.String `tfsdk:"cleanup_status"`
	CleanupError      types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupIAMCustomRole() resource.Resource {
	return &LegacyCleanupIAMCustomRole{}
}

func (r *LegacyCleanupIAMCustomRole) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_IAM_CUSTOM_ROLE
}

func (r *LegacyCleanupIAMCustomRole) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deletes the legacy IAM custom role (`vision_one_cam_role_*`) created by the Terraform Package Solution. The resource scans all project-level custom roles for the naming prefix and deletes the match. Returns `cleanup_status = \"not_found\"` if no legacy role exists.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID containing the legacy IAM custom role.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_key": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded JSON service account key used to authenticate with GCP.",
				Optional:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"custom_role_id": schema.StringAttribute{
				MarkdownDescription: "Optional full role name of the **new** provider-managed role (e.g. `projects/{proj}/roles/vision_one_cam_role_abc123`) to skip during cleanup. Any matching role with this name will not be deleted. Use this in migration scenarios where both the legacy and new roles share the same prefix.",
				Optional:            true,
			},
			"deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether the custom role was deleted.",
				Computed:            true,
			},
			"role_name": schema.StringAttribute{
				MarkdownDescription: "Full resource name of the detected role.",
				Computed:            true,
			},
			"deletion_timestamp": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the cleanup was performed.",
				Computed:            true,
			},
			"cleanup_status": schema.StringAttribute{
				MarkdownDescription: "Status: deleted, not_found, or failed.",
				Computed:            true,
			},
			"cleanup_error": schema.StringAttribute{
				MarkdownDescription: "Error message if cleanup failed.",
				Computed:            true,
			},
		},
	}
}

func (r *LegacyCleanupIAMCustomRole) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupIAMCustomRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	plan.ID = types.StringValue(projectID)
	plan.Deleted = types.BoolValue(false)
	plan.RoleName = types.StringValue("")
	plan.DeletionTimestamp = types.StringValue("")
	plan.CleanupError = types.StringValue("")

	clientOptions := []option.ClientOption{}
	if serviceAccountKey := plan.ServiceAccountKey.ValueString(); serviceAccountKey != "" {
		clientOption, err := newClientOptionFromEncodedServiceAccountKey(ctx, serviceAccountKey)
		if err != nil {
			resp.Diagnostics.AddError("[IAM Role Cleanup] Invalid service account key", err.Error())
			return
		}

		clientOptions = append(clientOptions, clientOption)
	}

	iamSvc, err := iam.NewService(ctx, clientOptions...)
	if err != nil {
		resp.Diagnostics.AddError("[IAM Role Cleanup] Failed to create IAM client", err.Error())
		return
	}

	parent := fmt.Sprintf("projects/%s", projectID)
	skipRoleName := plan.CustomRoleID.ValueString()
	var legacyRole *iam.Role

	if err := iamSvc.Projects.Roles.List(parent).View("FULL").Pages(ctx, func(page *iam.ListRolesResponse) error {
		for _, role := range page.Roles {
			parts := strings.Split(role.Name, "/")
			roleID := parts[len(parts)-1]
			if !strings.HasPrefix(roleID, config.LEGACY_GCP_CUSTOM_ROLE_PREFIX) {
				continue
			}
			// Skip the new provider-managed role so we only delete the legacy one.
			if skipRoleName != "" && role.Name == skipRoleName {
				tflog.Info(ctx, fmt.Sprintf("[IAM Role Cleanup] Skipping new provider-managed role: %s", role.Name))
				continue
			}
			legacyRole = role
			return errLegacyResourceFound
		}
		return nil
	}); err != nil && !errors.Is(err, errLegacyResourceFound) {
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(fmt.Sprintf("failed to list roles: %s", err))
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	if legacyRole == nil {
		tflog.Info(ctx, fmt.Sprintf("[IAM Role Cleanup] No legacy role found in project: %s", projectID))
		plan.CleanupStatus = types.StringValue("not_found")
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	plan.RoleName = types.StringValue(legacyRole.Name)

	_, deleteErr := iamSvc.Projects.Roles.Delete(legacyRole.Name).Context(ctx).Do()
	if deleteErr != nil {
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(fmt.Sprintf("failed to delete role: %s", deleteErr))
	} else {
		plan.Deleted = types.BoolValue(true)
		plan.DeletionTimestamp = types.StringValue(time.Now().UTC().Format(time.RFC3339))
		plan.CleanupStatus = types.StringValue("deleted")
		tflog.Info(ctx, fmt.Sprintf("[IAM Role Cleanup] Deleted role: %s", legacyRole.Name))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LegacyCleanupIAMCustomRole) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupIAMCustomRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Deleted.ValueBool() || state.RoleName.ValueString() == "" {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	clientOptions := []option.ClientOption{}
	if serviceAccountKey := state.ServiceAccountKey.ValueString(); serviceAccountKey != "" {
		clientOption, err := newClientOptionFromEncodedServiceAccountKey(ctx, serviceAccountKey)
		if err != nil {
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}

		clientOptions = append(clientOptions, clientOption)
	}

	iamSvc, err := iam.NewService(ctx, clientOptions...)
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	role, err := iamSvc.Projects.Roles.Get(state.RoleName.ValueString()).Context(ctx).Do()
	if err != nil || role.Deleted {
		state.Deleted = types.BoolValue(true)
		state.CleanupStatus = types.StringValue("deleted")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupIAMCustomRole) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state legacyCleanupIAMCustomRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var plan legacyCleanupIAMCustomRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Optional input fields must come from the plan, not prior state.
	state.ServiceAccountKey = plan.ServiceAccountKey
	state.CustomRoleID = plan.CustomRoleID
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupIAMCustomRole) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op
}
