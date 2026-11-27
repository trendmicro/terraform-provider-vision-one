package resources

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/api"
	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
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
	Roles               types.List `tfsdk:"roles"`
	PrimaryProjectRoles types.List `tfsdk:"primary_project_roles"`
	NodeScanRoles       types.List `tfsdk:"node_scan_roles"`

	// Key Rotation
	RotationTime types.String `tfsdk:"rotation_time"`

	// Computed Outputs
	ServiceAccountEmail           types.String `tfsdk:"service_account_email"`
	ServiceAccountName            types.String `tfsdk:"service_account_name"`
	ServiceAccountUniqueID        types.String `tfsdk:"service_account_unique_id"`
	KeyName                       types.String `tfsdk:"key_name"`
	PrivateKey                    types.String `tfsdk:"private_key"`
	ValidAfter                    types.String `tfsdk:"valid_after"`
	ValidBefore                   types.String `tfsdk:"valid_before"`
	BoundProjects                 types.List   `tfsdk:"bound_projects"`
	BoundProjectNumbers           types.List   `tfsdk:"bound_project_numbers"`
	BindingReconciliationRevision types.Int64  `tfsdk:"binding_reconciliation_revision"`
	// NodeScanBindingResource is "folders/{id}" or "organizations/{id}" where the node scan roles were granted.
	NodeScanBindingResource types.String `tfsdk:"node_scan_binding_resource"`
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
			"primary_project_roles": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of IAM role resource names to bind ONLY to the primary project (the service account's home project). These roles will NOT be replicated to sub-projects. Typically used for roles containing elevated permissions such as service account key management (e.g., iam.serviceAccountKeys.create/delete, iam.serviceAccounts.getAccessToken).",
				Optional:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"node_scan_roles": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of IAM role resource names to grant once at the folder or organization node for read-only discovery and scanning. Projects under the node, including projects created later, inherit these roles through IAM, so no per-project scan binding is required. Typically the organization-level scan custom role plus the predefined roles/viewer (a basic role cannot be inlined into a custom role, so it must be granted separately). Only applies in folder or organization mode (central_management_project_id_in_folder or central_management_project_id_in_org); ignored for single-project integrations. Changing this reconciles the node grants in place (e.g. enabling auto-detect after onboarding) without recreating the service account.",
				Optional:            true,
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
				MarkdownDescription: "List of project IDs in scope for this service account. Project discovery and IAM binding checks run on every plan, so newly created projects are added, deleted or excluded projects are removed, and missing bindings trigger an update to restore the configured roles.",
				Computed:            true,
			},
			"bound_project_numbers": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of project numbers corresponding to bound_projects, in the same order.",
				Computed:            true,
			},
			"binding_reconciliation_revision": schema.Int64Attribute{
				MarkdownDescription: "Provider-managed revision that triggers IAM binding reconciliation when a configured binding is missing.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"node_scan_binding_resource": schema.StringAttribute{
				MarkdownDescription: "The folder or organization resource (folders/{id} or organizations/{id}) where the node scan roles were granted. Set when central_management_project_id_in_folder or central_management_project_id_in_org is used together with node_scan_roles.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
		Client: client.WithTimeout(cam.CAMAPITimeout),
	}
}

// Create creates a new service account with key, custom role, and IAM bindings.
//
//nolint:gocyclo // Terraform CRUD - inherently complex with GCP multi-project role replication
func ensureDiscoveryRole(roles []string) []string {
	if slices.Contains(roles, config.GCP_SA_DISCOVERY_ROLE) {
		return roles
	}
	return append(roles, config.GCP_SA_DISCOVERY_ROLE)
}

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

	var primaryRoleNames []string
	if !plan.PrimaryProjectRoles.IsNull() {
		if roleDiags := plan.PrimaryProjectRoles.ElementsAs(ctx, &primaryRoleNames, false); roleDiags.HasError() {
			resp.Diagnostics.Append(roleDiags...)
			return
		}
		for _, roleName := range primaryRoleNames {
			_, err = gcpClients.IAMClient.Projects.Roles.Get(roleName).Context(ctx).Do()
			if err != nil {
				if strings.Contains(roleName, "projects/") && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found")) {
					resp.Diagnostics.AddError(
						"[Service Account Key][Create] Primary project custom role not found",
						fmt.Sprintf("The specified primary project custom role %s does not exist. Please create it first using visionone_cam_iam_custom_role resource.", roleName),
					)
					return
				}
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Could not validate primary role (this is normal for predefined roles): %s", roleName))
			} else {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Validated primary project custom role exists: %s", roleName))
			}
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

						createErr := createReplicatedCustomRole(ctx, gcpClients, sourceRole, targetProjectID, roleID)
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

	// Bind all roles to all target projects concurrently (per project)
	var (
		bindMu          sync.Mutex
		bindWg          sync.WaitGroup
		boundProjectIds []string
		bindFailures    = map[string][]string{}
	)
	sem := cam.GCPServiceAccountSem
	for _, targetProjectID := range targetProjects {
		bindWg.Add(1)
		go func(projID string) {
			defer bindWg.Done()
			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot

			tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Create] Adding service account %s as principal to project: %s", sa.Email, projID))
			projectBound := false
			var roleErrs []string
			for _, roleName := range roleNames {
				actualRoleName := roleName

				// Organization-level roles can be bound directly to any project
				if strings.HasPrefix(roleName, "organizations/") {
					tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Using organization-level custom role %s for project %s", roleName, projID))
					// actualRoleName is already correct, proceed to binding
				} else if strings.HasPrefix(roleName, "projects/") {
					// If this is a project-scoped custom role, use the replicated version for other projects
					parts := strings.Split(roleName, "/")
					if len(parts) >= 2 {
						sourceProjectID := parts[1]
						if sourceProjectID != projID {
							// Use the replicated role name if available
							if replicatedName, exists := customRoleReplicaMap[projID][roleName]; exists {
								actualRoleName = replicatedName
								tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Using replicated custom role %s for project %s", actualRoleName, projID))
							} else {
								tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Custom role was not replicated to project %s, skipping binding", projID))
								roleErrs = append(roleErrs, fmt.Sprintf("%s: custom role was not replicated to this project", roleName))
								continue
							}
						}
					}
				}

				bindErr := AddIAMBinding(ctx, gcpClients, projID, member, actualRoleName)
				if bindErr != nil {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Failed to add IAM binding for role %s to project %s: %s", actualRoleName, projID, bindErr.Error()))
					roleErrs = append(roleErrs, fmt.Sprintf("%s: %s", actualRoleName, bindErr.Error()))
					continue
				}
				projectBound = true
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] IAM binding for role %s added to project: %s", actualRoleName, projID))
			}
			bindMu.Lock()
			if projectBound {
				boundProjectIds = append(boundProjectIds, projID)
				tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Create] Successfully added service account as principal to project: %s", projID))
			} else if len(roleErrs) > 0 {
				bindFailures[projID] = roleErrs
			}
			bindMu.Unlock()
		}(targetProjectID)
	}
	bindWg.Wait()

	if roleErrs, primaryFailed := bindFailures[projectID]; primaryFailed {
		resp.Diagnostics.AddError(
			"[Service Account Key][Create] Failed to bind any role to the primary project",
			fmt.Sprintf("Service account %s was created but could not be granted any of its roles on project %s:\n\n  - %s\n\n"+
				"The service account is therefore not a principal on the project, which will surface later as an empty bound_projects "+
				"or as an IAM permission timeout in dependent resources.",
				sa.Email, projectID, strings.Join(roleErrs, "\n  - ")),
		)
		return
	}
	if len(bindFailures) > 0 {
		failedProjects := make([]string, 0, len(bindFailures))
		for projID := range bindFailures {
			failedProjects = append(failedProjects, projID)
		}
		slices.Sort(failedProjects)
		tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] No roles could be bound on %d secondary project(s), continuing: %s",
			len(failedProjects), strings.Join(failedProjects, ", ")))
	}

	if len(boundProjectIds) == 0 {
		resp.Diagnostics.AddError(
			"[Service Account Key][Create] No projects were bound to the service account",
			fmt.Sprintf("Service account %s was created but bound to no projects, so bound_projects and bound_project_numbers would be empty. Expected at least project %s.",
				sa.Email, projectID),
		)
		return
	}

	// Bind primary project roles to the primary project only (serial, no replication)
	for _, primaryRoleName := range primaryRoleNames {
		bindErr := AddIAMBinding(ctx, gcpClients, projectID, member, primaryRoleName)
		if bindErr != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Failed to add primary project IAM binding for role %s to project %s: %s", primaryRoleName, projectID, bindErr.Error()))
		} else {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Primary project IAM binding for role %s added to project: %s", primaryRoleName, projectID))
		}
	}

	if bindErr := AddIAMBinding(ctx, gcpClients, projectID, member, config.GCP_SA_DISCOVERY_ROLE); bindErr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Create] Failed to add primary project IAM binding for role %s to project %s: %s", config.GCP_SA_DISCOVERY_ROLE, projectID, bindErr.Error()))
	} else {
		tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Primary project IAM binding for role %s added to project: %s", config.GCP_SA_DISCOVERY_ROLE, projectID))
	}

	tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Create] IAM bindings created in %d projects for service account: %s", len(boundProjectIds), sa.Email))

	// Grant the node scan roles once on the folder or organization node so discovery and
	// read-only scanning cover every project under the node — including projects created later —
	// through IAM inheritance. Not applied to single-project integrations (no central management project).
	plan.NodeScanBindingResource = types.StringNull()
	var nodeScanRoles []string
	if !plan.NodeScanRoles.IsNull() && !plan.NodeScanRoles.IsUnknown() {
		if d := plan.NodeScanRoles.ElementsAs(ctx, &nodeScanRoles, false); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
	}

	nodeOnboarding := (!plan.CentralManagementProjectIDFolder.IsNull() && !plan.CentralManagementProjectIDFolder.IsUnknown()) ||
		(!plan.CentralManagementProjectIDOrg.IsNull() && !plan.CentralManagementProjectIDOrg.IsUnknown())
	if nodeOnboarding {
		nodeScanRoles = ensureDiscoveryRole(nodeScanRoles)
	}
	if len(nodeScanRoles) > 0 {
		if !plan.CentralManagementProjectIDFolder.IsNull() && !plan.CentralManagementProjectIDFolder.IsUnknown() {
			centralProjID := plan.CentralManagementProjectIDFolder.ValueString()
			ancestry, ancestryErr := gcpClients.CRMClient.Projects.GetAncestry(centralProjID, &cloudresourcemanager.GetAncestryRequest{}).Context(ctx).Do()
			if ancestryErr != nil {
				resp.Diagnostics.AddError(
					"[Service Account Key][Create] Failed to get folder ancestry",
					fmt.Sprintf("Error getting ancestry for node scan role binding: %s", ancestryErr.Error()),
				)
				return
			}
			var folderID string
			for _, ancestor := range ancestry.Ancestor {
				if ancestor.ResourceId.Type == config.PARENT_TYPE_FOLDER {
					folderID = ancestor.ResourceId.Id
					break
				}
			}
			if folderID != "" {
				for _, role := range nodeScanRoles {
					tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Granting node scan role %s on folder %s for service account %s", role, folderID, member))
					if bindErr := AddFolderIAMBinding(ctx, gcpClients, folderID, member, role); bindErr != nil {
						resp.Diagnostics.AddError(
							"[Service Account Key][Create] Failed to grant node scan role on folder",
							fmt.Sprintf("Error granting %s on folder %s: %s", role, folderID, bindErr.Error()),
						)
						return
					}
				}
				plan.NodeScanBindingResource = types.StringValue(fmt.Sprintf("folders/%s", folderID))
			}
		} else if !plan.CentralManagementProjectIDOrg.IsNull() && !plan.CentralManagementProjectIDOrg.IsUnknown() {
			centralProjID := plan.CentralManagementProjectIDOrg.ValueString()
			ancestry, ancestryErr := gcpClients.CRMClient.Projects.GetAncestry(centralProjID, &cloudresourcemanager.GetAncestryRequest{}).Context(ctx).Do()
			if ancestryErr != nil {
				resp.Diagnostics.AddError(
					"[Service Account Key][Create] Failed to get organization ancestry",
					fmt.Sprintf("Error getting ancestry for node scan role binding: %s", ancestryErr.Error()),
				)
				return
			}
			var orgID string
			for _, ancestor := range ancestry.Ancestor {
				if ancestor.ResourceId.Type == config.PARENT_TYPE_ORGANIZATION {
					orgID = ancestor.ResourceId.Id
					break
				}
			}
			if orgID != "" {
				for _, role := range nodeScanRoles {
					tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Create] Granting node scan role %s on organization %s for service account %s", role, orgID, member))
					if bindErr := AddOrgIAMBinding(ctx, gcpClients, orgID, member, role); bindErr != nil {
						resp.Diagnostics.AddError(
							"[Service Account Key][Create] Failed to grant node scan role on organization",
							fmt.Sprintf("Error granting %s on organization %s: %s", role, orgID, bindErr.Error()),
						)
						return
					}
				}
				plan.NodeScanBindingResource = types.StringValue(fmt.Sprintf("organizations/%s", orgID))
			}
		}
	}

	boundProjectsList, diags := types.ListValueFrom(ctx, types.StringType, boundProjectIds)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan.BoundProjects = boundProjectsList

	// Resolve project numbers for bound projects
	projectNumberMap, err := r.resolveProjectNumbers(ctx, gcpClients, boundProjectIds)
	if err != nil {
		resp.Diagnostics.AddError(
			"[Service Account Key][Create] Failed to resolve bound project numbers",
			err.Error(),
		)
		return
	}
	boundProjectNumbers := make([]string, 0, len(boundProjectIds))
	for _, pid := range boundProjectIds {
		boundProjectNumbers = append(boundProjectNumbers, projectNumberMap[pid])
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
	plan.BindingReconciliationRevision = types.Int64Null()

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

	// bound_projects tracks the discovered scope, which ModifyPlan reconciles on every plan. A missing
	// or unreadable binding must not shrink it here: downstream for_each would destroy that project's resources.
	for idx, projID := range stateBoundProjects {
		if IsSystemProjectID(projID) {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Read] Dropping system project from bound projects: %s", projID))
			continue
		}
		var projNum string
		if idx < len(stateBoundProjectNumbers) {
			projNum = stateBoundProjectNumbers[idx]
		}
		currentBoundProjects = append(currentBoundProjects, projID)
		// Keep a slot even when the number is unknown, so the two lists stay positionally
		// aligned; the gap is filled below.
		currentBoundProjectNumbers = append(currentBoundProjectNumbers, projNum)
	}

	// Repair any project number the state could not supply.
	if slices.Contains(currentBoundProjectNumbers, "") {
		tflog.Info(ctx, "[Service Account Key][Read] Re-resolving project number(s) missing from state")
		resolved, resolveErr := r.resolveProjectNumbers(ctx, gcpClients, currentBoundProjects)
		if resolveErr != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Could not resolve missing project number(s): %s", resolveErr))
		} else {
			for i, projID := range currentBoundProjects {
				if currentBoundProjectNumbers[i] == "" {
					currentBoundProjectNumbers[i] = resolved[projID]
				}
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

	// Verify the node scan role bindings still exist on the folder/organization node.
	if !state.NodeScanBindingResource.IsNull() && !state.NodeScanBindingResource.IsUnknown() {
		nodeResource := state.NodeScanBindingResource.ValueString()
		member := fmt.Sprintf("serviceAccount:%s", sa.Email)
		var nodeScanRoles []string
		if !state.NodeScanRoles.IsNull() && !state.NodeScanRoles.IsUnknown() {
			resp.Diagnostics.Append(state.NodeScanRoles.ElementsAs(ctx, &nodeScanRoles, false)...)
		}
		// Mirror Create: the node always carries roles/browser for a folder/org onboarding,
		// so drift-check and removal must account for it even if node_scan_roles was unset.
		nodeScanRoles = ensureDiscoveryRole(nodeScanRoles)
		if folderID, ok := strings.CutPrefix(nodeResource, "folders/"); ok {
			for _, role := range nodeScanRoles {
				bound, checkErr := HasFolderRoleBinding(ctx, gcpClients, folderID, member, role)
				if checkErr != nil {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Failed to check folder node scan binding for %s: %s", role, checkErr.Error()))
				} else if !bound {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Drift detected: node scan role %s missing on folder %s", role, folderID))
					state.NodeScanBindingResource = types.StringNull()
				}
			}
		} else if orgID, ok := strings.CutPrefix(nodeResource, "organizations/"); ok {
			for _, role := range nodeScanRoles {
				bound, checkErr := HasOrgRoleBinding(ctx, gcpClients, orgID, member, role)
				if checkErr != nil {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Failed to check organization node scan binding for %s: %s", role, checkErr.Error()))
				} else if !bound {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Read] Drift detected: node scan role %s missing on organization %s", role, orgID))
					state.NodeScanBindingResource = types.StringNull()
				}
			}
		}
	}

	if diags := resp.State.Set(ctx, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Read] Successfully read service account key resource: %s", sa.Email))
}

//nolint:gocyclo // Terraform CRUD - inherently complex with multi-project IAM update
func nodeScanRoleDelta(desired, previous []string) (toAdd, toRemove []string) {
	prevSet := make(map[string]bool, len(previous))
	for _, role := range previous {
		prevSet[role] = true
	}
	desiredSet := make(map[string]bool, len(desired))
	for _, role := range desired {
		desiredSet[role] = true
	}
	for _, role := range desired {
		if !prevSet[role] {
			toAdd = append(toAdd, role)
		}
	}
	for _, role := range previous {
		if !desiredSet[role] {
			toRemove = append(toRemove, role)
		}
	}
	return toAdd, toRemove
}

// ModifyPlan rediscovers the target projects on every plan so bound_projects adds newly created
// projects and drops deleted or excluded ones, with the change visible to downstream for_each.
func (r *ServiceAccountIntegration) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan, state serviceAccountIntegrationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if req.State.Raw.IsNull() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	revision := bindingReconciliationRevision(state.BindingReconciliationRevision, false)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("binding_reconciliation_revision"), revision)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update rediscovers once these are known.
	if plan.CentralManagementProjectIDFolder.IsUnknown() || plan.CentralManagementProjectIDOrg.IsUnknown() ||
		plan.ExcludeFreeTrialProjects.IsUnknown() || plan.ExcludeProjects.IsUnknown() {
		return
	}
	for _, excluded := range plan.ExcludeProjects.Elements() {
		if excluded.IsUnknown() {
			return
		}
	}

	gcpClients, diags := api.GetGCPClients(ctx, state.ProjectID.ValueString())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	projects, numbers, err := r.reconcileBoundProjects(ctx, gcpClients, &plan, &state)
	if err != nil {
		resp.Diagnostics.AddError("[Service Account Key][ModifyPlan] Failed to discover target projects", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("bound_projects"), projects)...)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("bound_project_numbers"), numbers)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Roles.IsNull() || plan.Roles.IsUnknown() || plan.PrimaryProjectRoles.IsUnknown() ||
		state.ServiceAccountEmail.IsNull() || state.ServiceAccountEmail.IsUnknown() ||
		state.ProjectID.IsNull() || state.ProjectID.IsUnknown() {
		return
	}
	if hasUnknownString(plan.Roles) || hasUnknownString(plan.PrimaryProjectRoles) {
		return
	}
	var roleNames, primaryRoleNames []string
	resp.Diagnostics.Append(plan.Roles.ElementsAs(ctx, &roleNames, false)...)
	if !plan.PrimaryProjectRoles.IsNull() {
		resp.Diagnostics.Append(plan.PrimaryProjectRoles.ElementsAs(ctx, &primaryRoleNames, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	member := fmt.Sprintf("serviceAccount:%s", state.ServiceAccountEmail.ValueString())
	missingProjects, checkFailures, err := inspectProjectBindingDrift(
		ctx, gcpClients, projects, roleNames, primaryRoleNames, state.ProjectID.ValueString(), member, cam.GCPServiceAccountSem,
	)
	if err != nil {
		resp.Diagnostics.AddError("[Service Account Key][ModifyPlan] Failed to check IAM bindings", err.Error())
		return
	}
	if len(missingProjects) > 0 {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("binding_reconciliation_revision"), bindingReconciliationRevision(revision, true))...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if len(checkFailures) > 0 {
		const maxFailureDetails = 5
		details := checkFailures
		if len(details) > maxFailureDetails {
			details = append(slices.Clone(details[:maxFailureDetails]), fmt.Sprintf("... and %d more", len(details)-maxFailureDetails))
		}
		resp.Diagnostics.AddWarning(
			"[Service Account Key][ModifyPlan] Unable to verify IAM bindings",
			fmt.Sprintf("Could not read IAM policies for %d project(s); those projects were not assumed to have missing bindings and will be checked again on a later plan. Read failures: %s",
				len(checkFailures), strings.Join(details, "; ")),
		)
	}
}

func bindingReconciliationRevision(current types.Int64, bindingsMissing bool) types.Int64 {
	if current.IsUnknown() {
		current = types.Int64Null()
	}
	if !bindingsMissing {
		return current
	}
	if current.IsNull() {
		return types.Int64Value(1)
	}
	return types.Int64Value(current.ValueInt64() + 1)
}

func hasUnknownString(value types.List) bool {
	if value.IsNull() || value.IsUnknown() {
		return value.IsUnknown()
	}
	for _, element := range value.Elements() {
		if element.IsUnknown() {
			return true
		}
	}
	return false
}

type projectBindingInspection struct {
	projectID    string
	missingRoles []string
	err          error
}

func inspectProjectBindingDrift(
	ctx context.Context,
	gcpClients *api.GCPClients,
	projects, roleNames, primaryRoleNames []string,
	primaryProjectID, member string,
	sem chan struct{},
) (missingProjects, failures []string, err error) {
	results := make(chan projectBindingInspection, len(projects))
	var wg sync.WaitGroup
	for _, projectID := range projects {
		expectedRoles := expectedProjectBindingRoles(projectID, primaryProjectID, roleNames, primaryRoleNames)
		if len(expectedRoles) == 0 {
			continue
		}
		wg.Add(1)
		go func(projectID string, expectedRoles []string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results <- projectBindingInspection{projectID: projectID, err: ctx.Err()}
				return
			}

			policy, getErr := getIAMPolicyV3(ctx, gcpClients, projectID)
			if getErr != nil {
				results <- projectBindingInspection{projectID: projectID, err: getErr}
				return
			}
			results <- projectBindingInspection{
				projectID:    projectID,
				missingRoles: missingUnconditionalRoleBindings(policy, expectedRoles, member),
			}
		}(projectID, expectedRoles)
	}
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", result.projectID, result.err))
			continue
		}
		if len(result.missingRoles) > 0 {
			missingProjects = append(missingProjects, result.projectID)
		}
	}
	slices.Sort(missingProjects)
	slices.Sort(failures)
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	return missingProjects, failures, nil
}

func expectedProjectBindingRoles(projectID, primaryProjectID string, roleNames, primaryRoleNames []string) []string {
	roles := make([]string, 0, len(roleNames)+len(primaryRoleNames)+1)
	addRole := func(roleName string) {
		if !slices.Contains(roles, roleName) {
			roles = append(roles, roleName)
		}
	}
	for _, roleName := range roleNames {
		addRole(projectRoleNameForProject(roleName, projectID))
	}
	if projectID == primaryProjectID {
		for _, roleName := range primaryRoleNames {
			addRole(roleName)
		}
		addRole(config.GCP_SA_DISCOVERY_ROLE)
	}
	return roles
}

func projectRoleNameForProject(roleName, projectID string) string {
	if !strings.HasPrefix(roleName, "projects/") {
		return roleName
	}
	parts := strings.Split(roleName, "/")
	if len(parts) >= 4 && parts[1] != projectID {
		return fmt.Sprintf("projects/%s/roles/%s", projectID, parts[3])
	}
	return roleName
}

// reconcileBoundProjects returns the discovered target projects and their numbers, ordered by
// mergeBoundProjects so an unchanged scope reproduces the prior lists exactly.
func (r *ServiceAccountIntegration) reconcileBoundProjects(
	ctx context.Context,
	gcpClients *api.GCPClients,
	plan, state *serviceAccountIntegrationResourceModel,
) (projects, numbers []string, err error) {
	discovered, err := r.discoverTargetProjects(ctx, gcpClients, plan, state.ProjectID.ValueString())
	if err != nil {
		return nil, nil, err
	}

	var priorProjects, priorNumbers []string
	if diags := state.BoundProjects.ElementsAs(ctx, &priorProjects, false); diags.HasError() {
		return nil, nil, fmt.Errorf("failed to read bound_projects from state")
	}
	if !state.BoundProjectNumbers.IsNull() {
		if diags := state.BoundProjectNumbers.ElementsAs(ctx, &priorNumbers, false); diags.HasError() {
			return nil, nil, fmt.Errorf("failed to read bound_project_numbers from state")
		}
	}

	projects, numbers = mergeBoundProjects(priorProjects, priorNumbers, discovered)

	var unresolved []string
	for i, projID := range projects {
		if numbers[i] == "" {
			unresolved = append(unresolved, projID)
		}
	}
	if len(unresolved) > 0 {
		resolved, resolveErr := r.resolveProjectNumbers(ctx, gcpClients, unresolved)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		for i, projID := range projects {
			if numbers[i] == "" {
				numbers[i] = resolved[projID]
			}
		}
	}
	return projects, numbers, nil
}

// mergeBoundProjects keeps prior projects still in scope in their prior order (with their known
// numbers), then appends newly discovered projects with an empty number for the caller to resolve.
func mergeBoundProjects(priorProjects, priorNumbers, discovered []string) (projects, numbers []string) {
	inScope := make(map[string]bool, len(discovered))
	for _, projID := range discovered {
		inScope[projID] = true
	}

	projects = make([]string, 0, len(discovered))
	numbers = make([]string, 0, len(discovered))
	seen := make(map[string]bool, len(discovered))
	for i, projID := range priorProjects {
		if !inScope[projID] || seen[projID] {
			continue
		}
		seen[projID] = true
		var projNum string
		if i < len(priorNumbers) {
			projNum = priorNumbers[i]
		}
		projects = append(projects, projID)
		numbers = append(numbers, projNum)
	}
	for _, projID := range discovered {
		if seen[projID] {
			continue
		}
		seen[projID] = true
		projects = append(projects, projID)
		numbers = append(numbers, "")
	}
	return projects, numbers
}

// ensureProjectRoleBindings idempotently grants the service account its roles on one project,
// replicating (or restoring) project-scoped custom roles first. It returns one entry per failure.
func ensureProjectRoleBindings(
	ctx context.Context,
	gcpClients *api.GCPClients,
	projID, member string,
	roleNames, primaryRoleNames []string,
	primaryProjectID string,
	sourceRoles map[string]*iam.Role,
) []string {
	var failures []string
	for _, roleName := range roleNames {
		actualRoleName := projectRoleNameForProject(roleName, projID)
		if actualRoleName != roleName {
			parts := strings.Split(roleName, "/")
			if err := ensureReplicatedCustomRole(ctx, gcpClients, sourceRoles[roleName], projID, parts[3]); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %s", actualRoleName, err))
				continue
			}
		}
		if err := AddIAMBinding(ctx, gcpClients, projID, member, actualRoleName); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", actualRoleName, err))
		}
	}

	if projID != primaryProjectID {
		return failures
	}
	for _, roleName := range append(slices.Clone(primaryRoleNames), config.GCP_SA_DISCOVERY_ROLE) {
		if err := AddIAMBinding(ctx, gcpClients, projID, member, roleName); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", roleName, err))
		}
	}
	return failures
}

// ensureReplicatedCustomRole makes sure the copy of sourceRole exists and is active in projID.
func ensureReplicatedCustomRole(ctx context.Context, gcpClients *api.GCPClients, sourceRole *iam.Role, projID, roleID string) error {
	targetRoleName := fmt.Sprintf("projects/%s/roles/%s", projID, roleID)
	existing, err := gcpClients.IAMClient.Projects.Roles.Get(targetRoleName).Context(ctx).Do()
	if err == nil {
		if !existing.Deleted {
			return nil
		}
		_, err = gcpClients.IAMClient.Projects.Roles.Undelete(targetRoleName, &iam.UndeleteRoleRequest{Etag: existing.Etag}).Context(ctx).Do()
		return err
	}
	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "not found") {
		return err
	}
	if sourceRole == nil {
		return fmt.Errorf("source custom role is unavailable to replicate")
	}
	return createReplicatedCustomRole(ctx, gcpClients, sourceRole, projID, roleID)
}

func createReplicatedCustomRole(ctx context.Context, gcpClients *api.GCPClients, sourceRole *iam.Role, projID, roleID string) error {
	_, err := gcpClients.IAMClient.Projects.Roles.Create(fmt.Sprintf("projects/%s", projID), &iam.CreateRoleRequest{
		RoleId: roleID,
		Role: &iam.Role{
			Title:               sourceRole.Title,
			Description:         fmt.Sprintf("%s (replicated for multi-project service account)", sourceRole.Description),
			IncludedPermissions: sourceRole.IncludedPermissions,
			Stage:               sourceRole.Stage,
		},
	}).Context(ctx).Do()
	return err
}

func (r *ServiceAccountIntegration) reconcileNodeScanRoles(ctx context.Context, gcpClients *api.GCPClients, plan, state *serviceAccountIntegrationResourceModel) (types.String, error) {
	folderScoped := !plan.CentralManagementProjectIDFolder.IsNull() && !plan.CentralManagementProjectIDFolder.IsUnknown()
	orgScoped := !plan.CentralManagementProjectIDOrg.IsNull() && !plan.CentralManagementProjectIDOrg.IsUnknown()
	if !folderScoped && !orgScoped {
		return state.NodeScanBindingResource, nil
	}

	toStrings := func(list types.List) ([]string, error) {
		var out []string
		if list.IsNull() || list.IsUnknown() {
			return out, nil
		}
		if d := list.ElementsAs(ctx, &out, false); d.HasError() {
			var msgs []string
			for _, diagnostic := range d {
				if diagnostic.Summary() != "" {
					msgs = append(msgs, diagnostic.Summary())
				}
				if diagnostic.Detail() != "" {
					msgs = append(msgs, diagnostic.Detail())
				}
			}
			return nil, fmt.Errorf("could not read node scan roles: %s", strings.Join(msgs, "; "))
		}
		return out, nil
	}
	desired, err := toStrings(plan.NodeScanRoles)
	if err != nil {
		return state.NodeScanBindingResource, err
	}
	previous, err := toStrings(state.NodeScanRoles)
	if err != nil {
		return state.NodeScanBindingResource, err
	}
	// The discovery role is always bound at the node while node onboarding, so keep
	// it on both sides to avoid revoking it when node_scan_roles is emptied.
	desired = ensureDiscoveryRole(desired)
	previous = ensureDiscoveryRole(previous)
	toAdd, toRemove := nodeScanRoleDelta(desired, previous)
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return state.NodeScanBindingResource, nil
	}

	centralProjID := plan.CentralManagementProjectIDOrg.ValueString()
	if folderScoped {
		centralProjID = plan.CentralManagementProjectIDFolder.ValueString()
	}
	ancestry, ancestryErr := gcpClients.CRMClient.Projects.GetAncestry(centralProjID, &cloudresourcemanager.GetAncestryRequest{}).Context(ctx).Do()
	if ancestryErr != nil {
		return state.NodeScanBindingResource, fmt.Errorf("failed to get ancestry for node scan role binding: %w", ancestryErr)
	}

	member := fmt.Sprintf("serviceAccount:%s", state.ServiceAccountEmail.ValueString())

	if folderScoped {
		var folderID string
		for _, ancestor := range ancestry.Ancestor {
			if ancestor.ResourceId.Type == config.PARENT_TYPE_FOLDER {
				folderID = ancestor.ResourceId.Id
				break
			}
		}
		if folderID == "" {
			return state.NodeScanBindingResource, nil
		}
		for _, role := range toAdd {
			if bindErr := AddFolderIAMBinding(ctx, gcpClients, folderID, member, role); bindErr != nil {
				return state.NodeScanBindingResource, fmt.Errorf("error granting %s on folder %s: %w", role, folderID, bindErr)
			}
		}
		for _, role := range toRemove {
			if unbindErr := RemoveFolderIAMBinding(ctx, gcpClients, folderID, member, role); unbindErr != nil {
				tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Update] Failed to revoke node scan role %s on folder %s: %s", role, folderID, unbindErr.Error()))
			}
		}
		return types.StringValue(fmt.Sprintf("folders/%s", folderID)), nil
	}

	var orgID string
	for _, ancestor := range ancestry.Ancestor {
		if ancestor.ResourceId.Type == config.PARENT_TYPE_ORGANIZATION {
			orgID = ancestor.ResourceId.Id
			break
		}
	}
	if orgID == "" {
		return state.NodeScanBindingResource, nil
	}
	for _, role := range toAdd {
		if bindErr := AddOrgIAMBinding(ctx, gcpClients, orgID, member, role); bindErr != nil {
			return state.NodeScanBindingResource, fmt.Errorf("error granting %s on organization %s: %w", role, orgID, bindErr)
		}
	}
	for _, role := range toRemove {
		if unbindErr := RemoveOrgIAMBinding(ctx, gcpClients, orgID, member, role); unbindErr != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Update] Failed to revoke node scan role %s on organization %s: %s", role, orgID, unbindErr.Error()))
		}
	}
	return types.StringValue(fmt.Sprintf("organizations/%s", orgID)), nil
}

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
	if plan.BindingReconciliationRevision.IsUnknown() {
		plan.BindingReconciliationRevision = state.BindingReconciliationRevision
	}
	if plan.BindingReconciliationRevision.IsUnknown() {
		plan.BindingReconciliationRevision = types.Int64Null()
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

	// ModifyPlan normally supplies the target projects; rediscover only if they were unknown at plan time.
	var targetProjects, targetProjectNumbers []string
	if plan.BoundProjects.IsUnknown() || plan.BoundProjectNumbers.IsUnknown() {
		var err error
		targetProjects, targetProjectNumbers, err = r.reconcileBoundProjects(ctx, gcpClients, &plan, &state)
		if err != nil {
			resp.Diagnostics.AddError("[Service Account Key][Update] Failed to discover target projects", err.Error())
			return
		}
	} else {
		resp.Diagnostics.Append(plan.BoundProjects.ElementsAs(ctx, &targetProjects, false)...)
		resp.Diagnostics.Append(plan.BoundProjectNumbers.ElementsAs(ctx, &targetProjectNumbers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var oldBoundProjects, roleNames, primaryRoleNames []string
	resp.Diagnostics.Append(state.BoundProjects.ElementsAs(ctx, &oldBoundProjects, false)...)
	resp.Diagnostics.Append(state.Roles.ElementsAs(ctx, &roleNames, false)...)
	if !state.PrimaryProjectRoles.IsNull() {
		resp.Diagnostics.Append(state.PrimaryProjectRoles.ElementsAs(ctx, &primaryRoleNames, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	member := fmt.Sprintf("serviceAccount:%s", state.ServiceAccountEmail.ValueString())
	primaryProjectID := state.ProjectID.ValueString()

	sourceRoles := make(map[string]*iam.Role)
	for _, roleName := range roleNames {
		if !strings.HasPrefix(roleName, "projects/") {
			continue
		}
		sourceRole, getErr := gcpClients.IAMClient.Projects.Roles.Get(roleName).Context(ctx).Do()
		if getErr != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Update] Failed to get custom role %s: %s", roleName, getErr.Error()))
			continue
		}
		sourceRoles[roleName] = sourceRole
	}

	{
		var bindWg sync.WaitGroup
		var bindMu sync.Mutex
		bindFailures := map[string][]string{}
		sem := cam.GCPServiceAccountSem
		for _, proj := range oldBoundProjects {
			if slices.Contains(targetProjects, proj) {
				continue
			}
			bindWg.Add(1)
			go func(projID string) {
				defer bindWg.Done()
				sem <- struct{}{}        // acquire slot
				defer func() { <-sem }() // release slot
				r.removeProjectRoleBindings(ctx, gcpClients, projID, member, roleNames, primaryRoleNames, primaryProjectID)
			}(proj)
		}
		// Re-granting every target project also restores bindings removed outside Terraform.
		for _, proj := range targetProjects {
			bindWg.Add(1)
			go func(projID string) {
				defer bindWg.Done()
				sem <- struct{}{}        // acquire slot
				defer func() { <-sem }() // release slot
				if failures := ensureProjectRoleBindings(ctx, gcpClients, projID, member, roleNames, primaryRoleNames, primaryProjectID, sourceRoles); len(failures) > 0 {
					bindMu.Lock()
					bindFailures[projID] = failures
					bindMu.Unlock()
				}
			}(proj)
		}
		bindWg.Wait()

		if len(bindFailures) > 0 {
			failedProjects := slices.Sorted(maps.Keys(bindFailures))
			details := make([]string, 0, len(failedProjects))
			for _, projID := range failedProjects {
				details = append(details, fmt.Sprintf("%s:\n    - %s", projID, strings.Join(bindFailures[projID], "\n    - ")))
			}
			resp.Diagnostics.AddWarning(
				"[Service Account Key][Update] Some IAM bindings could not be granted",
				fmt.Sprintf("These projects remain in bound_projects and will be retried on the next update:\n\n  %s", strings.Join(details, "\n  ")),
			)
		}
	}

	boundProjectsList, diags := types.ListValueFrom(ctx, types.StringType, targetProjects)
	resp.Diagnostics.Append(diags...)
	boundProjectNumbersList, diags := types.ListValueFrom(ctx, types.StringType, targetProjectNumbers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.BoundProjects = boundProjectsList
	plan.BoundProjectNumbers = boundProjectNumbersList

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

	if !plan.NodeScanRoles.Equal(state.NodeScanRoles) {
		bindingResource, err := r.reconcileNodeScanRoles(ctx, gcpClients, &plan, &state)
		if err != nil {
			resp.Diagnostics.AddError(
				"[Service Account Key][Update] Failed to reconcile node scan roles",
				err.Error(),
			)
			return
		}
		plan.NodeScanBindingResource = bindingResource
	} else {
		plan.NodeScanBindingResource = state.NodeScanBindingResource
	}

	if diags := resp.State.Set(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Update] Successfully updated service account key resource: %s", state.ServiceAccountEmail.ValueString()))
}

func (r *ServiceAccountIntegration) removeProjectRoleBindings(ctx context.Context, gcpClients *api.GCPClients, projID, member string, roleNames, primaryRoleNames []string, primaryProjectID string) {
	tflog.Info(ctx, fmt.Sprintf("[Service Account Key][Delete] Removing service account principal from project: %s", projID))
	for _, roleName := range roleNames {
		actualRoleName := projectRoleNameForProject(roleName, projID)
		if actualRoleName != roleName {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Using replicated role name for removal: %s", actualRoleName))
		}
		if removeErr := RemoveIAMBinding(ctx, gcpClients, projID, member, actualRoleName); removeErr != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to remove IAM binding for role %s from project %s: %s", actualRoleName, projID, removeErr.Error()))
		} else {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] IAM binding for role %s removed from project: %s", actualRoleName, projID))
		}
	}

	if projID != primaryProjectID {
		return
	}
	for _, primaryRoleName := range primaryRoleNames {
		if removeErr := RemoveIAMBinding(ctx, gcpClients, projID, member, primaryRoleName); removeErr != nil {
			tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to remove primary project IAM binding for role %s from project %s: %s", primaryRoleName, projID, removeErr.Error()))
		} else {
			tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Primary project IAM binding for role %s removed from project: %s", primaryRoleName, projID))
		}
	}
	if removeErr := RemoveIAMBinding(ctx, gcpClients, projID, member, config.GCP_SA_DISCOVERY_ROLE); removeErr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to remove primary project IAM binding for role %s from project %s: %s", config.GCP_SA_DISCOVERY_ROLE, projID, removeErr.Error()))
	}
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

	// Get primary project roles from state
	var primaryRoleNames []string
	if !state.PrimaryProjectRoles.IsNull() {
		if roleDiags := state.PrimaryProjectRoles.ElementsAs(ctx, &primaryRoleNames, false); roleDiags.HasError() {
			resp.Diagnostics.Append(roleDiags...)
			return
		}
	}

	primaryProjectID := state.ProjectID.ValueString()

	// Remove all role bindings from all target projects (concurrent)
	{
		var deleteBindWg sync.WaitGroup
		sem := cam.GCPServiceAccountSem
		for _, projectID := range currentTargetProjects {
			deleteBindWg.Add(1)
			go func(projID string) {
				defer deleteBindWg.Done()
				sem <- struct{}{}        // acquire slot
				defer func() { <-sem }() // release slot

				r.removeProjectRoleBindings(ctx, gcpClients, projID, member, roleNames, primaryRoleNames, primaryProjectID)
			}(projectID)
		}
		deleteBindWg.Wait()
	}

	// Delete replicated custom roles from all bound projects (concurrent)
	// Organization-level custom roles are not replicated, so we only need to clean up project-scoped custom roles
	{
		var deleteRoleWg sync.WaitGroup
		sem := cam.GCPServiceAccountSem
		for _, roleName := range roleNames {
			if strings.HasPrefix(roleName, "organizations/") {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Skipping organization-level custom role: %s (not replicated)", roleName))
				continue
			}

			if strings.HasPrefix(roleName, "projects/") {
				parts := strings.Split(roleName, "/")
				if len(parts) >= 4 {
					sourceProjectID := parts[1]
					roleID := parts[3]

					for _, projectID := range currentTargetProjects {
						if projectID == sourceProjectID {
							tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Skipping custom role deletion in source project: %s", projectID))
							continue
						}

						deleteRoleWg.Add(1)
						go func(projID, rID string) {
							defer deleteRoleWg.Done()
							sem <- struct{}{}        // acquire slot
							defer func() { <-sem }() // release slot

							replicatedRoleName := fmt.Sprintf("projects/%s/roles/%s", projID, rID)
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
						}(projectID, roleID)
					}
				}
			}
		}
		deleteRoleWg.Wait()
	}

	// Remove the node scan roles from the folder or organization node
	if !state.NodeScanBindingResource.IsNull() && !state.NodeScanBindingResource.IsUnknown() {
		nodeResource := state.NodeScanBindingResource.ValueString()
		deleteMember := fmt.Sprintf("serviceAccount:%s", state.ServiceAccountEmail.ValueString())
		var nodeScanRoles []string
		if !state.NodeScanRoles.IsNull() && !state.NodeScanRoles.IsUnknown() {
			resp.Diagnostics.Append(state.NodeScanRoles.ElementsAs(ctx, &nodeScanRoles, false)...)
		}
		// Mirror Create: the node always carries roles/browser for a folder/org onboarding,
		// so drift-check and removal must account for it even if node_scan_roles was unset.
		nodeScanRoles = ensureDiscoveryRole(nodeScanRoles)
		if folderID, ok := strings.CutPrefix(nodeResource, "folders/"); ok {
			for _, role := range nodeScanRoles {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Removing node scan role %s from folder %s", role, folderID))
				if removeErr := RemoveFolderIAMBinding(ctx, gcpClients, folderID, deleteMember, role); removeErr != nil {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to remove node scan role %s from folder %s: %s", role, folderID, removeErr.Error()))
				}
			}
		} else if orgID, ok := strings.CutPrefix(nodeResource, "organizations/"); ok {
			for _, role := range nodeScanRoles {
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key][Delete] Removing node scan role %s from organization %s", role, orgID))
				if removeErr := RemoveOrgIAMBinding(ctx, gcpClients, orgID, deleteMember, role); removeErr != nil {
					tflog.Warn(ctx, fmt.Sprintf("[Service Account Key][Delete] Failed to remove node scan role %s from organization %s: %s", role, orgID, removeErr.Error()))
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

						// Filter out system-created projects (sys-*)
						if IsSystemProject(project) {
							tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Excluding system project: %s", project.ProjectId))
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

						// Filter out system-created projects (sys-*)
						if IsSystemProject(project) {
							tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Excluding system project: %s", project.ProjectId))
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

					// Filter out system-created projects (sys-*)
					if IsSystemProject(project) {
						tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Excluding system project: %s", project.ProjectId))
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

// resolveProjectNumbers maps each project ID to its GCP project number.
func (r *ServiceAccountIntegration) resolveProjectNumbers(ctx context.Context, gcpClients *api.GCPClients, projectIDs []string) (map[string]string, error) {
	result := make(map[string]string)
	failures := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := cam.GCPServiceAccountSem
	for _, pid := range projectIDs {
		wg.Add(1)
		go func(projectID string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot

			var proj *cloudresourcemanager.Project
			var err error
			deadline := time.Now().Add(gcpKeyPropagationWaitTimeout)
			for attempt := 1; ; attempt++ {
				proj, err = gcpClients.CRMClient.Projects.Get(projectID).Context(ctx).Do()
				if err == nil {
					break
				}
				if !isGCPKeyPropagationError(err) || time.Now().After(deadline) {
					break
				}
				tflog.Debug(ctx, fmt.Sprintf("[Service Account Key] Project %s not readable yet (attempt %d), retrying in %s: %s",
					projectID, attempt, gcpKeyPropagationPollInterval, err))
				select {
				case <-ctx.Done():
					err = ctx.Err()
				case <-time.After(gcpKeyPropagationPollInterval):
					continue
				}
				break
			}

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures[projectID] = err
				return
			}
			result[projectID] = fmt.Sprintf("%d", proj.ProjectNumber)
		}(pid)
	}
	wg.Wait()

	if len(failures) > 0 {
		ids := make([]string, 0, len(failures))
		for id := range failures {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		details := make([]string, 0, len(ids))
		for _, id := range ids {
			details = append(details, fmt.Sprintf("%s: %s", id, failures[id]))
		}
		return nil, fmt.Errorf("could not resolve the project number for %d of %d bound project(s): %s",
			len(failures), len(projectIDs), strings.Join(details, "; "))
	}
	return result, nil
}
