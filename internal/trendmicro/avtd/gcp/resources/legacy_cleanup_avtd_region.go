package resources

import (
	"context"
	"fmt"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/avtd/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/option"
)

func stringListFromSlice(s []string) types.List {
	elems := make([]attr.Value, 0, len(s))
	for _, v := range s {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

var _ resource.Resource = &LegacyCleanupAVTDRegion{}
var _ resource.ResourceWithModifyPlan = &LegacyCleanupAVTDRegion{}

type LegacyCleanupAVTDRegion struct{}

type legacyCleanupAVTDRegionModel struct {
	ID                     types.String `tfsdk:"id"`
	ProjectID              types.String `tfsdk:"project_id"`
	SidecarProjectID       types.String `tfsdk:"sidecar_project_id"`
	Region                 types.String `tfsdk:"region"`
	Stage                  types.String `tfsdk:"stage"`
	ServiceAccountKey      types.String `tfsdk:"service_account_key"`
	ResourcePrefixes       types.List   `tfsdk:"resource_prefixes"`
	PreserveResourceBucket types.Bool   `tfsdk:"preserve_resource_bucket"`
	PreserveVPC            types.Bool   `tfsdk:"preserve_vpc"`
	PreserveFirestore      types.Bool   `tfsdk:"preserve_firestore"`

	NamePrefixes      types.List   `tfsdk:"name_prefixes"`
	ResourcesDeleted  types.Map    `tfsdk:"resources_deleted"`
	OrphanBucketNames types.List   `tfsdk:"orphan_bucket_names"`
	DeletionTimestamp types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus     types.String `tfsdk:"cleanup_status"`
	CleanupError      types.String `tfsdk:"cleanup_error"`
}

func NewLegacyCleanupAVTDRegion() resource.Resource {
	return &LegacyCleanupAVTDRegion{}
}

func (r *LegacyCleanupAVTDRegion) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_LEGACY_CLEANUP_AVTD_REGION
}

func (r *LegacyCleanupAVTDRegion) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deletes the per-region AVTD (cloud-sentry) resources created by the legacy Terraform Package Solution in a single GCP project, so a Terraform-provider deployment can reuse the same name prefixes. Each instance is keyed by `(project_id, region)`. Resources are discovered by LISTing each family and matching the configured `resource_prefixes` (default `v1avtd`/`v1common`/`v1phoenix`), so the same code path covers pre- and post-consolidation legacy stacks. Deletion order: eventarc triggers → cloud run services/jobs → schedulers → pub/sub subscriptions → pub/sub topics → workflows → logging sinks → secrets → firewalls → subnets → networks → firestore → buckets (the scan resource bucket is PRESERVED and reported via `orphan_bucket_names` for import). Returns `cleanup_status = \"not_found\"` if no matching legacy resources exist.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`{project_id}/{region}`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The scanned (CAM-bound) GCP project ID. Used as the cleanup target when no `sidecar_project_id` is given (self-contained legacy deployment), and as the keying identity otherwise.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"sidecar_project_id": schema.StringAttribute{
				MarkdownDescription: "Set only when the legacy deployment used a SEPARATE sidecar project to host AVTD infrastructure. When set, the per-region infra (Cloud Run, Pub/Sub, workflows, schedulers, eventarc, sinks, secrets, networking, Firestore, buckets) is deleted from THIS project instead of `project_id`. The CAM SA must be granted the `cloud-sentry` feature role on this project out-of-band (it is not a CAM bound project) — see the cross-project IAM grant in the integration root. Omit for self-contained legacy deployments (`sidecar_project_id == project_id`).",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The GCP region to clean up (e.g. `northamerica-northeast2`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"stage": schema.StringAttribute{
				MarkdownDescription: "Informational stage label of the legacy deployment (e.g. `alpha`, `int`, `prod`). Recorded for diagnostics; AVTD resource names are discovered by prefix, not derived from stage.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_key": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded JSON service account key used to authenticate with GCP for cleanup operations. Optional — three common patterns:\n" +
					"\n" +
					"- **CAM-integrated** (recommended): set to `visionone_cam_service_account_integration.comprehensive.private_key`.\n" +
					"- **BYO key**: set to a base64-encoded JSON key for any service account with delete permissions on the legacy AVTD resources.\n" +
					"- **ADC**: omit the attribute entirely. Falls back to Application Default Credentials.",
				Optional:  true,
				Sensitive: true,
			},
			"resource_prefixes": schema.ListAttribute{
				MarkdownDescription: "Legacy AVTD resource name prefixes to match when listing resources for deletion. Defaults to `[\"v1avtd\", \"v1common\", \"v1phoenix\"]` (the cloud-sentry module's `var.RESOURCE_PREFIX` defaults). Override only if the legacy deployment used custom prefixes. Mutable: changing it after the one-time cleanup records the new value in state without re-running the sweep.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"preserve_resource_bucket": schema.BoolAttribute{
				MarkdownDescription: "When true (default), the scan resource bucket (`{prefix}-resource-bucket-{region}-{suffix}`) is NOT deleted and is reported via `orphan_bucket_names` so the new module can adopt it with `import {}`. Set false to delete it (only when scan data is disposable).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"preserve_vpc": schema.BoolAttribute{
				MarkdownDescription: "When true, the legacy VPC networking — firewalls, subnets, and the (global) network (`{prefix}-vpc-*`) — is NOT deleted, so the new module can adopt it in place via `import {}` instead of delete-then-recreate. Use this when the consolidated stack recreates the same VPC (same project and name prefix): it also avoids the cross-region failure where deleting the global network is blocked by another region's subnet still referencing it. Defaults to false (legacy networking is deleted).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"preserve_firestore": schema.BoolAttribute{
				MarkdownDescription: "When true, the CONSOLIDATED scan-tracking Firestore database (`{prefix}-scan-tracking`, no region suffix) is NOT deleted, so an already-consolidated customer's database can be adopted in place via `import {}` instead of delete-then-recreate — which loses scan-tracking state and hits Firestore's post-delete database-ID cooldown. Pre-consolidation per-region databases (`{prefix}-scan-tracking-{region}`) are still deleted regardless, as they do not match the new single name. Defaults to false (legacy Firestore is deleted).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"name_prefixes": schema.ListAttribute{
				MarkdownDescription: "The effective list of resource name prefixes used for matching.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"resources_deleted": schema.MapAttribute{
				MarkdownDescription: "Count of legacy resources deleted, keyed by resource family (run_services, run_jobs, triggers, schedulers, subscriptions, topics, workflows, sinks, secrets, firewalls, subnets, networks, firestore_databases, firestore_preserved, buckets, orphan_buckets_preserved). Legacy service accounts AND custom roles are intentionally NOT deleted — both are adopted by the new stack via import (SAs reuse fixed names; roles + their shared random_string are imported so they update in place) rather than destroyed/recreated.",
				ElementType:         types.Int64Type,
				Computed:            true,
			},
			"orphan_bucket_names": schema.ListAttribute{
				MarkdownDescription: "Scan resource bucket name(s) that pre-existed for this (project, region) and were intentionally **not** deleted. Consume from the downstream new-module via `import { for_each = ... }` to adopt the bucket(s) into the new state. Empty on fresh installs.",
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

func (r *LegacyCleanupAVTDRegion) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan legacyCleanupAVTDRegionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	infraProject := r.infraProjectID(plan)
	region := plan.Region.ValueString()
	prefixes := r.effectivePrefixes(ctx, plan, resp)

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", projectID, region))
	plan.NamePrefixes = stringListFromSlice(prefixes)
	plan.DeletionTimestamp = types.StringValue("")
	plan.CleanupError = types.StringValue("")

	var clientOptions []option.ClientOption
	if key := plan.ServiceAccountKey.ValueString(); key != "" {
		opt, err := newClientOptionFromEncodedServiceAccountKey(ctx, key)
		if err != nil {
			resp.Diagnostics.AddError("[AVTD Region Cleanup] Invalid service account key", err.Error())
			return
		}
		clientOptions = append(clientOptions, opt)
	}

	tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] start scanned_project=%s infra_project=%s region=%s prefixes=%v", projectID, infraProject, region, prefixes))

	result, err := runAVTDRegionCleanup(ctx, avtdRegionCleanupOptions{
		ProjectID:              infraProject,
		CustomerProjectID:      projectID,
		Region:                 region,
		Prefixes:               prefixes,
		PreserveResourceBucket: plan.PreserveResourceBucket.ValueBool(),
		PreserveVPC:            plan.PreserveVPC.ValueBool(),
		PreserveFirestore:      plan.PreserveFirestore.ValueBool(),
		ClientOptions:          clientOptions,
	})

	resourcesDeleted, diag := types.MapValueFrom(ctx, types.Int64Type, result.ResourcesDeleted)
	resp.Diagnostics.Append(diag...)
	plan.ResourcesDeleted = resourcesDeleted

	plan.OrphanBucketNames = stringListFromSlice(result.OrphanBuckets)

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

	tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] done project=%s region=%s status=%s", projectID, region, plan.CleanupStatus.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("[AVTD Region Cleanup] cleanup %s for project=%s region=%s", plan.CleanupStatus.ValueString(), projectID, region),
			fmt.Sprintf("%s\n\nResolve the listed resources manually (or via gcloud) and re-run `terraform apply`.", err.Error()),
		)
	}
}

func (r *LegacyCleanupAVTDRegion) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan legacyCleanupAVTDRegionModel
	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if !req.State.Raw.IsNull() {
		var state legacyCleanupAVTDRegionModel
		if diags := req.State.Get(ctx, &state); !diags.HasError() && !state.OrphanBucketNames.IsNull() && !state.OrphanBucketNames.IsUnknown() {
			plan.OrphanBucketNames = state.OrphanBucketNames
			resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
			return
		}
	}

	if plan.ProjectID.IsUnknown() || plan.Region.IsUnknown() || plan.SidecarProjectID.IsUnknown() {
		return
	}
	if !plan.PreserveResourceBucket.IsNull() && !plan.PreserveResourceBucket.IsUnknown() && !plan.PreserveResourceBucket.ValueBool() {
		plan.OrphanBucketNames = stringListFromSlice(nil)
		resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
		return
	}

	prefixes := r.effectivePrefixes(ctx, plan, nil)
	buckets, err := probeOrphanResourceBuckets(ctx, r.infraProjectID(plan), plan.Region.ValueString(), prefixes)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] ModifyPlan probe failed (ADC required): %v", err))
		buckets = nil
	}
	plan.OrphanBucketNames = stringListFromSlice(buckets)
	resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
}

func (r *LegacyCleanupAVTDRegion) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state legacyCleanupAVTDRegionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupAVTDRegion) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state legacyCleanupAVTDRegionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var plan legacyCleanupAVTDRegionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ServiceAccountKey = plan.ServiceAccountKey
	state.PreserveResourceBucket = plan.PreserveResourceBucket
	state.PreserveVPC = plan.PreserveVPC
	state.PreserveFirestore = plan.PreserveFirestore
	state.ResourcePrefixes = plan.ResourcePrefixes
	state.NamePrefixes = stringListFromSlice(r.effectivePrefixes(ctx, plan, nil))
	if !plan.OrphanBucketNames.IsNull() && !plan.OrphanBucketNames.IsUnknown() {
		state.OrphanBucketNames = plan.OrphanBucketNames
	} else if state.OrphanBucketNames.IsNull() || state.OrphanBucketNames.IsUnknown() {
		state.OrphanBucketNames = stringListFromSlice(nil)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupAVTDRegion) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	_ = resp
}

func (r *LegacyCleanupAVTDRegion) effectivePrefixes(ctx context.Context, plan legacyCleanupAVTDRegionModel, resp *resource.CreateResponse) []string {
	if plan.ResourcePrefixes.IsNull() || plan.ResourcePrefixes.IsUnknown() {
		return config.DEFAULT_RESOURCE_PREFIXES
	}
	var prefixes []string
	diags := plan.ResourcePrefixes.ElementsAs(ctx, &prefixes, false)
	if diags.HasError() {
		if resp != nil {
			resp.Diagnostics.Append(diags...)
		}
		return config.DEFAULT_RESOURCE_PREFIXES
	}
	if len(prefixes) == 0 {
		return config.DEFAULT_RESOURCE_PREFIXES
	}
	return prefixes
}

func (r *LegacyCleanupAVTDRegion) infraProjectID(plan legacyCleanupAVTDRegionModel) string {
	if !plan.SidecarProjectID.IsNull() && !plan.SidecarProjectID.IsUnknown() && plan.SidecarProjectID.ValueString() != "" {
		return plan.SidecarProjectID.ValueString()
	}
	return plan.ProjectID.ValueString()
}

func totalDeleted(counts map[string]int) int {
	sum := 0
	for _, v := range counts {
		sum += v
	}
	return sum
}
