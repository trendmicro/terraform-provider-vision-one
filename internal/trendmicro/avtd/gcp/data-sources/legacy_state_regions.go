package data_sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro/avtd/gcp/data-sources/config"
	resourcesconfig "terraform-provider-vision-one/internal/trendmicro/avtd/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	firestore "google.golang.org/api/firestore/v1"
	iam "google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	storagev1 "google.golang.org/api/storage/v1"
)

var (
	_ datasource.DataSource = &LegacyStateRegionsDataSource{}
)

type LegacyStateRegionsDataSource struct{}

type legacyStateRegionsModel struct {
	ID                    types.String `tfsdk:"id"`
	ProjectID             types.String `tfsdk:"project_id"`
	SidecarProjectID      types.String `tfsdk:"sidecar_project_id"`
	ServiceAccountKey     types.String `tfsdk:"service_account_key"`
	NewStateBucket        types.String `tfsdk:"new_state_bucket"`
	ResourcePrefixes      types.List   `tfsdk:"resource_prefixes"`
	BucketName            types.String `tfsdk:"bucket_name"`
	LegacyDeploymentFound types.Bool   `tfsdk:"legacy_deployment_found"`
	Regions               types.Set    `tfsdk:"regions"`
	CustomRoleSuffix      types.String `tfsdk:"custom_role_suffix"`
	SidecarCreated        types.Bool   `tfsdk:"sidecar_created"`
	ResourceBuckets       types.Map    `tfsdk:"resource_buckets"`
	LiveResourceBuckets   types.Map    `tfsdk:"live_resource_buckets"`
	AccessLogBuckets      types.Map    `tfsdk:"access_log_buckets"`
	RoleIDs               types.Map    `tfsdk:"role_ids"`
	ServiceAccountEmails  types.Map    `tfsdk:"service_account_emails"`
	VPCSubnetRegions      types.Set    `tfsdk:"vpc_subnet_regions"`
	VPCNetworkExists      types.Bool   `tfsdk:"vpc_network_exists"`
	VPCFirewallExists     types.Bool   `tfsdk:"vpc_firewall_exists"`
	FirestoreExists       types.Bool   `tfsdk:"firestore_scan_tracking_exists"`
	ControlSAExists       types.Bool   `tfsdk:"control_sa_exists"`
	DataSAExists          types.Bool   `tfsdk:"data_sa_exists"`
	CustomerSAExists      types.Bool   `tfsdk:"customer_sa_exists"`
	ControlRoleExists     types.Bool   `tfsdk:"control_role_exists"`
	DataRoleExists        types.Bool   `tfsdk:"data_role_exists"`
	CustomerRoleExists    types.Bool   `tfsdk:"customer_role_exists"`
}

type legacyStateInfo struct {
	Regions          []string
	RoleSuffix       string
	SidecarProjectID string
	SidecarCreated   bool
	ResourceBuckets  map[string]string
	RoleIDs          map[string]string
	SAEmails         map[string]string
	AccessDenied     bool
}

func NewLegacyStateRegionsDataSource() datasource.DataSource {
	return &LegacyStateRegionsDataSource{}
}

func (d *LegacyStateRegionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.DATA_SOURCE_TYPE_AVTD_LEGACY_STATE_REGIONS
}

func (d *LegacyStateRegionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Discovers the legacy Terraform Package Solution AVTD (cloud-sentry) deployment by reading `gs://trendmicro-v1-{project_id}/default.tfstate`, and reconciles the scan resource buckets against a live GCS listing so a partial/missing state file does not strand existing buckets. Surfaces the sidecar project, per-region resource-bucket names, legacy custom-role ids, and service-account emails so the migration root can adopt them by exact identity (no `random_string` import, no live-probe `ModifyPlan`). Every output degrades to empty when no legacy deployment exists, so the same config covers fresh installs (nothing to adopt).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Equals `project_id`.",
				Computed:            true,
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID whose legacy state bucket should be inspected.",
				Required:            true,
			},
			"sidecar_project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project that hosts the legacy AVTD infrastructure (and thus the resource buckets). Optional input: set it for a SEPARATE-sidecar legacy deployment so the live bucket reconciliation lists the right project. When omitted it is computed from the legacy state's `cam_custom_settings.sidecar_project_id` output, falling back to `project_id` (self-contained deployment).",
				Optional:            true,
				Computed:            true,
			},
			"service_account_key": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded JSON service account key used to authenticate with GCS for reading the legacy state file. Optional — three common patterns:\n" +
					"\n" +
					"- **CAM-integrated** (recommended): set to `visionone_cam_service_account_integration.comprehensive.private_key`.\n" +
					"- **BYO key**: set to a base64-encoded JSON key for any service account with `storage.objects.get` on the legacy bucket.\n" +
					"- **ADC**: omit the attribute entirely. Falls back to Application Default Credentials.",
				Optional:  true,
				Sensitive: true,
			},
			"new_state_bucket": schema.StringAttribute{
				MarkdownDescription: "Optional GCS bucket (the new terraform-provider state backend) where the legacy state was copied as `{project_id}.tfstate` during migration cleanup. When set, discovery reads `{new_state_bucket}/{project_id}.tfstate` first and falls back to the legacy `trendmicro-v1-{project_id}/default.tfstate`. This keeps discovery stable across re-applies after the legacy bucket has been deleted. Leave empty to only read the legacy bucket.",
				Optional:            true,
			},
			"resource_prefixes": schema.ListAttribute{
				MarkdownDescription: "AVTD (cloud-sentry) resource name prefixes used by the LIVE reconciliation (bucket listing, VPC probe, Firestore probe) to identify this deployment's resources. Defaults to `[\"v1avtd\", \"v1common\", \"v1phoenix\"]` (the cloud-sentry module's `RESOURCE_PREFIX` defaults). Set this to the customer's overridden `RESOURCE_PREFIX` values so live detection still finds their resources, and so a shared infra project does not leak other stacks' buckets/regions into the discovered set. The state-file parse is prefix-agnostic and unaffected by this input.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"bucket_name": schema.StringAttribute{
				MarkdownDescription: "The legacy state bucket name that was probed (`trendmicro-v1-{project_id}`).",
				Computed:            true,
			},
			"legacy_deployment_found": schema.BoolAttribute{
				MarkdownDescription: "True when a legacy state object was found at EITHER the legacy bucket (`trendmicro-v1-{project_id}/default.tfstate`) OR the migrated-copy fallback (`{new_state_bucket}/{project_id}.tfstate`) — a plain existence check, independent of whether its contents could be parsed into anything useful. Use this (not `regions` being non-empty) to gate the legacy cleanup/import pipeline for a project: it stays accurate even when the state file has no per-region resources, or is denied to read (`AccessDenied`), or has a legacy layout `regions` cannot parse cleanly. False only when neither object exists at all, e.g. a genuinely new onboarding.",
				Computed:            true,
			},
			"regions": schema.SetAttribute{
				MarkdownDescription: "Set of GCP region names extracted from the legacy state file (state parse ONLY — not unioned with live-bucket regions, so a live or already-migrated region is never reported as legacy). Empty when no legacy state exists.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"custom_role_suffix": schema.StringAttribute{
				MarkdownDescription: "The shared random suffix of the legacy AVTD custom roles (`v1_avtd_*_custom_role_<suffix>`), parsed from the legacy state. Empty when no legacy roles exist. Retained for back-compat; prefer `role_ids` for exact-id adoption.",
				Computed:            true,
			},
			"sidecar_created": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy deployment CREATED its sidecar project (a managed `google_project` resource is present in the legacy state), false when it adopted an existing project (the project appears only as a data source). Lets the migration decide whether the sidecar project is safe to delete.",
				Computed:            true,
			},
			"resource_buckets": schema.MapAttribute{
				MarkdownDescription: "Map of `region => scan resource-bucket name` for the legacy deployment, reconciled from `state UNION live GCS listing` so buckets present in GCP but absent from a partial state are still captured. Use this map to feed the parsed suffix to the storage module. Gate the bucket `import {}` off `live_resource_buckets` (not this map) so a name that is in state but no longer exists live does not import-error. Empty on fresh installs.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"live_resource_buckets": schema.MapAttribute{
				MarkdownDescription: "Map of `region => scan resource-bucket name` that currently EXIST live in the infra project (`{prefix}-resource-bucket-{region}-{suffix}`). Subset of `resource_buckets` limited to buckets confirmed by the live GCS listing. Gate the resource-bucket `import {}` off this so only present buckets are adopted — a name carried only in a stale/partial legacy state does not import-error; the module creates it instead. Empty when none exist or the probe is denied.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"access_log_buckets": schema.MapAttribute{
				MarkdownDescription: "Map of `region => access-logs bucket name` that currently EXIST live in the infra project (`{prefix}-access-logs-{region}-{suffix}`). Gate the access-logs bucket `import {}` off this so only present buckets are adopted — a legacy deployment that never created access-logs buckets (or an older layout) has none, and the module creates them instead of import-erroring. Empty when none exist or the probe is denied.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"role_ids": schema.MapAttribute{
				MarkdownDescription: "Map of legacy custom-role `role_id`s keyed by logical name: `cam` (`vision_one_cam_role_*`) and the AVTD trio `control`/`customer`/`data` (`v1_avtd_*_custom_role_*`). Use to import the roles by exact id so the new stack updates them in place. Empty when no legacy roles exist.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"service_account_emails": schema.MapAttribute{
				MarkdownDescription: "Map of legacy service-account emails keyed by logical name (`cam`, `control`, `customer`, `data`). Service accounts have fixed names and GCP reserves a deleted name (~30 days), so existing SAs must be imported rather than recreated. Empty when none exist.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"vpc_subnet_regions": schema.SetAttribute{
				MarkdownDescription: "Set of regions whose legacy AVTD VPC subnet (`{prefix}-vpc-subnet-{region}`) currently EXISTS live in the infra project. Drive the new module's subnet `import {}` off this so only present subnets are adopted (a subnet already deleted is recreated, not import-errored). Live-probed; empty when none exist or the probe is denied.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"vpc_network_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy AVTD global VPC network (`{prefix}-vpc-network`) currently EXISTS live in the infra project. Gate the network `import {}` on this. Live-probed; false when absent or the probe is denied (so the new stack recreates it).",
				Computed:            true,
			},
			"vpc_firewall_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy AVTD global VPC firewall (`{prefix}-vpc-firewall`) currently EXISTS live in the infra project. Probed independently of the network (either can be deleted alone). Gate the firewall `import {}` on this.",
				Computed:            true,
			},
			"firestore_scan_tracking_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the CONSOLIDATED scan-tracking Firestore database (`{prefix}-scan-tracking`, no region suffix) currently EXISTS live in the infra project — i.e. the customer is already on the consolidated stack, whose DB name is unchanged by the migration. Gate the Firestore `import {}` on this so an already-consolidated DB is adopted in place, while a pre-consolidation (per-region) or fresh customer has no such DB and it is (re)created by the module instead of import-errored. Live-probed; false when absent or the probe is denied.",
				Computed:            true,
			},
			"control_sa_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy AVTD control-plane service account (`{common_prefix}-control@{sidecar_project}.iam.gserviceaccount.com`) currently EXISTS live in the sidecar/infra project. Gate the control-plane SA `import {}` on this so a surviving SA is adopted (GCP reserves a deleted SA's name, so it cannot be recreated), while an absent one is created by the module instead of import-erroring. Live-probed via the IAM API; false when absent or the probe is denied.",
				Computed:            true,
			},
			"data_sa_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy AVTD data-plane service account (`{common_prefix}-data@{sidecar_project}.iam.gserviceaccount.com`) currently EXISTS live in the sidecar/infra project. Gate the data-plane SA `import {}` on this. Live-probed via the IAM API; false when absent or the probe is denied.",
				Computed:            true,
			},
			"customer_sa_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy AVTD customer-role service account (`{common_prefix}-customer-role@{project_id}.iam.gserviceaccount.com`) currently EXISTS live in the bound (customer) project. Gate the customer-role SA `import {}` on this. Live-probed via the IAM API; false when absent or the probe is denied.",
				Computed:            true,
			},
			"control_role_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy AVTD control custom role (`v1_avtd_control_custom_role_{custom_role_suffix}`) currently EXISTS live in the sidecar/infra project. Gate the control custom-role `import {}` on this so a surviving role is adopted/updated in place, while an absent one is created by the module instead of import-erroring. Live-probed via the IAM API; false when absent or the probe is denied.",
				Computed:            true,
			},
			"data_role_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy AVTD data custom role (`v1_avtd_data_custom_role_{custom_role_suffix}`) currently EXISTS live in the sidecar/infra project. Gate the data custom-role `import {}` on this. Live-probed via the IAM API; false when absent or the probe is denied.",
				Computed:            true,
			},
			"customer_role_exists": schema.BoolAttribute{
				MarkdownDescription: "True when the legacy AVTD customer custom role (`v1_avtd_customer_custom_role_{custom_role_suffix}`) currently EXISTS live in the bound (customer) project. Gate the customer custom-role `import {}` on this. Live-probed via the IAM API; false when absent or the probe is denied.",
				Computed:            true,
			},
		},
	}
}

func (d *LegacyStateRegionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data legacyStateRegionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ProjectID.ValueString()
	bucketName := resourcesconfig.LEGACY_GCP_GCS_BUCKET_PREFIX + projectID
	data.ID = types.StringValue(projectID)
	data.BucketName = types.StringValue(bucketName)

	clientOptions, err := buildStorageClientOptions(ctx, data.ServiceAccountKey.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("[AVTD Legacy State Regions] Invalid service account key", err.Error())
		return
	}

	prefixes := resourcesconfig.DEFAULT_RESOURCE_PREFIXES
	if !data.ResourcePrefixes.IsNull() && !data.ResourcePrefixes.IsUnknown() {
		var overrides []string
		resp.Diagnostics.Append(data.ResourcePrefixes.ElementsAs(ctx, &overrides, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(overrides) > 0 {
			prefixes = overrides
		}
	}

	info, found, err := discoverFromLegacyState(ctx, bucketName, resourcesconfig.LEGACY_GCP_STATE_FILE_NAME, clientOptions)
	if err != nil {
		resp.Diagnostics.AddError("[AVTD Legacy State Regions] Failed to read legacy state", err.Error())
		return
	}
	if !found {
		if newBucket := data.NewStateBucket.ValueString(); newBucket != "" {
			var fallbackFound bool
			info, fallbackFound, err = discoverFromLegacyState(ctx, newBucket, projectID+".tfstate", clientOptions)
			if err != nil {
				resp.Diagnostics.AddError("[AVTD Legacy State Regions] Failed to read migrated state", err.Error())
				return
			}
			found = fallbackFound
		}
	}
	data.LegacyDeploymentFound = types.BoolValue(found)

	if info.AccessDenied {
		msg := fmt.Sprintf(
			"Permission denied reading gs://%s/%s. The legacy state bucket exists but the "+
				"service account cannot read it, so legacy roles/service accounts/buckets could "+
				"not be discovered for adoption. Grant the CAM service account 'storage.objects.get' "+
				"on this bucket (the cloud-sentry feature role includes it), or pass a "+
				"'service_account_key' that can read it, then re-run.",
			bucketName, resourcesconfig.LEGACY_GCP_STATE_FILE_NAME,
		)
		tflog.Warn(ctx, "[AVTD Legacy State Regions] "+msg)
		resp.Diagnostics.AddWarning("[AVTD Legacy State Regions] Cannot read legacy state (permission denied)", msg)
	}

	sidecarProject := data.SidecarProjectID.ValueString()
	if sidecarProject == "" {
		sidecarProject = info.SidecarProjectID
	}
	if sidecarProject == "" {
		sidecarProject = projectID
	}

	liveResourceBuckets := map[string]string{}
	if liveBuckets, listErr := listLiveBucketsByInfix(ctx, sidecarProject, resourcesconfig.RESOURCE_BUCKET_INFIX, prefixes, clientOptions); listErr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] live bucket reconciliation skipped for project=%s: %v", sidecarProject, listErr))
	} else {
		liveResourceBuckets = liveBuckets
		for region, name := range liveBuckets {
			if _, ok := info.ResourceBuckets[region]; !ok {
				info.ResourceBuckets[region] = name
			}
		}
	}

	accessLogBuckets := map[string]string{}
	if liveLogs, listErr := listLiveBucketsByInfix(ctx, sidecarProject, resourcesconfig.ACCESS_LOGS_BUCKET_INFIX, prefixes, clientOptions); listErr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] access-logs bucket reconciliation skipped for project=%s: %v", sidecarProject, listErr))
	} else {
		accessLogBuckets = liveLogs
	}

	regions := make([]string, 0, len(info.Regions))
	regions = append(regions, info.Regions...)

	tflog.Info(ctx, fmt.Sprintf("[AVTD Legacy State Regions] project=%s sidecar=%s bucket=%s regions=%d buckets=%d roles=%d created=%t",
		projectID, sidecarProject, bucketName, len(regions), len(info.ResourceBuckets), len(info.RoleIDs), info.SidecarCreated))

	setVal, diag := types.SetValueFrom(ctx, types.StringType, regions)
	resp.Diagnostics.Append(diag...)
	data.Regions = setVal

	bucketsVal, diag := types.MapValueFrom(ctx, types.StringType, info.ResourceBuckets)
	resp.Diagnostics.Append(diag...)
	data.ResourceBuckets = bucketsVal

	liveBucketsVal, diag := types.MapValueFrom(ctx, types.StringType, liveResourceBuckets)
	resp.Diagnostics.Append(diag...)
	data.LiveResourceBuckets = liveBucketsVal

	accessLogsVal, diag := types.MapValueFrom(ctx, types.StringType, accessLogBuckets)
	resp.Diagnostics.Append(diag...)
	data.AccessLogBuckets = accessLogsVal

	roleIDsVal, diag := types.MapValueFrom(ctx, types.StringType, info.RoleIDs)
	resp.Diagnostics.Append(diag...)
	data.RoleIDs = roleIDsVal

	saVal, diag := types.MapValueFrom(ctx, types.StringType, info.SAEmails)
	resp.Diagnostics.Append(diag...)
	data.ServiceAccountEmails = saVal

	var vpcSubnetRegions []string
	var vpcNetworkExists, vpcFirewallExists bool
	if computeOptions, cerr := buildComputeClientOptions(ctx, data.ServiceAccountKey.ValueString()); cerr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] VPC probe skipped (bad key) for project=%s: %v", sidecarProject, cerr))
	} else {
		vpcSubnetRegions, vpcNetworkExists, vpcFirewallExists = probeLegacyVPC(ctx, sidecarProject, prefixes, computeOptions)
	}

	vpcRegionsVal, diag := types.SetValueFrom(ctx, types.StringType, vpcSubnetRegions)
	resp.Diagnostics.Append(diag...)
	data.VPCSubnetRegions = vpcRegionsVal
	data.VPCNetworkExists = types.BoolValue(vpcNetworkExists)
	data.VPCFirewallExists = types.BoolValue(vpcFirewallExists)

	firestoreExists := probeLegacyFirestore(ctx, sidecarProject, prefixes, clientOptions)
	data.FirestoreExists = types.BoolValue(firestoreExists)

	var identity legacyIdentityLive
	if iamOptions, ierr := buildIAMClientOptions(ctx, data.ServiceAccountKey.ValueString()); ierr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] identity probe skipped (bad key) for sidecar=%s customer=%s: %v", sidecarProject, projectID, ierr))
	} else {
		identity = probeLegacyIdentity(ctx, sidecarProject, projectID, prefixes, info.RoleSuffix, iamOptions)
	}
	data.ControlSAExists = types.BoolValue(identity.ControlSA)
	data.DataSAExists = types.BoolValue(identity.DataSA)
	data.CustomerSAExists = types.BoolValue(identity.CustomerSA)
	data.ControlRoleExists = types.BoolValue(identity.ControlRole)
	data.DataRoleExists = types.BoolValue(identity.DataRole)
	data.CustomerRoleExists = types.BoolValue(identity.CustomerRole)

	if resp.Diagnostics.HasError() {
		return
	}

	data.CustomRoleSuffix = types.StringValue(info.RoleSuffix)
	data.SidecarProjectID = types.StringValue(sidecarProject)
	data.SidecarCreated = types.BoolValue(info.SidecarCreated)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func buildStorageClientOptions(ctx context.Context, encodedKey string) ([]option.ClientOption, error) {
	if encodedKey == "" {
		return nil, nil
	}
	keyJSON, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode service account key: %w", err)
	}
	creds, err := google.CredentialsFromJSONWithType(ctx, keyJSON, google.ServiceAccount, storagev1.DevstorageReadOnlyScope)
	if err != nil {
		return nil, fmt.Errorf("credentials from service account key: %w", err)
	}
	return []option.ClientOption{option.WithCredentials(creds)}, nil
}

func buildComputeClientOptions(ctx context.Context, encodedKey string) ([]option.ClientOption, error) {
	if encodedKey == "" {
		return nil, nil
	}
	keyJSON, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode service account key: %w", err)
	}
	creds, err := google.CredentialsFromJSONWithType(ctx, keyJSON, google.ServiceAccount, compute.ComputeReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("credentials from service account key: %w", err)
	}
	return []option.ClientOption{option.WithCredentials(creds)}, nil
}

func buildIAMClientOptions(ctx context.Context, encodedKey string) ([]option.ClientOption, error) {
	if encodedKey == "" {
		return nil, nil
	}
	keyJSON, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode service account key: %w", err)
	}
	creds, err := google.CredentialsFromJSONWithType(ctx, keyJSON, google.ServiceAccount, iam.CloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("credentials from service account key: %w", err)
	}
	return []option.ClientOption{option.WithCredentials(creds)}, nil
}

func vpcNameMatches(name string, prefixes []string, kind string) bool {
	for _, p := range prefixes {
		switch kind {
		case "network":
			if name == p+"-vpc-network" {
				return true
			}
		case "firewall":
			if name == p+"-vpc-firewall" {
				return true
			}
		case "subnet":
			if strings.HasPrefix(name, p+"-vpc-subnet-") {
				return true
			}
		}
	}
	return false
}

func regionFromSubnetURL(regionURL string) string {
	if i := strings.LastIndex(regionURL, "/"); i >= 0 {
		return regionURL[i+1:]
	}
	return regionURL
}

func probeLegacyVPC(ctx context.Context, projectID string, prefixes []string, clientOptions []option.ClientOption) (subnetRegions []string, networkExists, firewallExists bool) {
	svc, err := compute.NewService(ctx, clientOptions...)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] VPC probe: compute client error for project=%s: %v", projectID, err))
		return nil, false, false
	}

	if nwList, lerr := svc.Networks.List(projectID).Context(ctx).Do(); lerr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] VPC probe: networks.list failed for project=%s: %v", projectID, lerr))
	} else {
		for _, nw := range nwList.Items {
			if vpcNameMatches(nw.Name, prefixes, "network") {
				networkExists = true
				break
			}
		}
	}

	if fwList, lerr := svc.Firewalls.List(projectID).Context(ctx).Do(); lerr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] VPC probe: firewalls.list failed for project=%s: %v", projectID, lerr))
	} else {
		for _, fw := range fwList.Items {
			if vpcNameMatches(fw.Name, prefixes, "firewall") {
				firewallExists = true
				break
			}
		}
	}

	seen := map[string]struct{}{}
	if lerr := svc.Subnetworks.AggregatedList(projectID).Pages(ctx, func(page *compute.SubnetworkAggregatedList) error {
		for _, scoped := range page.Items {
			for _, sn := range scoped.Subnetworks {
				if !vpcNameMatches(sn.Name, prefixes, "subnet") {
					continue
				}
				if region := regionFromSubnetURL(sn.Region); region != "" {
					if _, ok := seen[region]; !ok {
						seen[region] = struct{}{}
						subnetRegions = append(subnetRegions, region)
					}
				}
			}
		}
		return nil
	}); lerr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] VPC probe: subnetworks.aggregatedList failed for project=%s: %v", projectID, lerr))
	}

	return subnetRegions, networkExists, firewallExists
}

func probeLegacyFirestore(ctx context.Context, projectID string, prefixes []string, clientOptions []option.ClientOption) bool {
	svc, err := firestore.NewService(ctx, clientOptions...)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] Firestore probe: client error for project=%s: %v", projectID, err))
		return false
	}
	listResp, lerr := svc.Projects.Databases.List(fmt.Sprintf("projects/%s", projectID)).Context(ctx).Do()
	if lerr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] Firestore probe: databases.list failed for project=%s: %v", projectID, lerr))
		return false
	}
	for _, db := range listResp.Databases {
		short := db.Name
		if i := strings.LastIndex(short, "/"); i >= 0 {
			short = short[i+1:]
		}
		for _, p := range prefixes {
			if short == p+"-scan-tracking" {
				return true
			}
		}
	}
	return false
}

type legacyIdentityLive struct {
	ControlSA    bool
	DataSA       bool
	CustomerSA   bool
	ControlRole  bool
	DataRole     bool
	CustomerRole bool
}

func probeLegacyIdentity(ctx context.Context, sidecarProject, customerProject string, prefixes []string, roleSuffix string, clientOptions []option.ClientOption) legacyIdentityLive {
	var out legacyIdentityLive

	svc, err := iam.NewService(ctx, clientOptions...)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] identity probe: IAM client error: %v", err))
		return out
	}

	saSidecar := listLiveSAKeys(ctx, svc, sidecarProject, prefixes)
	out.ControlSA = saSidecar["control"]
	out.DataSA = saSidecar["data"]
	saCustomer := saSidecar
	if customerProject != sidecarProject {
		saCustomer = listLiveSAKeys(ctx, svc, customerProject, prefixes)
	}
	out.CustomerSA = saCustomer["customer"]

	roleSidecar := listLiveRoleKeys(ctx, svc, sidecarProject, roleSuffix)
	out.ControlRole = roleSidecar["control"]
	out.DataRole = roleSidecar["data"]
	roleCustomer := roleSidecar
	if customerProject != sidecarProject {
		roleCustomer = listLiveRoleKeys(ctx, svc, customerProject, roleSuffix)
	}
	out.CustomerRole = roleCustomer["customer"]

	return out
}

func listLiveSAKeys(ctx context.Context, svc *iam.Service, projectID string, prefixes []string) map[string]bool {
	found := map[string]bool{}
	err := svc.Projects.ServiceAccounts.List("projects/"+projectID).Pages(ctx, func(page *iam.ListServiceAccountsResponse) error {
		for _, sa := range page.Accounts {
			if !hasAnyPrefix(sa.Email, prefixes) {
				continue
			}
			if key := classifySA(sa.Email); key == "control" || key == "data" || key == "customer" {
				found[key] = true
			}
		}
		return nil
	})
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] identity probe: serviceAccounts.list failed for project=%s: %v", projectID, err))
	}
	return found
}

func listLiveRoleKeys(ctx context.Context, svc *iam.Service, projectID, roleSuffix string) map[string]bool {
	found := map[string]bool{}
	err := svc.Projects.Roles.List("projects/"+projectID).Pages(ctx, func(page *iam.ListRolesResponse) error {
		for _, role := range page.Roles {
			roleID := role.Name
			if i := strings.LastIndex(roleID, "/"); i >= 0 {
				roleID = roleID[i+1:]
			}
			if roleSuffix != "" && !strings.HasSuffix(roleID, roleSuffix) {
				continue
			}
			if key := classifyRole(roleID); key == "control" || key == "data" || key == "customer" {
				found[key] = true
			}
		}
		return nil
	})
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Legacy State Regions] identity probe: roles.list failed for project=%s: %v", projectID, err))
	}
	return found
}

var regionTokenRE = regexp.MustCompile(`\["([^"]+)"\]`)

const customRoleInfix = "_custom_role_"

func discoverFromLegacyState(ctx context.Context, bucketName, objectName string, clientOptions []option.ClientOption) (*legacyStateInfo, bool, error) {
	info := &legacyStateInfo{
		ResourceBuckets: map[string]string{},
		RoleIDs:         map[string]string{},
		SAEmails:        map[string]string{},
	}

	svc, err := storagev1.NewService(ctx, clientOptions...)
	if err != nil {
		return nil, false, fmt.Errorf("storage client: %w", err)
	}

	rc, err := svc.Objects.Get(bucketName, objectName).Context(ctx).Download()
	if err != nil {
		if isStoragePermissionDenied(err) {
			info.AccessDenied = true
			return info, true, nil
		}
		if isStorageNotFound(err) {
			return info, false, nil
		}
		return nil, false, fmt.Errorf("download legacy state: %w", err)
	}
	defer rc.Body.Close()

	body, err := io.ReadAll(rc.Body)
	if err != nil {
		return nil, false, fmt.Errorf("read legacy state: %w", err)
	}

	var state struct {
		Outputs map[string]struct {
			Value json.RawMessage `json:"value"`
		} `json:"outputs"`
		Resources []struct {
			Module    string `json:"module"`
			Mode      string `json:"mode"`
			Type      string `json:"type"`
			Instances []struct {
				Attributes struct {
					RoleID   string `json:"role_id"`
					Name     string `json:"name"`
					Location string `json:"location"`
					Email    string `json:"email"`
					Project  string `json:"project"`
				} `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, false, fmt.Errorf("parse legacy state: %w", err)
	}

	if out, ok := state.Outputs["cam_custom_settings"]; ok && len(out.Value) > 0 {
		var settings struct {
			SidecarProjectID string `json:"sidecar_project_id"`
		}
		if err := json.Unmarshal(out.Value, &settings); err == nil {
			info.SidecarProjectID = settings.SidecarProjectID
		}
	}

	seen := make(map[string]struct{})
	bucketProject := ""
	for _, r := range state.Resources {
		for _, m := range regionTokenRE.FindAllStringSubmatch(r.Module, -1) {
			tok := m[1]
			if !isRegionToken(tok) {
				continue
			}
			seen[tok] = struct{}{}
		}

		switch r.Type {
		case "google_project":
			if r.Mode == "managed" {
				info.SidecarCreated = true
			}
		case "google_storage_bucket":
			for _, inst := range r.Instances {
				name := inst.Attributes.Name
				if !strings.Contains(name, resourcesconfig.RESOURCE_BUCKET_INFIX) {
					continue
				}
				region := strings.ToLower(inst.Attributes.Location)
				if region != "" {
					info.ResourceBuckets[region] = name
				}
				if bucketProject == "" && inst.Attributes.Project != "" {
					bucketProject = inst.Attributes.Project
				}
			}
		case "google_project_iam_custom_role":
			for _, inst := range r.Instances {
				roleID := inst.Attributes.RoleID
				if key := classifyRole(roleID); key != "" {
					info.RoleIDs[key] = roleID
				}
				if info.RoleSuffix == "" {
					if idx := strings.LastIndex(roleID, customRoleInfix); idx >= 0 {
						info.RoleSuffix = roleID[idx+len(customRoleInfix):]
					}
				}
			}
		case "google_service_account":
			for _, inst := range r.Instances {
				if key := classifySA(inst.Attributes.Email); key != "" {
					info.SAEmails[key] = inst.Attributes.Email
				}
			}
		}
	}

	if info.SidecarProjectID == "" && bucketProject != "" {
		info.SidecarProjectID = bucketProject
	}

	info.Regions = make([]string, 0, len(seen))
	for r := range seen {
		info.Regions = append(info.Regions, r)
	}
	return info, true, nil
}

func classifyRole(roleID string) string {
	switch {
	case strings.Contains(roleID, "_cam_role_"):
		return "cam"
	case strings.Contains(roleID, "_control_custom_role_"):
		return "control"
	case strings.Contains(roleID, "_customer_custom_role_"):
		return "customer"
	case strings.Contains(roleID, "_data_custom_role_"):
		return "data"
	}
	return ""
}

func classifySA(email string) string {
	switch {
	case strings.HasPrefix(email, "vision-one-service-account@"):
		return "cam"
	case strings.Contains(email, "-customer-role@"):
		return "customer"
	case strings.Contains(email, "-control@"):
		return "control"
	case strings.Contains(email, "-data@"):
		return "data"
	}
	return ""
}

func hasAnyPrefix(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func listLiveBucketsByInfix(ctx context.Context, projectID, infix string, prefixes []string, clientOptions []option.ClientOption) (map[string]string, error) {
	svc, err := storagev1.NewService(ctx, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}

	out := map[string]string{}
	call := svc.Buckets.List(projectID).Context(ctx)
	err = call.Pages(ctx, func(page *storagev1.Buckets) error {
		for _, b := range page.Items {
			if !hasAnyPrefix(b.Name, prefixes) || !strings.Contains(b.Name, infix) {
				continue
			}
			region := strings.ToLower(b.Location)
			if region != "" {
				out[region] = b.Name
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}
	return out, nil
}

func isStorageNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "404") ||
		strings.Contains(msg, "notFound") || strings.Contains(msg, "doesn't exist")
}

func isStoragePermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "403") ||
		strings.Contains(msg, "Forbidden") || strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "PERMISSION_DENIED") || strings.Contains(msg, "permission")
}

func isRegionToken(tok string) bool {
	if tok == "" {
		return false
	}
	last := tok[len(tok)-1]
	if last < '0' || last > '9' {
		return false
	}
	return !isAllDigits(tok)
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
