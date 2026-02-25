package resources

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/iam/v1"
)

type IAMCustomRole struct {
	client *api.CamClient
}

type customRoleDefinitionResourceModel struct {
	RoleID             types.String `tfsdk:"role_id"`
	Title              types.String `tfsdk:"title"`
	Description        types.String `tfsdk:"description"`
	Permissions        types.List   `tfsdk:"permissions"`
	FeaturePermissions types.Set    `tfsdk:"feature_permissions"`
	ProjectID          types.String `tfsdk:"project_id"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	Name               types.String `tfsdk:"name"`
	Deleted            types.Bool   `tfsdk:"deleted"`
	Stage              types.String `tfsdk:"stage"`
}

func NewIAMCustomRole() resource.Resource {
	return &IAMCustomRole{
		client: &api.CamClient{},
	}
}

func (r *IAMCustomRole) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_IAM_CUSTOM_ROLE
}

func (r *IAMCustomRole) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Trend Micro Vision One Cloud Account Management GCP Role Definition resource. Creates a custom GCP IAM role with the [necessary permissions](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-gcp-required-granted-permissions) for Trend Micro Vision One Cloud Account Management.",
		Attributes: map[string]schema.Attribute{
			"role_id": schema.StringAttribute{
				MarkdownDescription: "Role ID to use for this custom role. If not specified, a Trend Micro Vision One Cloud Account Management Custom Role ID will be generated.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "Human-readable title for the Trend Micro Vision One Cloud Account Management custom role.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Vision One Cloud Account Management Features role"),
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the Trend Micro Vision One Cloud Account Management custom role definition.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("The custom role for Vision One Cloud Account Management and features"),
			},
			"permissions": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of permissions associated with the Trend Micro Vision One Cloud Account Management custom role definition. **IMPORTANT**: If specified, this list will OVERWRITE (not append to) the default core permissions. If not specified, the role will include the core permissions appropriate for the parent level (organization or project). Organization-level roles include organization, folder, and project permissions, while project-level roles include only project permissions. For detailed permission requirements, refer to the [Permissions API](coming-soon).",
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(cam.ConvertStringSliceToListValue(config.GCP_CUSTOM_ROLE_CORE_PERMISSIONS)), // Once the API is ready, we can remove the default and get the permissions from the API based on the default behavior and features.
			},
			"feature_permissions": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Set of features associated with the Trend Micro Vision One Cloud Account Management custom role definition. When specified, the role will include all permissions required by the specified features in addition to the base permissions (either default core permissions or your custom permissions list). The permissions are automatically retrieved and aggregated according to the Trend Micro Vision One GCP required permissions documentation. For available features, see the [Features API](coming-soon). Example: `[\"cloud-sentry\", \"real-time-posture-monitoring\"]`.",
				Optional:            true,
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The project ID used for GCP authentication and API calls. When creating a project-level custom role, this is where the role will be created. When creating an organization-level custom role (with organization_id), this project is used only for authentication. Required in all cases.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID where the custom role will be created. When specified, creates an organization-level custom role that can be used across all projects in the organization. **Recommended for multi-project deployments** to allow the same custom role to be used across all projects in a folder. When this is set, project_id is still required for GCP authentication.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Full resource name of the Trend Micro Vision One Cloud Account Management custom role definition.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether the Trend Micro Vision One Cloud Account Management custom role has been deleted.",
				Computed:            true,
			},
			"stage": schema.StringAttribute{
				MarkdownDescription: "Current launch stage of the Trend Micro Vision One Cloud Account Management custom role (e.g., ALPHA, BETA, GA, DEPRECATED).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *IAMCustomRole) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customRoleDefinitionResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Determine parent type and construct parent string
	var parent string
	var parentType string // "project" or "organization"
	var gcpClients *api.GCPClients
	var diags diag.Diagnostics
	var projectID, orgID string

	// Get project ID for authentication (required even for organization-level roles)
	projectID = plan.ProjectID.ValueString()

	// Check which parent is specified
	if !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() {
		// Organization-level role
		orgID = plan.OrganizationID.ValueString()
		parentType = config.PARENT_TYPE_ORGANIZATION
		parent = fmt.Sprintf("organizations/%s", orgID)

		// Use project for GCP authentication
		if projectID == "" {
			resp.Diagnostics.AddError(
				"[GCP Role Definition][Create] Invalid configuration",
				"project_id is required for GCP authentication, even when creating organization-level custom roles",
			)
			return
		}

		gcpClients, diags = api.GetGCPClients(ctx, projectID)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		projectID = gcpClients.ProjectID

		tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Create] Creating organization-level custom role in: %s (using project %s for authentication)", parent, projectID))
	} else {
		// Project-level role (default)
		if projectID == "" {
			resp.Diagnostics.AddError(
				"[GCP Role Definition][Create] Invalid configuration",
				"project_id must be specified",
			)
			return
		}

		parentType = "project"
		gcpClients, diags = api.GetGCPClients(ctx, projectID)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		projectID = gcpClients.ProjectID
		parent = fmt.Sprintf("projects/%s", projectID)

		tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Create] Creating project-level custom role in: %s", parent))
	}

	// Generate role ID if not provided
	roleID := plan.RoleID.ValueString()
	if roleID == "" {
		roleID = config.GCP_CUSTOM_ROLE_NAME + cam.GenerateRandomString(4)
	}

	// Extract permissions from plan, or use default core permissions if not provided
	var permissions []string
	if !plan.Permissions.IsNull() && !plan.Permissions.IsUnknown() {
		diags = plan.Permissions.ElementsAs(ctx, &permissions, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Create] Using user-provided permissions count: %d", len(permissions)))
	} else {
		// Use default core permissions if user didn't provide any (make a copy)
		permissions = make([]string, len(config.GCP_CUSTOM_ROLE_CORE_PERMISSIONS))
		copy(permissions, config.GCP_CUSTOM_ROLE_CORE_PERMISSIONS)
		tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Create] Using default core permissions count: %d", len(permissions)))
	}

	var featurePermissions []string
	if !plan.FeaturePermissions.IsNull() && !plan.FeaturePermissions.IsUnknown() {
		diags = plan.FeaturePermissions.ElementsAs(ctx, &featurePermissions, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
	}

	aggregatedPermissions, err := r.aggregatePermissions(ctx, permissions, featurePermissions)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Role Definition][Create] Failed to aggregate permissions",
			fmt.Sprintf("Error aggregating permissions: %s", err.Error()),
		)
		return
	}
	permissions = aggregatedPermissions

	stage := plan.Stage.ValueString()
	if stage == "" {
		stage = "GA"
	}

	role := &iam.Role{
		Title:               plan.Title.ValueString(),
		Description:         plan.Description.ValueString(),
		IncludedPermissions: permissions,
		Stage:               stage,
	}

	createRoleReq := &iam.CreateRoleRequest{
		RoleId: roleID,
		Role:   role,
	}

	var createdRole *iam.Role

	// Call the appropriate API based on parent type
	if parentType == config.PARENT_TYPE_ORGANIZATION {
		createdRole, err = gcpClients.IAMClient.Organizations.Roles.Create(parent, createRoleReq).Context(ctx).Do()
		if err != nil {
			resp.Diagnostics.AddError(
				"[GCP Role Definition][Create] Failed to create organization-level custom role",
				fmt.Sprintf("Error creating custom role: %s", err.Error()),
			)
			return
		}
	} else {
		createdRole, err = gcpClients.IAMClient.Projects.Roles.Create(parent, createRoleReq).Context(ctx).Do()
		if err != nil {
			resp.Diagnostics.AddError(
				"[GCP Role Definition][Create] Failed to create project-level custom role",
				fmt.Sprintf("Error creating custom role: %s", err.Error()),
			)
			return
		}
	}

	tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Create] Created role: %s", createdRole.Name))

	roleName := createdRole.Name
	if roleName == "" {
		if parentType == config.PARENT_TYPE_ORGANIZATION {
			roleName = fmt.Sprintf("organizations/%s/roles/%s", orgID, roleID)
		} else {
			roleName = fmt.Sprintf("projects/%s/roles/%s", projectID, roleID)
		}
		tflog.Warn(ctx, fmt.Sprintf("[GCP Role Definition][Create] API did not return name, constructed: %s", roleName))
	}

	plan.Name = types.StringValue(roleName)
	plan.RoleID = types.StringValue(roleID)

	if parentType == config.PARENT_TYPE_ORGANIZATION {
		plan.OrganizationID = types.StringValue(orgID)
		plan.ProjectID = types.StringValue(projectID) // Keep project ID for authentication
	} else {
		plan.ProjectID = types.StringValue(projectID)
		// Set organization_id to null for project-level roles
		plan.OrganizationID = types.StringNull()
	}
	plan.Title = types.StringValue(createdRole.Title)
	plan.Description = types.StringValue(createdRole.Description)
	plan.Deleted = types.BoolValue(createdRole.Deleted)
	plan.Stage = types.StringValue(createdRole.Stage)

	permissionsList, diags := types.ListValueFrom(ctx, types.StringType, permissions)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan.Permissions = permissionsList

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *IAMCustomRole) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customRoleDefinitionResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	roleName := state.Name.ValueString()

	var gcpClients *api.GCPClients
	var diags diag.Diagnostics

	// Determine if this is an organization or project-level role
	isOrgRole := strings.HasPrefix(roleName, "organizations/")

	if isOrgRole {
		// For organization roles, we can use empty project ID
		gcpClients, diags = api.GetGCPClients(ctx, "")
	} else {
		projectID := state.ProjectID.ValueString()
		gcpClients, diags = api.GetGCPClients(ctx, projectID)
	}

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Get the role
	var role *iam.Role
	var err error

	if isOrgRole {
		role, err = gcpClients.IAMClient.Organizations.Roles.Get(roleName).Context(ctx).Do()
	} else {
		role, err = gcpClients.IAMClient.Projects.Roles.Get(roleName).Context(ctx).Do()
	}

	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			// Role has been deleted outside of Terraform
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"[GCP Role Definition][Read] Failed to read custom role",
			fmt.Sprintf("Error reading custom role: %s", err.Error()),
		)
		return
	}

	state.Title = types.StringValue(role.Title)
	state.Description = types.StringValue(role.Description)
	state.Deleted = types.BoolValue(role.Deleted)
	state.Stage = types.StringValue(role.Stage)

	sortedPermissions := make([]string, len(role.IncludedPermissions))
	copy(sortedPermissions, role.IncludedPermissions)
	sort.Strings(sortedPermissions)

	permissionsList, diags := types.ListValueFrom(ctx, types.StringType, sortedPermissions)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state.Permissions = permissionsList

	if diags := resp.State.Set(ctx, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *IAMCustomRole) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state customRoleDefinitionResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	roleName := state.Name.ValueString()

	var gcpClients *api.GCPClients
	var diags diag.Diagnostics

	// Determine if this is an organization or project-level role
	isOrgRole := strings.HasPrefix(roleName, "organizations/")

	if isOrgRole {
		gcpClients, diags = api.GetGCPClients(ctx, "")
	} else {
		projectID := state.ProjectID.ValueString()
		gcpClients, diags = api.GetGCPClients(ctx, projectID)
	}

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Extract permissions from plan, or use default core permissions if not provided
	var permissions []string
	if !plan.Permissions.IsNull() && !plan.Permissions.IsUnknown() {
		diags = plan.Permissions.ElementsAs(ctx, &permissions, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Update] Using user-provided permissions count: %d", len(permissions)))
	} else {
		// Use default core permissions if user didn't provide any (make a copy)
		permissions = make([]string, len(config.GCP_CUSTOM_ROLE_CORE_PERMISSIONS))
		copy(permissions, config.GCP_CUSTOM_ROLE_CORE_PERMISSIONS)
		tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Update] Using default core permissions count: %d", len(permissions)))
	}

	// Extract features from plan if provided
	var features []string
	if !plan.FeaturePermissions.IsNull() && !plan.FeaturePermissions.IsUnknown() {
		diags = plan.FeaturePermissions.ElementsAs(ctx, &features, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
	}

	// Aggregate permissions with feature permissions
	aggregatedPermissions, err := r.aggregatePermissions(ctx, permissions, features)
	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Role Definition][Update] Failed to aggregate permissions",
			fmt.Sprintf("Error aggregating permissions: %s", err.Error()),
		)
		return
	}
	permissions = aggregatedPermissions

	// Build the update request
	role := &iam.Role{
		Title:               plan.Title.ValueString(),
		Description:         plan.Description.ValueString(),
		IncludedPermissions: permissions,
		Stage:               plan.Stage.ValueString(),
	}

	// Update the role
	var updatedRole *iam.Role

	if isOrgRole {
		updatedRole, err = gcpClients.IAMClient.Organizations.Roles.Patch(roleName, role).Context(ctx).Do()
	} else {
		updatedRole, err = gcpClients.IAMClient.Projects.Roles.Patch(roleName, role).Context(ctx).Do()
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"[GCP Role Definition][Update] Failed to update custom role",
			fmt.Sprintf("Error updating custom role: %s", err.Error()),
		)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Update] Updated role: %s", updatedRole.Name))

	// Update plan with updated role information
	plan.Title = types.StringValue(updatedRole.Title)
	plan.Description = types.StringValue(updatedRole.Description)
	plan.Deleted = types.BoolValue(updatedRole.Deleted)
	plan.Stage = types.StringValue(updatedRole.Stage)

	// Convert permissions to list - use the sorted permissions we sent, not the API response
	// to ensure consistent ordering
	permissionsList, diags := types.ListValueFrom(ctx, types.StringType, permissions)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan.Permissions = permissionsList

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *IAMCustomRole) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customRoleDefinitionResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	roleName := state.Name.ValueString()

	var gcpClients *api.GCPClients
	var diags diag.Diagnostics

	// Determine if this is an organization or project-level role
	isOrgRole := strings.HasPrefix(roleName, "organizations/")

	if isOrgRole {
		gcpClients, diags = api.GetGCPClients(ctx, "")
	} else {
		projectID := state.ProjectID.ValueString()
		gcpClients, diags = api.GetGCPClients(ctx, projectID)
	}

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Delete the role and handle eventual consistency logically
	var err error

	if isOrgRole {
		_, err = gcpClients.IAMClient.Organizations.Roles.Delete(roleName).Context(ctx).Do()
	} else {
		_, err = gcpClients.IAMClient.Projects.Roles.Delete(roleName).Context(ctx).Do()
	}

	if err != nil {
		// If role is already deleted, we can ignore the error because of the GCP eventual consistency model
		if !strings.Contains(err.Error(), "404") &&
			!strings.Contains(err.Error(), "not found") &&
			!strings.Contains(err.Error(), "already deleted") {
			resp.Diagnostics.AddError(
				"[GCP Role Definition][Delete] Failed to delete custom role",
				fmt.Sprintf("Error deleting custom role: %s", err.Error()),
			)
			return
		}
		tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Delete] Custom role already deleted or in soft-delete period: %s", roleName))
	}

	tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition][Delete] Deleted role: %s", roleName))
}

func (r *IAMCustomRole) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *trendmicro.Client, got something else.",
		)
		return
	}

	r.client = &api.CamClient{
		Client: client,
	}
	tflog.Debug(ctx, "[GCP Role Definition] resource configured successfully")
}

// aggregatePermissions combines base permissions with feature-specific permissions
// TODO: In the future, this will call an API to retrieve permissions for each feature
//
//nolint:unparam // error return is currently always nil but will be used when API integration is implemented
func (r *IAMCustomRole) aggregatePermissions(ctx context.Context, corePermissions, features []string) ([]string, error) {
	// Start with base permissions
	permissionMap := make(map[string]bool)
	for _, perm := range corePermissions {
		permissionMap[perm] = true
	}

	// If features are specified, aggregate their permissions
	if len(features) > 0 {
		// TODO: Call API to get permissions for each feature based on parent type
		// For now, this is a placeholder that will be implemented when the API is ready
		tflog.Debug(ctx, fmt.Sprintf("[GCP Role Definition] Features specified: %v (feature permissions will be added when API is available)", features))
	}

	// Convert map back to slice and sort for consistent ordering
	aggregated := make([]string, 0, len(permissionMap))
	for perm := range permissionMap {
		aggregated = append(aggregated, perm)
	}
	sort.Strings(aggregated)

	return aggregated, nil
}
