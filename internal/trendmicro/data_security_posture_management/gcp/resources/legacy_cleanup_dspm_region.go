package resources

import (
	"context"
	"fmt"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/data_security_posture_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

var _ resource.Resource = &LegacyCleanupDSPMRegion{}

type LegacyCleanupDSPMRegion struct{}

type legacyCleanupDSPMRegionModel struct {
	ID                       types.String `tfsdk:"id"`
	ProjectID                types.String `tfsdk:"project_id"`
	Region                   types.String `tfsdk:"region"`
	Stage                    types.String `tfsdk:"stage"`
	ServiceAccountKey        types.String `tfsdk:"service_account_key"`
	SnapshotDiskBeforeDelete types.Bool   `tfsdk:"snapshot_disk_before_delete"`

	NamePrefix        types.String `tfsdk:"name_prefix"`
	SnapshotName      types.String `tfsdk:"snapshot_name"`
	ResourcesDeleted  types.Map    `tfsdk:"resources_deleted"`
	DeletionTimestamp types.String `tfsdk:"deletion_timestamp"`
	CleanupStatus     types.String `tfsdk:"cleanup_status"`
	CleanupError      types.String `tfsdk:"cleanup_error"`
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
			"name_prefix": schema.StringAttribute{
				MarkdownDescription: "The computed legacy resource prefix (e.g. `dspm-i-use1`).",
				Computed:            true,
			},
			"snapshot_name": schema.StringAttribute{
				MarkdownDescription: "The disk snapshot name created before disk deletion (empty if no disk existed or snapshot was disabled).",
				Computed:            true,
			},
			"resources_deleted": schema.MapAttribute{
				MarkdownDescription: "Count of legacy resources deleted, keyed by resource family (functions, triggers, schedulers, run_services, vms, firewalls, router_nats, routers, subnets, vpcs, connectors, disks, snapshots, resource_policies).",
				ElementType:         types.Int64Type,
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
	if key := plan.ServiceAccountKey.ValueString(); key != "" {
		opt, err := newClientOptionFromEncodedServiceAccountKey(ctx, key)
		if err != nil {
			resp.Diagnostics.AddError("[DSPM Region Cleanup] Invalid service account key", err.Error())
			return
		}
		clientOptions = append(clientOptions, opt)
	}

	tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] start project=%s region=%s prefix=%s", projectID, region, namePrefix))

	result, err := runDSPMRegionCleanup(ctx, dspmRegionCleanupOptions{
		ProjectID:                projectID,
		Region:                   region,
		NamePrefix:               namePrefix,
		SnapshotDiskBeforeDelete: plan.SnapshotDiskBeforeDelete.ValueBool(),
		ClientOptions:            clientOptions,
	})

	resourcesDeleted, diag := types.MapValueFrom(ctx, types.Int64Type, result.ResourcesDeleted)
	resp.Diagnostics.Append(diag...)
	plan.ResourcesDeleted = resourcesDeleted
	plan.SnapshotName = types.StringValue(result.SnapshotName)
	plan.DeletionTimestamp = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	deletedCount := totalDeleted(result.ResourcesDeleted)
	switch {
	case err != nil && deletedCount > 0:
		// Some resource types succeeded, at least one failed — surface details via cleanup_error.
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LegacyCleanupDSPMRegion) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op. Removing the resource from state does not undo the legacy GCP
	// deletions; matches the existing legacy_cleanup_* family.
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
