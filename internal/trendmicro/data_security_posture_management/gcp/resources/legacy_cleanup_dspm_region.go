package resources

import (
	"context"
	"fmt"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/data_security_posture_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/option"
)

// stringListFromSlice builds a known-non-null types.List; ListValueFrom can normalize empty to null, tripping TF consistency.
func stringListFromSlice(s []string) types.List {
	elems := make([]attr.Value, 0, len(s))
	for _, v := range s {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

var _ resource.Resource = &LegacyCleanupDSPMRegion{}
var _ resource.ResourceWithModifyPlan = &LegacyCleanupDSPMRegion{}

type LegacyCleanupDSPMRegion struct{}

type legacyCleanupDSPMRegionModel struct {
	ID                       types.String `tfsdk:"id"`
	ProjectID                types.String `tfsdk:"project_id"`
	Region                   types.String `tfsdk:"region"`
	Stage                    types.String `tfsdk:"stage"`
	ServiceAccountKey        types.String `tfsdk:"service_account_key"`
	SnapshotDiskBeforeDelete types.Bool   `tfsdk:"snapshot_disk_before_delete"`
	StateBucket              types.String `tfsdk:"state_bucket"`

	NamePrefix         types.String `tfsdk:"name_prefix"`
	SnapshotName       types.String `tfsdk:"snapshot_name"`
	ResourcesDeleted   types.Map    `tfsdk:"resources_deleted"`
	ResourcesPreserved types.Map    `tfsdk:"resources_preserved"`
	OrphanBucketNames  types.List   `tfsdk:"orphan_bucket_names"`
	DeletionTimestamp  types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus      types.String `tfsdk:"cleanup_status"`
	CleanupError       types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupDSPMRegion() resource.Resource {
	return &LegacyCleanupDSPMRegion{}
}

func (r *LegacyCleanupDSPMRegion) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_DSPM_REGION
}

func (r *LegacyCleanupDSPMRegion) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deletes the per-region DSPM resources created by the legacy Terraform Package Solution in a single GCP project, so a Terraform Provider deployment can reuse the same name prefix. Each instance is keyed by `(project_id, region)`. Deletion order matches the original local-exec bash: eventarc triggers → functions / run services → schedulers → disk (snapshot first if requested) + resource policy → VMs → VPC connector → firewall rules → NAT → router → subnet → VPC. Returns `cleanup_status = \"not_found\"` if no matching legacy resources exist in the region.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`{project_id}/{region}`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID whose legacy DSPM resources should be cleaned up.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The GCP region to clean up (e.g. `us-east1`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"stage": schema.StringAttribute{
				MarkdownDescription: "DSPM stage the legacy Package deployment was rolled out under. One of `int`, `stg`, `prod`. The legacy resource name prefix becomes `dspm-{i|s|p}-{region_abbr}`, derived from this value.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("int", "stg", "prod"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_key": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded JSON service account key used to authenticate with GCP for cleanup operations. Optional — three common patterns:\n" +
					"\n" +
					"- **CAM-integrated** (recommended): set to `visionone_cam_service_account_integration.comprehensive.private_key`. The CAM-minted SA (with IAM bindings granted in the same plan) is used without any customer-side key management.\n" +
					"- **BYO key**: set to a base64-encoded JSON key for any service account with delete permissions on the legacy DSPM resources. Use this when operator policy forbids using the CAM-minted SA or ADC for delete operations (e.g. enterprise-managed credentials, scope-limited audit trail).\n" +
					"- **ADC**: omit the attribute entirely. The provider falls back to Application Default Credentials (gcloud, workload identity, GCE metadata).",
				Optional:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"snapshot_disk_before_delete": schema.BoolAttribute{
				MarkdownDescription: "When true (default), the persistent scan-job disk is snapshotted as `{name_prefix}-disk-pre-upgrade` before deletion. Keep enabled so main-app can migrate scan data on first boot of the new stack.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"state_bucket": schema.StringAttribute{
				MarkdownDescription: "GCS bucket holding the *current* Provider-mode Terraform state for this deployment " +
					"(read from `gs://{state_bucket}/terraform.tfstate/default.tfstate`). When set, cleanup checks " +
					"each candidate resource against this state before deleting it, and skips anything already " +
					"tracked there — this is what prevents a forced replacement of this resource (e.g. a " +
					"`bound_projects` change rotating the CAM service account key) from deleting infrastructure " +
					"that a Provider-mode-to-Provider-mode migration is still actively using via a shared state file. " +
					"Omit for a legacy Package-mode migration, where no Provider-mode state exists yet and the " +
					"unconditional-delete behavior is safe (see `visionone_dspm_legacy_state_regions` for that path). " +
					"If the state object can't be read for a reason other than \"doesn't exist yet\" (e.g. a " +
					"permissions error), cleanup fails closed — it reports `cleanup_status = \"failed\"` rather than " +
					"silently deleting as if nothing were tracked.",
				Optional: true,
			},
			"name_prefix": schema.StringAttribute{
				MarkdownDescription: "The computed legacy resource prefix (e.g. `dspm-i-use1`).",
				Computed:            true,
			},
			"snapshot_name": schema.StringAttribute{
				MarkdownDescription: "The disk snapshot name created before disk deletion (empty if no disk existed or snapshot was disabled).",
				Computed:            true,
			},
			"resources_deleted": schema.MapAttribute{
				MarkdownDescription: "Count of legacy resources deleted, keyed by resource family (functions, triggers, schedulers, run_services, vms, firewalls, router_nats, routers, subnets, vpcs, connectors, disks, snapshots, resource_policies, sinks, alert_policies, dashboards, orphan_buckets_preserved, orphan_bindings).",
				ElementType:         types.Int64Type,
				Computed:            true,
			},
			"resources_preserved": schema.MapAttribute{
				MarkdownDescription: "Count of candidate resources intentionally **not** deleted because `state_bucket` " +
					"lookup found them already tracked in the current Provider-mode state, keyed by the same resource " +
					"family names used in `resources_deleted` (only families that can be state-checked appear: " +
					"firewalls, router_nats, routers, subnets, vpcs, connectors, disks, resource_policies, sinks, " +
					"alert_policies, dashboards). Empty when `state_bucket` is unset.",
				ElementType: types.Int64Type,
				Computed:    true,
			},
			"orphan_bucket_names": schema.ListAttribute{
				MarkdownDescription: "GCS bucket names that pre-existed for this (project, region) tuple and were intentionally **not** deleted by cleanup. Audit-log buckets are data-preservation-sensitive, and deleting them races GCP's audit-log forwarding pipeline. Consume this list from the downstream new-module via `import { for_each = ... }` blocks to adopt the buckets into the new state. Empty on fresh installs.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"deletion_timestamp": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when cleanup was performed.",
				Computed:            true,
			},
			"cleanup_status": schema.StringAttribute{
				MarkdownDescription: "Status: `deleted`, `partial`, `not_found`, or `failed`.",
				Computed:            true,
			},
			"cleanup_error": schema.StringAttribute{
				MarkdownDescription: "Error message if cleanup encountered failures.",
				Computed:            true,
			},
		},
	}
}

func (r *LegacyCleanupDSPMRegion) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupDSPMRegionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	region := plan.Region.ValueString()
	stageLetter := stageNameToLetter(plan.Stage.ValueString())
	regionAbbr := regionAbbreviation(region)
	namePrefix := fmt.Sprintf("%s%s-%s", config.LEGACY_GCP_DSPM_NAME_BASE, stageLetter, regionAbbr)

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", projectID, region))
	plan.NamePrefix = types.StringValue(namePrefix)
	plan.SnapshotName = types.StringValue("")
	plan.DeletionTimestamp = types.StringValue("")
	plan.CleanupError = types.StringValue("")

	var clientOptions []option.ClientOption
	var saEmail string
	if key := plan.ServiceAccountKey.ValueString(); key != "" {
		opt, err := newClientOptionFromEncodedServiceAccountKey(ctx, key)
		if err != nil {
			resp.Diagnostics.AddError("[DSPM Region Cleanup] Invalid service account key", err.Error())
			return
		}
		clientOptions = append(clientOptions, opt)
		// SA email feeds the orphan-binding janitor; on parse failure
		// continue without it (key is still usable for cleanup ops).
		if email, err := saEmailFromEncodedKey(key); err != nil {
			tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] could not extract SA email for janitor: %v", err))
		} else {
			saEmail = email
		}
	}

	tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] start project=%s region=%s prefix=%s", projectID, region, namePrefix))

	result, err := runDSPMRegionCleanup(ctx, dspmRegionCleanupOptions{
		ProjectID:                projectID,
		Region:                   region,
		NamePrefix:               namePrefix,
		SnapshotDiskBeforeDelete: plan.SnapshotDiskBeforeDelete.ValueBool(),
		ClientOptions:            clientOptions,
		SAEmail:                  saEmail,
		StateBucket:              plan.StateBucket.ValueString(),
	})

	resourcesDeleted, diag := types.MapValueFrom(ctx, types.Int64Type, result.ResourcesDeleted)
	resp.Diagnostics.Append(diag...)
	plan.ResourcesDeleted = resourcesDeleted

	resourcesPreserved, diag := types.MapValueFrom(ctx, types.Int64Type, result.ResourcesPreserved)
	resp.Diagnostics.Append(diag...)
	plan.ResourcesPreserved = resourcesPreserved

	// Always known-non-null — root module's `import { for_each }` rejects unknown / null.
	plan.OrphanBucketNames = stringListFromSlice(result.OrphanBuckets)

	plan.SnapshotName = types.StringValue(result.SnapshotName)
	plan.DeletionTimestamp = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	deletedCount := totalDeleted(result.ResourcesDeleted)
	switch {
	case err != nil && deletedCount > 0:
		plan.CleanupStatus = types.StringValue("partial")
		plan.CleanupError = types.StringValue(err.Error())
	case err != nil:
		plan.CleanupStatus = types.StringValue("failed")
		plan.CleanupError = types.StringValue(err.Error())
	case deletedCount == 0:
		plan.CleanupStatus = types.StringValue("not_found")
	default:
		plan.CleanupStatus = types.StringValue("deleted")
	}

	tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] done project=%s region=%s status=%s", projectID, region, plan.CleanupStatus.ValueString()))

	// Persist before hard-stop so operator can inspect cleanup_* attrs.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	// Fail fast — letting apply continue surfaces confusing 409s downstream.
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("[DSPM Region Cleanup] cleanup %s for project=%s region=%s", plan.CleanupStatus.ValueString(), projectID, region),
			fmt.Sprintf("%s\n\nResolve the listed resources manually (or via gcloud) and re-run `terraform apply`.", err.Error()),
		)
	}
}

// ModifyPlan probes GCP for orphan buckets at plan time; TF forbids unknown for_each. Uses ADC (SA key may be unknown). Failure → empty list.
func (r *LegacyCleanupDSPMRegion) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan legacyCleanupDSPMRegionModel
	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if !req.State.Raw.IsNull() {
		var state legacyCleanupDSPMRegionModel
		if diags := req.State.Get(ctx, &state); !diags.HasError() && !state.OrphanBucketNames.IsNull() && !state.OrphanBucketNames.IsUnknown() {
			plan.OrphanBucketNames = state.OrphanBucketNames
			resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
			return
		}
	}

	if plan.ProjectID.IsUnknown() || plan.Region.IsUnknown() || plan.Stage.IsUnknown() {
		return
	}

	buckets, err := probeOrphanBuckets(
		ctx,
		plan.ProjectID.ValueString(),
		plan.Region.ValueString(),
		plan.Stage.ValueString(),
	)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] ModifyPlan probe failed (ADC required): %v", err))
		buckets = nil
	}
	plan.OrphanBucketNames = stringListFromSlice(buckets)
	resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
}

func (r *LegacyCleanupDSPMRegion) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupDSPMRegionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupDSPMRegion) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state legacyCleanupDSPMRegionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var plan legacyCleanupDSPMRegionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ServiceAccountKey = plan.ServiceAccountKey
	state.SnapshotDiskBeforeDelete = plan.SnapshotDiskBeforeDelete
	state.StateBucket = plan.StateBucket
	// Preserve plan's OrphanBucketNames (set by ModifyPlan) for TF plan/state consistency on second apply.
	if !plan.OrphanBucketNames.IsNull() && !plan.OrphanBucketNames.IsUnknown() {
		state.OrphanBucketNames = plan.OrphanBucketNames
	} else if state.OrphanBucketNames.IsNull() || state.OrphanBucketNames.IsUnknown() {
		state.OrphanBucketNames = stringListFromSlice(nil)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupDSPMRegion) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op: removing from state does not undo legacy GCP deletions; matches legacy_cleanup_* family.
	_ = resp
}

// stageNameToLetter maps the public stage to the legacy bash's i/s/p prefix token.
func stageNameToLetter(stage string) string {
	switch stage {
	case "int":
		return "i"
	case "stg":
		return "s"
	case "prod":
		return "p"
	}
	return stage
}

func totalDeleted(counts map[string]int) int {
	sum := 0
	for _, v := range counts {
		sum += v
	}
	return sum
}
