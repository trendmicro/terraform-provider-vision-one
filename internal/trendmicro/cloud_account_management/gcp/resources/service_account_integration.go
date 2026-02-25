package resources

import (
	"context"
	"fmt"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
)

type ServiceAccountIntegration struct {
	client *api.CamClient
}

type serviceAccountIntegrationResourceModel struct {
	// Service Account Configuration
	ProjectID                 types.String `tfsdk:"project_id"`
	AccountID                 types.String `tfsdk:"account_id"`
	DisplayName               types.String `tfsdk:"display_name"`
	Description               types.String `tfsdk:"description"`
	CreateIgnoreAlreadyExists types.Bool   `tfsdk:"create_ignore_already_exists"`

	// Multi-Project Scope Configuration
	CentralManagementProjectIDFolder types.String `tfsdk:"central_management_project_id_in_folder"`
	CentralManagementProjectIDOrg    types.String `tfsdk:"central_management_project_id_in_org"`
	ExcludeFreeTrialProjects         types.Bool   `tfsdk:"exclude_free_trial_projects"`
	ExcludeProjects                  types.List   `tfsdk:"exclude_projects"`

	// Role Configuration
	Roles types.List `tfsdk:"roles"`

	// Key Rotation
	RotationTime types.String `tfsdk:"rotation_time"`

	// Computed Outputs
	ServiceAccountEmail    types.String `tfsdk:"service_account_email"`
	ServiceAccountName     types.String `tfsdk:"service_account_name"`
	ServiceAccountUniqueID types.String `tfsdk:"service_account_unique_id"`
	KeyName                types.String `tfsdk:"key_name"`
	PrivateKey             types.String `tfsdk:"private_key"`
	ValidAfter             types.String `tfsdk:"valid_after"`
	ValidBefore            types.String `tfsdk:"valid_before"`
	BoundProjects          types.List   `tfsdk:"bound_projects"`
	BoundProjectNumbers    types.List   `tfsdk:"bound_project_numbers"`
}

func NewServiceAccountIntegration() resource.Resource {
	return &ServiceAccountIntegration{
		client: &api.CamClient{},
	}
}

func (r *ServiceAccountIntegration) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_SERVICE_ACCOUNT_INTEGRATION
}

func (r *ServiceAccountIntegration) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a GCP service account with rotating keys, custom IAM role, and multi-project role bindings for Trend Micro Vision One Cloud Account Management.",
		Attributes: map[string]schema.Attribute{
			// ===== Service Account Configuration =====
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project where the service account will be created. Defaults to provider project configuration.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The account ID (email prefix) for the service account. Must be 6-30 characters, lowercase letters, digits, hyphens.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name for the service account. If not specified, defaults to '" + config.SERVICE_ACCOUNT_DEFAULT_DISPLAY_NAME + "'.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(config.SERVICE_ACCOUNT_DEFAULT_DISPLAY_NAME),
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the service account. Maximum 256 UTF-8 bytes. If not specified, defaults to '" + config.SERVICE_ACCOUNT_DEFAULT_DESCRIPTION + "'.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(config.SERVICE_ACCOUNT_DEFAULT_DESCRIPTION),
			},
			"create_ignore_already_exists": schema.BoolAttribute{
				MarkdownDescription: "If true, skip creation if a service account with the same email already exists (handles GCP 30-day soft deletion). The resource will adopt the existing service account. Defaults to true.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			// ===== Multi-Project Scope Configuration =====
			"central_management_project_id_in_folder": schema.StringAttribute{
				MarkdownDescription: "Project ID under a folder for centralized management. Service account will receive role bindings in all projects under the same folder. Mutually exclusive with central_management_project_id_in_org.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("central_management_project_id_in_org")),
				},
			},
			"central_management_project_id_in_org": schema.StringAttribute{
				MarkdownDescription: "Project ID under an organization for centralized management. Service account will receive role bindings in all projects under the same organization.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("central_management_project_id_in_folder")),
				},
			},
			"exclude_free_trial_projects": schema.BoolAttribute{
				MarkdownDescription: "If true, exclude free trial projects when applying IAM bindings across multiple projects. Only applies when using central_management_project_id_in_folder or central_management_project_id_in_org.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"exclude_projects": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of project IDs to exclude from IAM bindings. Only applies when using central_management_project_id_in_folder or central_management_project_id_in_org.",
				Optional:            true,
			},

			// ===== Role Configuration =====
			"roles": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of IAM role resource names to bind to the service account. Each role will be bound to the service account across all target projects. Supports both custom roles (e.g., projects/{project}/roles/{role_id}) and predefined roles (e.g., roles/viewer). At least one role is required.",
				Required:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},

			// ===== Key Rotation Configuration =====
			"rotation_time": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp from time_rotating resource to trigger key rotation. When this value changes, the old key is deleted and a new key is created. Use with time_rotating resource's rotation_rfc3339 output.",
				Optional:            true,
			},

			// ===== Computed Outputs =====
			"service_account_email": schema.StringAttribute{
				MarkdownDescription: "The email address of the created service account.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_account_name": schema.StringAttribute{
				MarkdownDescription: "The fully-qualified name of the service account (projects/{project}/serviceAccounts/{email}).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_account_unique_id": schema.StringAttribute{
				MarkdownDescription: "The unique numeric ID of the service account.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key_name": schema.StringAttribute{
				MarkdownDescription: "The resource name of the service account key.",
				Computed:            true,
			},
			"private_key": schema.StringAttribute{
				MarkdownDescription: "The private key in JSON format, base64 encoded. This is sensitive and should be stored securely.",
				Computed:            true,
				Sensitive:           true,
			},
			"valid_after": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp indicating when the key becomes valid.",
				Computed:            true,
			},
			"valid_before": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp indicating when the key expires.",
				Computed:            true,
			},
			"bound_projects": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of project IDs where IAM role bindings were created for this service account.",
				Computed:            true,
			},
			"bound_project_numbers": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of project numbers corresponding to bound_projects, in the same order.",
				Computed:            true,
			},
		},
	}
}

func (r *ServiceAccountIntegration) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
		Client: client,
	}
}

// Create creates a new service account with key, custom role, and IAM bindings.
//
//nolint:gocyclo // Terraform CRUD - inherently complex with GCP multi-project role replication
func (r *ServiceAccountIntegration) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountIntegrationResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if err := ValidateServiceAccountID(plan.AccountID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"[Service Account Key][Create] Invalid account_id",
			fmt.Sprintf("Invalid account_id: %s", err.Error()),
		)
		return
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		if err := ValidateDescription(plan.Description.ValueString()); err != nil {
			resp.Diagnostics.AddError(
				"[Service Account Key][Create] Invalid description",
				fmt.Sprintf("Invalid description: %s", err.Error()),
			)
			return
		}
	}

	// Resolve project ID from available sources
	var resolvedProjectID string
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		resolvedProjectID = plan.ProjectID.ValueString()
	} else if !plan.CentralManagementProjectIDFolder.IsNull() && !plan.CentralManagementProjectIDFolder.IsUnknown() {
		resolvedProjectID = plan.CentralManagementProjectIDFolder.ValueString()
	} else if !plan.CentralManagementProjectIDOrg.IsNull() && !plan.CentralManagementProjectIDOrg.IsUnknown() {
		resolvedProjectID = plan.CentralManagementProjectIDOrg.ValueString()
	} else {
		resp.Diagnostics.AddError(
			"[Service Account Key][Create] Missing project_id",
			"Unable to determine project ID for service account creation. Please specify project_id or central_management_project_id_in_folder or central_management_project_id_in_org.",
		)
	}

	gcpClients, diags := api.GetGCPClients(ctx, resolvedProjectID)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	projectID := gcpClients.ProjectID

	plan.ProjectID = types.StringValue(projectID)

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Creating service account in project: %s", projectID))

	sa, err := CreateServiceAccount(ctx, gcpClients, projectID, plan.AccountID.ValueString(),
		plan.DisplayName.ValueString(), plan.Description.ValueString(), plan.CreateIgnoreAlreadyExists.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError(
			"[Service Account Key][Create] Failed to create service account",
			fmt.Sprintf("Error creating service account: %s", err.Error()),
		)
		return
	}

	plan.ServiceAccountEmail = types.StringValue(sa.Email)
	plan.ServiceAccountName = types.StringValue(sa.Name)
	plan.ServiceAccountUniqueID = types.StringValue(sa.UniqueId)

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Service account created: %s", sa.Email))

	// Wait for GCP eventual consistency - service account needs time to propagate before IAM bindings can be created
	tflog.Debug(ctx, "[Service Account Key][Create] Waiting 5 seconds for service account to propagate...")
	time.Sleep(5 * time.Second)

	// Get roles from 'roles' field
	var roleNames []string
	if roleDiags := plan.Roles.ElementsAs(ctx, &roleNames, false); roleDiags.HasError() {
		resp.Diagnostics.Append(roleDiags...)
		return
	}

	// Validate all roles
	for _, roleName := range roleNames {
		_, err = gcpClients.IAMClient.Projects.Roles.Get(roleName).Context(ctx).Do()
		if err != nil {
			if strings.Contains(roleName, "projects/") && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found")) {
				resp.Diagnostics.AddError(
					"[Service Account Key][Create] Custom role not found",
					fmt.Sprintf("The specified custom role %s does not exist. Please create it first using visionone_cam_iam_custom_role resource.", roleName),
				)
				return
			}
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Could not validate role (this is normal for predefined roles): %s", roleName))
		} else {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Validated custom role exists: %s", roleName))
		}
	}

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Binding %d role(s) to service account", len(roleNames)))

	targetProjects, err := r.discoverTargetProjects(ctx, gcpClients, &plan, projectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"[Service Account Key][Create] Failed to discover target projects",
			fmt.Sprintf("Error discovering projects: %s", err.Error()),
		)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Discovered %d target projects", len(targetProjects)))

	member := fmt.Sprintf("serviceAccount:%s", sa.Email)
	var boundProjectIds []string

	// Replicate project-scoped custom roles to all target projects
	// GCP limitation: project-level custom roles can only be used in their own project
	// Solution: Create a copy of the custom role in each target project
	customRoleReplicaMap := make(map[string]map[string]string) // projectID -> originalRoleName -> replicatedRoleName

	for _, roleName := range roleNames {
		// Organization-level custom roles work across all projects - no replication needed
		if strings.HasPrefix(roleName, "organizations/") {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Organization-level custom role detected: %s - no replication needed", roleName))
			continue
		}

		if strings.HasPrefix(roleName, "projects/") {
			// This is a project-scoped custom role - we need to replicate it
			sourceRole, getRoleErr := gcpClients.IAMClient.Projects.Roles.Get(roleName).Context(ctx).Do()
			if getRoleErr != nil {
				tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Failed to get custom role %s: %s", roleName, getRoleErr.Error()))
				continue
			}

			// Extract role ID from the role name
			parts := strings.Split(roleName, "/")
			if len(parts) >= 4 {
				roleID := parts[3]

				// Create this custom role in each target project
				for _, targetProjectID := range targetProjects {
					// Skip the source project (role already exists there)
					sourceProjectID := parts[1]
					if targetProjectID == sourceProjectID {
						continue
					}

					// Check if role already exists in target project
					targetRoleName := fmt.Sprintf("projects/%s/roles/%s", targetProjectID, roleID)
					_, checkErr := gcpClients.IAMClient.Projects.Roles.Get(targetRoleName).Context(ctx).Do()

					if checkErr != nil && (strings.Contains(checkErr.Error(), "404") || strings.Contains(checkErr.Error(), "not found")) {
						// Role doesn't exist, create it
						tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Replicating custom role %s to project %s", roleID, targetProjectID))

						createRoleReq := &iam.CreateRoleRequest{
							RoleId: roleID,
							Role: &iam.Role{
								Title:               sourceRole.Title,
								Description:         fmt.Sprintf("%s (replicated for multi-project service account)", sourceRole.Description),
								IncludedPermissions: sourceRole.IncludedPermissions,
								Stage:               sourceRole.Stage,
							},
						}

						_, createErr := gcpClients.IAMClient.Projects.Roles.Create(fmt.Sprintf("projects/%s", targetProjectID), createRoleReq).Context(ctx).Do()
						if createErr != nil {
							tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Failed to create custom role in project %s: %s", targetProjectID, createErr.Error()))
						} else {
							tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Create] Created custom role %s in project %s", roleID, targetProjectID))
							// Track the replicated role
							if customRoleReplicaMap[targetProjectID] == nil {
								customRoleReplicaMap[targetProjectID] = make(map[string]string)
							}
							customRoleReplicaMap[targetProjectID][roleName] = targetRoleName
						}
					} else if checkErr == nil {
						// Role already exists
						tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Custom role %s already exists in project %s", roleID, targetProjectID))
						if customRoleReplicaMap[targetProjectID] == nil {
							customRoleReplicaMap[targetProjectID] = make(map[string]string)
						}
						customRoleReplicaMap[targetProjectID][roleName] = targetRoleName
					}
				}
			}
		}
	}

	// Bind all roles to all target projects
	for _, targetProjectID := range targetProjects {
		projectBound := false
		for _, roleName := range roleNames {
			actualRoleName := roleName

			// Organization-level roles can be bound directly to any project
			if strings.HasPrefix(roleName, "organizations/") {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Using organization-level custom role %s for project %s", roleName, targetProjectID))
				// actualRoleName is already correct, proceed to binding
			} else if strings.HasPrefix(roleName, "projects/") {
				// If this is a project-scoped custom role, use the replicated version for other projects
				parts := strings.Split(roleName, "/")
				if len(parts) >= 2 {
					sourceProjectID := parts[1]
					if sourceProjectID != targetProjectID {
						// Use the replicated role name if available
						if replicatedName, exists := customRoleReplicaMap[targetProjectID][roleName]; exists {
							actualRoleName = replicatedName
							tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Using replicated custom role %s for project %s", actualRoleName, targetProjectID))
						} else {
							tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Custom role was not replicated to project %s, skipping binding", targetProjectID))
							continue
						}
					}
				}
			}

			bindErr := AddIAMBinding(ctx, gcpClients, targetProjectID, member, actualRoleName)
			if bindErr != nil {
				tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Failed to add IAM binding for role %s to project %s: %s", actualRoleName, targetProjectID, bindErr.Error()))
				continue
			}
			projectBound = true
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] IAM binding for role %s added to project: %s", actualRoleName, targetProjectID))
		}
		if projectBound {
			boundProjectIds = append(boundProjectIds, targetProjectID)
		}
	}

	tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Create] IAM bindings created in %d projects for service account: %s", len(boundProjectIds), sa.Email))

	boundProjectsList, diags := types.ListValueFrom(ctx, types.StringType, boundProjectIds)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan.BoundProjects = boundProjectsList

	// Resolve project numbers for bound projects
	projectNumberMap := r.resolveProjectNumbers(ctx, gcpClients, boundProjectIds)
	var boundProjectNumbers []string
	for _, pid := range boundProjectIds {
		if num, ok := projectNumberMap[pid]; ok {
			boundProjectNumbers = append(boundProjectNumbers, num)
		}
	}
	boundProjectNumbersList, diags := types.ListValueFrom(ctx, types.StringType, boundProjectNumbers)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan.BoundProjectNumbers = boundProjectNumbersList

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Bindings created in %d projects", len(boundProjectIds)))

	key, err := CreateServiceAccountKey(ctx, gcpClients, sa.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"[Service Account Key][Create] Failed to create service account key",
			fmt.Sprintf("Error creating service account key: %s", err.Error()),
		)
		return
	}

	plan.KeyName = types.StringValue(key.Name)
	plan.PrivateKey = types.StringValue(key.PrivateKeyData)
	plan.ValidAfter = types.StringValue(key.ValidAfterTime)
	plan.ValidBefore = types.StringValue(key.ValidBeforeTime)

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Service account key created: %s", key.Name))

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Create] Successfully created service account key resource: %s", sa.Email))
}

//nolint:gocyclo // Terraform CRUD - inherently complex with multi-project IAM verification
func (r *ServiceAccountIntegration) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountIntegrationResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	gcpClients, diags := api.GetGCPClients(ctx, state.ProjectID.ValueString())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Read] Reading service account: %s", state.ServiceAccountName.ValueString()))

	sa, err := gcpClients.IAMClient.Projects.ServiceAccounts.Get(state.ServiceAccountName.ValueString()).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Service account not found, removing from state: %s", state.ServiceAccountName.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"[Service Account Key][Read] Failed to read service account",
			fmt.Sprintf("Error reading service account: %s", err.Error()),
		)
		return
	}

	state.ServiceAccountEmail = types.StringValue(sa.Email)
	state.ServiceAccountUniqueID = types.StringValue(sa.UniqueId)

	// Get roles from 'roles' field
	var roleNames []string
	if roleDiags := state.Roles.ElementsAs(ctx, &roleNames, false); roleDiags.HasError() {
		resp.Diagnostics.Append(roleDiags...)
		return
	}

	// Validate all roles still exist
	for _, roleName := range roleNames {
		_, err = gcpClients.IAMClient.Projects.Roles.Get(roleName).Context(ctx).Do()
		if err != nil {
			if strings.Contains(roleName, "projects/") && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found")) {
				tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Custom role not found: %s", roleName))
			}
		}
	}

	var currentBoundProjects []string
	var currentBoundProjectNumbers []string
	var stateBoundProjects []string
	if bpDiags := state.BoundProjects.ElementsAs(ctx, &stateBoundProjects, false); bpDiags.HasError() {
		resp.Diagnostics.Append(bpDiags...)
		return
	}
	var stateBoundProjectNumbers []string
	if !state.BoundProjectNumbers.IsNull() {
		if bpnDiags := state.BoundProjectNumbers.ElementsAs(ctx, &stateBoundProjectNumbers, false); bpnDiags.HasError() {
			resp.Diagnostics.Append(bpnDiags...)
			return
		}
	}

	member := fmt.Sprintf("serviceAccount:%s", sa.Email)
	for p, projectID := range stateBoundProjects {
		getReq := &cloudresourcemanager.GetIamPolicyRequest{}
		policy, policyErr := gcpClients.CRMClient.Projects.GetIamPolicy(projectID, getReq).Context(ctx).Do()
		if policyErr != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Failed to get IAM policy for project %s: %s", projectID, policyErr.Error()))
			continue
		}

		// Check if all roles are bound to this project
		allRolesBound := true
		for _, roleName := range roleNames {
			actualRoleName := roleName

			// Organization-level roles can be bound directly to any project
			if strings.HasPrefix(roleName, "organizations/") {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Read] Checking organization-level custom role %s for project %s", roleName, projectID))
				// actualRoleName is already correct
			} else if strings.HasPrefix(roleName, "projects/") {
				// If this is a project-scoped custom role, use the replicated version for other projects
				parts := strings.Split(roleName, "/")
				if len(parts) >= 4 {
					sourceProjectID := parts[1]
					roleID := parts[3]

					if sourceProjectID != projectID {
						// Use the replicated role name
						actualRoleName = fmt.Sprintf("projects/%s/roles/%s", projectID, roleID)
						tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Read] Checking replicated custom role %s for project %s", actualRoleName, projectID))
					}
				}
			}

			if !HasRoleBinding(policy, actualRoleName, member) {
				allRolesBound = false
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Read] Drift detected: IAM binding for role %s missing in project %s", actualRoleName, projectID))
				break
			}
		}

		if allRolesBound {
			currentBoundProjects = append(currentBoundProjects, projectID)
			if p < len(stateBoundProjectNumbers) {
				currentBoundProjectNumbers = append(currentBoundProjectNumbers, stateBoundProjectNumbers[p])
			}
		}
	}

	boundProjectsList, diags := types.ListValueFrom(ctx, types.StringType, currentBoundProjects)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state.BoundProjects = boundProjectsList

	boundProjectNumbersList, diags := types.ListValueFrom(ctx, types.StringType, currentBoundProjectNumbers)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state.BoundProjectNumbers = boundProjectNumbersList

	key, err := gcpClients.IAMClient.Projects.ServiceAccounts.Keys.Get(state.KeyName.ValueString()).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Key not found: %s", state.KeyName.ValueString()))
		} else {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Failed to read key: %s", err.Error()))
		}
	} else {
		state.ValidAfter = types.StringValue(key.ValidAfterTime)
		state.ValidBefore = types.StringValue(key.ValidBeforeTime)
	}

	if diags := resp.State.Set(ctx, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Read] Successfully read service account key resource: %s", sa.Email))
}

//nolint:gocyclo // Terraform CRUD - inherently complex with multi-project IAM update
func (r *ServiceAccountIntegration) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state serviceAccountIntegrationResourceModel

	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	gcpClients, diags := api.GetGCPClients(ctx, state.ProjectID.ValueString())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Update] Updating service account key resource: %s", state.ServiceAccountName.ValueString()))

	if !plan.DisplayName.Equal(state.DisplayName) || !plan.Description.Equal(state.Description) {
		tflog.Debug(ctx, "[Service Account Key][Update] Updating service account display name or description")

		sa, err := gcpClients.IAMClient.Projects.ServiceAccounts.Get(state.ServiceAccountName.ValueString()).Context(ctx).Do()
		if err != nil {
			resp.Diagnostics.AddError(
				"[Service Account Key][Update] Failed to get service account",
				fmt.Sprintf("Error getting service account: %s", err.Error()),
			)
			return
		}

		sa.DisplayName = plan.DisplayName.ValueString()
		sa.Description = plan.Description.ValueString()

		patchReq := &iam.PatchServiceAccountRequest{
			ServiceAccount: sa,
		}

		_, err = gcpClients.IAMClient.Projects.ServiceAccounts.Patch(state.ServiceAccountName.ValueString(), patchReq).Context(ctx).Do()
		if err != nil {
			resp.Diagnostics.AddError(
				"[Service Account Key][Update] Failed to update service account",
				fmt.Sprintf("Error updating service account: %s", err.Error()),
			)
			return
		}

		tflog.Debug(ctx, "[Service Account Key][Update] Service account updated successfully")
	}

	if !plan.ExcludeProjects.Equal(state.ExcludeProjects) || !plan.ExcludeFreeTrialProjects.Equal(state.ExcludeFreeTrialProjects) {
		tflog.Debug(ctx, "[Service Account Key][Update] Updating IAM bindings due to exclusion changes")

		newTargetProjects, err := r.discoverTargetProjects(ctx, gcpClients, &plan, state.ProjectID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"[Service Account Key][Update] Failed to discover target projects",
				fmt.Sprintf("Error discovering projects: %s", err.Error()),
			)
			return
		}

		var oldBoundProjects []string
		if diags := state.BoundProjects.ElementsAs(ctx, &oldBoundProjects, false); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		oldProjectsMap := make(map[string]bool)
		for _, proj := range oldBoundProjects {
			oldProjectsMap[proj] = true
		}

		newProjectsMap := make(map[string]bool)
		for _, proj := range newTargetProjects {
			newProjectsMap[proj] = true
		}

		member := fmt.Sprintf("serviceAccount:%s", state.ServiceAccountEmail.ValueString())

		// Get roles from 'roles' field
		var roleNames []string
		if diags := state.Roles.ElementsAs(ctx, &roleNames, false); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		// Remove all role bindings from projects that are no longer in scope
		for _, proj := range oldBoundProjects {
			if !newProjectsMap[proj] {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Update] Removing bindings from project: %s", proj))
				for _, roleName := range roleNames {
					if err := RemoveIAMBinding(ctx, gcpClients, proj, member, roleName); err != nil {
						tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Update] Failed to remove binding for role %s from project %s: %s", roleName, proj, err.Error()))
					}
				}
			}
		}

		// Add all role bindings to new projects
		for _, proj := range newTargetProjects {
			if !oldProjectsMap[proj] {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Update] Adding bindings to project: %s", proj))
				for _, roleName := range roleNames {
					if err := AddIAMBinding(ctx, gcpClients, proj, member, roleName); err != nil {
						tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Update] Failed to add binding for role %s to project %s: %s", roleName, proj, err.Error()))
					}
				}
			}
		}

		boundProjectsList, diags := types.ListValueFrom(ctx, types.StringType, newTargetProjects)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		plan.BoundProjects = boundProjectsList

		// Resolve project numbers for new target projects
		projectNumberMap := r.resolveProjectNumbers(ctx, gcpClients, newTargetProjects)
		var newProjectNumbers []string
		for _, pid := range newTargetProjects {
			if num, ok := projectNumberMap[pid]; ok {
				newProjectNumbers = append(newProjectNumbers, num)
			}
		}
		boundProjectNumbersList, diags := types.ListValueFrom(ctx, types.StringType, newProjectNumbers)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		plan.BoundProjectNumbers = boundProjectNumbersList
	} else {
		plan.BoundProjects = state.BoundProjects
		plan.BoundProjectNumbers = state.BoundProjectNumbers
	}

	if !plan.RotationTime.Equal(state.RotationTime) {
		tflog.Debug(ctx, "[Service Account Key][Update] Rotation time changed, rotating key")

		err := DeleteServiceAccountKey(ctx, gcpClients, state.KeyName.ValueString())
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Update] Failed to delete old key: %s", err.Error()))
		}

		newKey, err := CreateServiceAccountKey(ctx, gcpClients, state.ServiceAccountName.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"[Service Account Key][Update] Failed to create new key during rotation",
				fmt.Sprintf("Error creating new key: %s", err.Error()),
			)
			return
		}

		plan.KeyName = types.StringValue(newKey.Name)
		plan.PrivateKey = types.StringValue(newKey.PrivateKeyData)
		plan.ValidAfter = types.StringValue(newKey.ValidAfterTime)
		plan.ValidBefore = types.StringValue(newKey.ValidBeforeTime)

		tflog.Debug(ctx, "[Service Account Key][Update] Key rotated successfully")
	} else {
		plan.KeyName = state.KeyName
		plan.PrivateKey = state.PrivateKey
		plan.ValidAfter = state.ValidAfter
		plan.ValidBefore = state.ValidBefore
	}

	plan.ServiceAccountEmail = state.ServiceAccountEmail
	plan.ServiceAccountName = state.ServiceAccountName
	plan.ServiceAccountUniqueID = state.ServiceAccountUniqueID

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Update] Successfully updated service account key resource: %s", state.ServiceAccountEmail.ValueString()))
}

// Delete deletes the resource.
func (r *ServiceAccountIntegration) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountIntegrationResourceModel

	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	gcpClients, diags := api.GetGCPClients(ctx, state.ProjectID.ValueString())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Deleting service account key resource: %s", state.ServiceAccountName.ValueString()))

	err := DeleteServiceAccountKey(ctx, gcpClients, state.KeyName.ValueString())
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to delete key: %s", err.Error()))
	} else {
		tflog.Debug(ctx, "[Service Account Key][Delete] Service account key deleted")
	}

	tflog.Debug(ctx, "[Service Account Key][Delete] Rediscovering target projects for complete IAM cleanup")
	currentTargetProjects, err := r.discoverTargetProjects(ctx, gcpClients, &state, state.ProjectID.ValueString())
	if err != nil {
		// If rediscovery fails, fall back to using bound_projects from state
		tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to rediscover target projects, falling back to bound_projects from state: %s", err.Error()))

		var boundProjects []string
		if bpDiags := state.BoundProjects.ElementsAs(ctx, &boundProjects, false); bpDiags.HasError() {
			resp.Diagnostics.Append(bpDiags...)
			return
		}
		currentTargetProjects = boundProjects
	} else {
		tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Delete] Successfully rediscovered %d target projects for cleanup", len(currentTargetProjects)))
	}

	member := fmt.Sprintf("serviceAccount:%s", state.ServiceAccountEmail.ValueString())

	// Get roles from 'roles' field
	var roleNames []string
	if roleDiags := state.Roles.ElementsAs(ctx, &roleNames, false); roleDiags.HasError() {
		resp.Diagnostics.Append(roleDiags...)
		return
	}

	// Remove all role bindings from all target projects
	for _, projectID := range currentTargetProjects {
		for _, roleName := range roleNames {
			actualRoleName := roleName

			// For project-scoped custom roles, use the replicated role name in non-source projects
			if strings.HasPrefix(roleName, "projects/") {
				parts := strings.Split(roleName, "/")
				if len(parts) >= 4 {
					sourceProjectID := parts[1]
					roleID := parts[3]

					// If this is not the source project, use the replicated role name
					if projectID != sourceProjectID {
						actualRoleName = fmt.Sprintf("projects/%s/roles/%s", projectID, roleID)
						tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Using replicated role name for removal: %s", actualRoleName))
					}
				}
			}

			removeErr := RemoveIAMBinding(ctx, gcpClients, projectID, member, actualRoleName)
			if removeErr != nil {
				tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to remove IAM binding for role %s from project %s: %s", actualRoleName, projectID, removeErr.Error()))
			} else {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] IAM binding for role %s removed from project: %s", actualRoleName, projectID))
			}
		}
	}

	// Delete replicated custom roles from all bound projects
	// Organization-level custom roles are not replicated, so we only need to clean up project-scoped custom roles
	for _, roleName := range roleNames {
		// Skip organization-level roles (they were not replicated)
		if strings.HasPrefix(roleName, "organizations/") {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Skipping organization-level custom role: %s (not replicated)", roleName))
			continue
		}

		// Only delete replicated project-scoped custom roles
		if strings.HasPrefix(roleName, "projects/") {
			parts := strings.Split(roleName, "/")
			if len(parts) >= 4 {
				sourceProjectID := parts[1]
				roleID := parts[3]

				// Delete the replicated custom role from each target project (except the source project)
				for _, projectID := range currentTargetProjects {
					if projectID == sourceProjectID {
						// Skip the source project - the custom role resource itself will handle deletion
						tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Skipping custom role deletion in source project: %s", projectID))
						continue
					}

					// Try to delete the replicated custom role
					replicatedRoleName := fmt.Sprintf("projects/%s/roles/%s", projectID, roleID)
					tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Attempting to delete replicated custom role: %s", replicatedRoleName))

					_, deleteErr := gcpClients.IAMClient.Projects.Roles.Delete(replicatedRoleName).Context(ctx).Do()
					if deleteErr != nil {
						if strings.Contains(deleteErr.Error(), "404") || strings.Contains(deleteErr.Error(), "not found") {
							tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Replicated custom role already deleted: %s", replicatedRoleName))
						} else {
							tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to delete replicated custom role %s: %s", replicatedRoleName, deleteErr.Error()))
						}
					} else {
						tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Delete] Successfully deleted replicated custom role: %s", replicatedRoleName))
					}
				}
			}
		}
	}

	_, err = gcpClients.IAMClient.Projects.ServiceAccounts.Delete(state.ServiceAccountName.ValueString()).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, "[Service Account Key][Delete] Service account already deleted or in soft-delete period")
		} else {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to delete service account: %s", err.Error()))
		}
	} else {
		tflog.Debug(ctx, "[Service Account Key][Delete] Service account deleted (30-day soft delete period starts)")
	}

	tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Delete] Successfully deleted service account key resource: %s", state.ServiceAccountEmail.ValueString()))
}

// ===== Helper Functions =====

// discoverTargetProjects discovers projects for IAM binding based on folder/org scope.
//
//nolint:gocyclo // Complex discovery logic with recursive folder/org traversal
func (r *ServiceAccountIntegration) discoverTargetProjects(
	ctx context.Context,
	gcpClients *api.GCPClients,
	plan *serviceAccountIntegrationResourceModel,
	serviceAccountProjectID string,
) ([]string, error) {
	// If no central management specified, return only the service account's project
	if plan.CentralManagementProjectIDFolder.IsNull() && plan.CentralManagementProjectIDOrg.IsNull() {
		return []string{serviceAccountProjectID}, nil
	}

	var targetResourceID string
	var resourceType string // "folder" or "organization"

	// Determine scope type and get ancestry
	if !plan.CentralManagementProjectIDFolder.IsNull() {
		centralProjID := plan.CentralManagementProjectIDFolder.ValueString()
		ancestry, err := gcpClients.CRMClient.Projects.GetAncestry(centralProjID, &cloudresourcemanager.GetAncestryRequest{}).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get ancestry for folder project %s: %w", centralProjID, err)
		}

		// Find folder in ancestry (skip project and look for first folder)
		for _, ancestor := range ancestry.Ancestor {
			if ancestor.ResourceId.Type == config.PARENT_TYPE_FOLDER {
				targetResourceID = ancestor.ResourceId.Id
				resourceType = config.PARENT_TYPE_FOLDER
				break
			}
		}
		if targetResourceID == "" {
			return nil, fmt.Errorf("no folder found in ancestry of project %s", centralProjID)
		}
	} else {
		centralProjID := plan.CentralManagementProjectIDOrg.ValueString()
		ancestry, err := gcpClients.CRMClient.Projects.GetAncestry(centralProjID,
			&cloudresourcemanager.GetAncestryRequest{}).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get ancestry for org project %s: %w", centralProjID, err)
		}

		// Find organization in ancestry
		for _, ancestor := range ancestry.Ancestor {
			if ancestor.ResourceId.Type == config.PARENT_TYPE_ORGANIZATION {
				targetResourceID = ancestor.ResourceId.Id
				resourceType = config.PARENT_TYPE_ORGANIZATION
				break
			}
		}
		if targetResourceID == "" {
			return nil, fmt.Errorf("no organization found in ancestry of project %s", centralProjID)
		}
	}

	// List all projects and filter by parent
	var targetProjects []string

	if resourceType == config.PARENT_TYPE_FOLDER {
		// Recursively discover all folders under the target folder
		allFolderIDs, err := DiscoverAllFolders(ctx, gcpClients, targetResourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to discover folders: %w", err)
		}
		tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Discovered %d folders (including sub-folders): %v", len(allFolderIDs), allFolderIDs))

		// List projects from all folders
		for _, folderID := range allFolderIDs {
			filter := fmt.Sprintf("parent.type:folder parent.id:%s", folderID)
			err := gcpClients.CRMClient.Projects.List().Filter(filter).Pages(ctx,
				func(resp *cloudresourcemanager.ListProjectsResponse) error {
					for _, project := range resp.Projects {
						// Filter out non-active projects
						if project.LifecycleState != config.LIFECYCLE_STATE_ACTIVE {
							continue
						}

						// Filter out free trial projects if requested
						if plan.ExcludeFreeTrialProjects.ValueBool() {
							if IsFreeTrialProject(project) {
								tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Excluding free trial project: %s", project.ProjectId))
								continue
							}
						}

						targetProjects = append(targetProjects, project.ProjectId)
					}
					return nil
				})
			if err != nil {
				return nil, fmt.Errorf("failed to list projects in folder %s: %w", folderID, err)
			}
		}
	} else {
		// For organization scope, discover all folders in the organization recursively
		// Then list projects from each folder AND projects directly under the organization

		// First, discover all folders in the organization
		allFolderIDs, err := DiscoverAllFoldersInOrganization(ctx, gcpClients, targetResourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to discover folders in organization: %w", err)
		}
		tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Discovered %d folders in organization: %v", len(allFolderIDs), allFolderIDs))

		// List projects from all folders
		for _, folderID := range allFolderIDs {
			filter := fmt.Sprintf("parent.type:folder parent.id:%s", folderID)
			listErr := gcpClients.CRMClient.Projects.List().Filter(filter).Pages(ctx,
				func(resp *cloudresourcemanager.ListProjectsResponse) error {
					for _, project := range resp.Projects {
						// Filter out non-active projects
						if project.LifecycleState != config.LIFECYCLE_STATE_ACTIVE {
							continue
						}

						// Filter out free trial projects if requested
						if plan.ExcludeFreeTrialProjects.ValueBool() {
							if IsFreeTrialProject(project) {
								tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Excluding free trial project: %s", project.ProjectId))
								continue
							}
						}

						targetProjects = append(targetProjects, project.ProjectId)
					}
					return nil
				})
			if listErr != nil {
				return nil, fmt.Errorf("failed to list projects in folder %s: %w", folderID, listErr)
			}
		}

		// Also list projects directly under the organization (not in any folder)
		filter := fmt.Sprintf("parent.type:organization parent.id:%s", targetResourceID)
		err = gcpClients.CRMClient.Projects.List().Filter(filter).Pages(ctx,
			func(resp *cloudresourcemanager.ListProjectsResponse) error {
				for _, project := range resp.Projects {
					// Filter out non-active projects
					if project.LifecycleState != config.LIFECYCLE_STATE_ACTIVE {
						continue
					}

					// Filter out free trial projects if requested
					if plan.ExcludeFreeTrialProjects.ValueBool() {
						if IsFreeTrialProject(project) {
							tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Excluding free trial project: %s", project.ProjectId))
							continue
						}
					}

					targetProjects = append(targetProjects, project.ProjectId)
				}
				return nil
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list projects directly under organization: %w", err)
		}
	}

	// Apply exclude list
	if !plan.ExcludeProjects.IsNull() {
		var excludeList []string
		if diags := plan.ExcludeProjects.ElementsAs(ctx, &excludeList, false); diags.HasError() {
			return nil, fmt.Errorf("failed to parse exclude_projects list")
		}

		targetProjects = FilterProjects(targetProjects, excludeList)
	}

	return targetProjects, nil
}

// resolveProjectNumbers looks up the GCP project number for each project ID.
func (r *ServiceAccountIntegration) resolveProjectNumbers(ctx context.Context, gcpClients *api.GCPClients, projectIDs []string) map[string]string {
	result := make(map[string]string)
	for _, pid := range projectIDs {
		proj, err := gcpClients.CRMClient.Projects.Get(pid).Context(ctx).Do()
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key] Failed to get project number for %s: %s", pid, err.Error()))
			continue
		}
		result[pid] = fmt.Sprintf("%d", proj.ProjectNumber)
	}
	return result
}
