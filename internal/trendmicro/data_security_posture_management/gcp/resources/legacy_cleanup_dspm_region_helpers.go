package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	camconfig "terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"
	"terraform-provider-vision-one/internal/trendmicro/data_security_posture_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	cloudbuild "google.golang.org/api/cloudbuild/v1"
	cloudfunctions "google.golang.org/api/cloudfunctions/v2"
	scheduler "google.golang.org/api/cloudscheduler/v1"
	compute "google.golang.org/api/compute/v1"
	crm "google.golang.org/api/cloudresourcemanager/v1"
	eventarc "google.golang.org/api/eventarc/v1"
	"google.golang.org/api/googleapi"
	iam "google.golang.org/api/iam/v1"
	logging "google.golang.org/api/logging/v2"
	"google.golang.org/api/option"
	run "google.golang.org/api/run/v2"
	storagev1 "google.golang.org/api/storage/v1"
	vpcaccess "google.golang.org/api/vpcaccess/v1"
)

// dspmFeatureRoleTitle is the canonical Title field on the custom role created
// by `visionone_cam_iam_custom_role.dspm_feature` for each bound project. The
// janitor uses this as a scope guard so it never touches another product's role.
const dspmFeatureRoleTitle = "Vision One DSPM Feature Role"

// regionAbbreviationOverrides mirrors the explicit `region_abbr()` case
// statement that ships in dspm-cloud-autonomous-gcp-tf today
// (config/module_template_mg.{int,stg,prod}.txt). Bytes MUST match the bash
// output or legacy resource names won't be found.
var regionAbbreviationOverrides = map[string]string{
	"us-central1":             "usc1",
	"us-east1":                "use1",
	"us-east4":                "use4",
	"us-west1":                "usw1",
	"us-west2":                "usw2",
	"us-west3":                "usw3",
	"us-west4":                "usw4",
	"us-south1":               "uss1",
	"europe-central2":         "euc2",
	"europe-north1":           "eun1",
	"europe-southwest1":       "eusw1",
	"europe-west1":            "euw1",
	"europe-west2":            "euw2",
	"europe-west3":            "euw3",
	"europe-west4":            "euw4",
	"europe-west6":            "euw6",
	"europe-west8":            "euw8",
	"europe-west9":            "euw9",
	"asia-east1":              "ase1",
	"asia-east2":              "ase2",
	"asia-northeast1":         "asne1",
	"asia-northeast2":         "asne2",
	"asia-northeast3":         "asne3",
	"asia-south1":             "ass1",
	"asia-south2":             "ass2",
	"asia-southeast1":         "asse1",
	"asia-southeast2":         "asse2",
	"australia-southeast1":    "ause1",
	"australia-southeast2":    "ause2",
	"southamerica-east1":      "sae1",
	"southamerica-west1":      "saw1",
	"northamerica-northeast1": "nane1",
	"northamerica-northeast2": "nane2",
	"me-central1":             "mec1",
	"me-west1":                "mew1",
}

// regionAbbreviation returns the legacy prefix abbreviation; falls back to bash's `tr -d '-' | cut -c1-8`.
func regionAbbreviation(region string) string {
	if v, ok := regionAbbreviationOverrides[region]; ok {
		return v
	}
	stripped := strings.ReplaceAll(region, "-", "")
	if len(stripped) > 8 {
		stripped = stripped[:8]
	}
	return stripped
}

type dspmRegionCleanupOptions struct {
	ProjectID                string
	Region                   string
	NamePrefix               string // e.g. "dspm-i-use1"
	SnapshotDiskBeforeDelete bool
	ClientOptions            []option.ClientOption
	// SAEmail is the client_email from ServiceAccountKey, used by the
	// orphan-binding janitor pass. Empty string skips janitor entirely.
	SAEmail string
}

type dspmRegionCleanupResult struct {
	ResourcesDeleted map[string]int
	SnapshotName     string
	// OrphanBuckets lists the new-module-style GCS buckets that pre-existed
	// for this (project, region) tuple. cleanup_region intentionally does
	// NOT delete these (audit logs are data-preservation-sensitive, and
	// deleting them races GCP's audit-log forwarding pipeline buffer).
	// The downstream new-module is expected to adopt them via Terraform
	// `import { for_each = ... }` blocks keyed on these names. Empty on
	// fresh installs.
	OrphanBuckets []string
}

const (
	// 30 polls × 10s matches the bash retry loop (`for i in $(seq 1 30); do … sleep 10; done`).
	asyncOpPollInterval = 10 * time.Second
	asyncOpMaxPolls     = 30

	// IAM propagation wait budget — fail-fast philosophy. Active polling
	// catches normal propagation (typically 60-120s) within budget. Beyond
	// that, something is structurally wrong (missing role, broken binding,
	// wrong SA) and waiting longer just hides the real error. Better to
	// surface a clear "IAM didn't propagate in N seconds" diagnostic
	// pointing at the upstream resources than to silently hang the apply.
	cleanupPermsWaitMaxDuration = 3 * time.Minute
	cleanupPermsPollStart       = 5 * time.Second
	cleanupPermsPollCap         = 20 * time.Second
	cleanupPermsPollFactor      = 2.0

	// Per-service IAM cache warmup — same fail-fast principle. Each service
	// usually catches up within 30-60s of the central IAM update. If it
	// doesn't, retrying longer rarely helps.
	cleanupServiceCacheMaxDuration = 90 * time.Second
	cleanupServiceCachePollStart   = 5 * time.Second
	cleanupServiceCachePollCap     = 15 * time.Second
)

// warmupServiceCaches polls each GCP service's IAM resolver until it stops
// 403'ing the SA. Even after testIamPermissions reports all perms granted
// (central IAM), individual services keep their own IAM caches that can lag
// 30-90s. We probe with cheap LIST calls (covered by viewer-level perms the
// SA holds) — a 200 / 404 means the cache has refreshed for our principal;
// a 403 means it hasn't.
//
// Probe coverage MUST match the services this cleanup actually calls
// destructive APIs against. Missing a service here = first delete call to
// that service will false-403 (the original bug). Each service we delete
// from below has a corresponding probe here.
func warmupServiceCaches(ctx context.Context, projectID, region string, opts ...option.ClientOption) error {
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)
	projParent := fmt.Sprintf("projects/%s", projectID)

	type probe struct {
		name string
		fn   func() error
	}

	probes := []probe{
		// Services with location/region scope:
		{"eventarc", func() error {
			svc, err := eventarc.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Triggers.List(parent).PageSize(1).Context(ctx).Do()
			return err
		}},
		{"cloudfunctions", func() error {
			svc, err := cloudfunctions.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Functions.List(parent).PageSize(1).Context(ctx).Do()
			return err
		}},
		{"run", func() error {
			svc, err := run.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Services.List(parent).PageSize(1).Context(ctx).Do()
			return err
		}},
		{"cloudscheduler", func() error {
			svc, err := scheduler.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Jobs.List(parent).PageSize(1).Context(ctx).Do()
			return err
		}},
		{"vpcaccess", func() error {
			svc, err := vpcaccess.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Connectors.List(parent).PageSize(1).Context(ctx).Do()
			return err
		}},
		// Project-scoped services:
		{"compute", func() error {
			// One list call probes the compute API's IAM resolver — covers
			// disks/instances/firewalls/networks/routers/subnetworks/snapshots/
			// resourcePolicies (all under compute.googleapis.com).
			svc, err := compute.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Networks.List(projectID).MaxResults(1).Context(ctx).Do()
			return err
		}},
		{"logging", func() error {
			svc, err := logging.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Sinks.List(projParent).PageSize(1).Context(ctx).Do()
			return err
		}},
		{"storage", func() error {
			svc, err := storagev1.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Buckets.List(projectID).MaxResults(1).Context(ctx).Do()
			return err
		}},
	}

	is403 := func(err error) bool {
		if err == nil {
			return false
		}
		var gerr *googleapi.Error
		if errors.As(err, &gerr) {
			return gerr.Code == 403
		}
		return strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "PERMISSION_DENIED")
	}

	deadline := time.Now().Add(cleanupServiceCacheMaxDuration)
	backoff := cleanupServiceCachePollStart
	attempt := 0
	for {
		attempt++
		notReady := make([]string, 0)
		for _, p := range probes {
			if err := p.fn(); err != nil && is403(err) {
				notReady = append(notReady, p.name)
			}
		}
		if len(notReady) == 0 {
			tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] per-service IAM caches warm on %s (attempt %d)", projectID, attempt))
			return nil
		}
		tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] per-service IAM cache lag on %s: %v still 403 (attempt %d)", projectID, notReady, attempt))

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("per-service IAM cache warmup timeout on project %s after %s — services still 403: %v", projectID, cleanupServiceCacheMaxDuration, notReady)
		}
		wait := backoff
		if wait > remaining {
			wait = remaining
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
		backoff *= 2
		if backoff > cleanupServiceCachePollCap {
			backoff = cleanupServiceCachePollCap
		}
	}
}

// waitForCleanupPermsReady polls projects.testIamPermissions as the calling
// principal (the CAM SA when opts carries its key, or ADC otherwise) until
// every perm in the DSPM feature role has been granted on projectID, or the
// budget runs out. Eliminates the racy "fresh role + binding + 60s sleep"
// pattern that intermittently fires 403s on the first apply of a folder/org
// install.
//
// Returns nil when ready. On timeout the error names a sample missing perm so
// the operator log points straight at the broken IAM binding rather than
// surfacing a downstream "permission denied" from whichever Delete call
// happened to run first.
func waitForCleanupPermsReady(ctx context.Context, projectID string, opts ...option.ClientOption) error {
	required := camconfig.FEATURE_PERMISSIONS[camconfig.FEATURE_DATA_SECURITY_POSTURE_MANAGEMENT]
	if len(required) == 0 {
		// Defensive: empty list means no perms to wait for — nothing to do.
		return nil
	}

	crmSvc, err := crm.NewService(ctx, opts...)
	if err != nil {
		return fmt.Errorf("crm client: %w", err)
	}

	deadline := time.Now().Add(cleanupPermsWaitMaxDuration)
	backoff := cleanupPermsPollStart
	attempt := 0
	for {
		attempt++
		resp, callErr := crmSvc.Projects.TestIamPermissions(projectID, &crm.TestIamPermissionsRequest{
			Permissions: required,
		}).Context(ctx).Do()

		if callErr == nil {
			granted := make(map[string]struct{}, len(resp.Permissions))
			for _, p := range resp.Permissions {
				granted[p] = struct{}{}
			}
			missing := make([]string, 0)
			for _, p := range required {
				if _, ok := granted[p]; !ok {
					missing = append(missing, p)
				}
			}
			if len(missing) == 0 {
				tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] central IAM ready on %s — all %d cleanup perms granted (attempt %d)", projectID, len(required), attempt))
				return nil
			}
			tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] IAM not ready on %s: %d/%d granted, %d missing (e.g. %s) — attempt %d", projectID, len(required)-len(missing), len(required), len(missing), missing[0], attempt))
		} else {
			// 403 here means the SA isn't even recognised at the project yet.
			// Retry — propagation may catch up.
			tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] testIamPermissions on %s failed (attempt %d): %v — will retry", projectID, attempt, callErr))
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			if callErr != nil {
				return fmt.Errorf("IAM propagation timeout on project %s after %s — testIamPermissions kept failing: %w", projectID, cleanupPermsWaitMaxDuration, callErr)
			}
			return fmt.Errorf("IAM propagation timeout on project %s after %s — CAM SA still missing cleanup perms (verify visionone_cam_iam_custom_role.dspm_feature[%q] + google_project_iam_member.dspm_feature_binding[%q] succeeded)", projectID, cleanupPermsWaitMaxDuration, projectID, projectID)
		}

		wait := backoff
		if wait > remaining {
			wait = remaining
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
		backoff = time.Duration(float64(backoff) * cleanupPermsPollFactor)
		if backoff > cleanupPermsPollCap {
			backoff = cleanupPermsPollCap
		}
	}
}

// runDSPMRegionCleanup deletes legacy DSPM Package resources in dependency
// order. Errors are collected but don't short-circuit — goal is best-effort
// cleanup so the new stack can claim the same names. NotFound is silent success.
func runDSPMRegionCleanup(ctx context.Context, opts dspmRegionCleanupOptions) (dspmRegionCleanupResult, error) {
	result := dspmRegionCleanupResult{
		ResourcesDeleted: map[string]int{
			"triggers":          0,
			"functions":         0,
			"run_services":      0,
			"schedulers":        0,
			"disks":             0,
			"snapshots":         0,
			"resource_policies": 0,
			"vms":               0,
			"connectors":        0,
			"firewalls":         0,
			"router_nats":       0,
			"routers":           0,
			"subnets":           0,
			"vpcs":              0,
			"sinks":                    0,
			"buckets":                  0,
			"orphan_buckets_preserved": 0,
			"orphan_bindings":          0,
		},
	}
	var errs []string
	noteErr := func(family, name string, err error) {
		if err == nil || isGCPNotFound(err) {
			return
		}
		errs = append(errs, fmt.Sprintf("%s/%s: %v", family, name, err))
		tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] %s/%s failed: %v", family, name, err))
	}
	tally := func(family string, deleted bool) {
		if deleted {
			result.ResourcesDeleted[family]++
		}
	}

	pfx := opts.NamePrefix
	parent := fmt.Sprintf("projects/%s/locations/%s", opts.ProjectID, opts.Region)

	// Block until the CAM SA actually has the cleanup perms on this project.
	// Two layers because GCP IAM is eventually consistent in TWO places:
	//   (1) waitForCleanupPermsReady — polls testIamPermissions until the
	//       central IAM (resourcemanager) acknowledges all 19 cleanup perms.
	//   (2) warmupServiceCaches — even after (1), each per-service IAM
	//       resolver (cloudscheduler, vpcaccess, cloudfunctions, run, etc.)
	//       keeps its own cache that lags central by 30-90s. Probe each
	//       service's list endpoint until it stops 403'ing.
	// The static `time_sleep.wait_for_dspm_iam = 60s` in the integration TF
	// only gives a head-start; these two polls are the source of truth.
	if err := waitForCleanupPermsReady(ctx, opts.ProjectID, opts.ClientOptions...); err != nil {
		return result, err
	}
	if err := warmupServiceCaches(ctx, opts.ProjectID, opts.Region, opts.ClientOptions...); err != nil {
		return result, err
	}

	// Eventarc triggers must precede the functions/run services they fan out to.
	if eaSvc, err := eventarc.NewService(ctx, opts.ClientOptions...); err != nil {
		errs = append(errs, fmt.Sprintf("eventarc client: %v", err))
	} else {
		for _, suffix := range []string{"-launch-vm-trigger", "-terminate-vm-trigger", "-token-rotator-trigger"} {
			name := fmt.Sprintf("%s/triggers/%s%s", parent, pfx, suffix)
			deleted, err := deleteAndWaitEventarcTrigger(ctx, eaSvc, name)
			tally("triggers", deleted)
			noteErr("trigger", name, err)
		}
	}

	if fnSvc, err := cloudfunctions.NewService(ctx, opts.ClientOptions...); err != nil {
		errs = append(errs, fmt.Sprintf("cloudfunctions client: %v", err))
	} else {
		for _, suffix := range []string{"-launch-vm", "-terminate-vm"} {
			name := fmt.Sprintf("%s/functions/%s%s", parent, pfx, suffix)
			deleted, err := deleteAndWaitFunction(ctx, fnSvc, name)
			tally("functions", deleted)
			noteErr("function", name, err)
		}
	}

	if runSvc, err := run.NewService(ctx, opts.ClientOptions...); err != nil {
		errs = append(errs, fmt.Sprintf("cloud run client: %v", err))
	} else {
		name := fmt.Sprintf("%s/services/%s-token-rotator", parent, pfx)
		deleted, err := deleteAndWaitRunService(ctx, runSvc, name)
		tally("run_services", deleted)
		noteErr("run_service", name, err)
	}

	if schSvc, err := scheduler.NewService(ctx, opts.ClientOptions...); err != nil {
		errs = append(errs, fmt.Sprintf("scheduler client: %v", err))
	} else {
		for _, suffix := range []string{"-launch-vm-scheduler", "-token-rotation-scheduler"} {
			name := fmt.Sprintf("%s/jobs/%s%s", parent, pfx, suffix)
			deleted, err := deleteSchedulerJob(ctx, schSvc, name)
			tally("schedulers", deleted)
			noteErr("scheduler", name, err)
		}
	}

	cSvc, computeErr := compute.NewService(ctx, opts.ClientOptions...)
	if computeErr != nil {
		errs = append(errs, fmt.Sprintf("compute client: %v", computeErr))
	} else {
		// VMs must be deleted before the disk — if a VM holds the disk, disk delete returns 400 "in use".
		instances, err := listDSPMInstances(ctx, cSvc, opts.ProjectID, opts.Region)
		noteErr("instances_list", opts.Region, err)
		for _, inst := range instances {
			err := deleteAndWaitComputeInstance(ctx, cSvc, opts.ProjectID, inst.zone, inst.name)
			tally("vms", err == nil)
			noteErr("vm", inst.name, err)
		}

		diskName := fmt.Sprintf("%s-persistent-scan-job-disk", pfx)
		diskZone := opts.Region + "-b"
		snapName := fmt.Sprintf("%s-disk-pre-upgrade", pfx)

		diskExists, err := computeDiskExists(ctx, cSvc, opts.ProjectID, diskZone, diskName)
		noteErr("disk_describe", diskName, err)

		if diskExists && opts.SnapshotDiskBeforeDelete {
			if snapErr := snapshotDiskAndWait(ctx, cSvc, opts.ProjectID, diskZone, diskName, snapName); snapErr != nil {
				noteErr("disk_snapshot", snapName, snapErr)
			} else {
				result.SnapshotName = snapName
				tally("snapshots", true)
			}
		}

		if diskExists {
			delErr := deleteAndWaitComputeDisk(ctx, cSvc, opts.ProjectID, diskZone, diskName)
			tally("disks", delErr == nil)
			noteErr("disk", diskName, delErr)
		}

		policyName := fmt.Sprintf("%s-disk-snapshot-schedule", pfx)
		deleted, err := deleteAndWaitResourcePolicy(ctx, cSvc, opts.ProjectID, opts.Region, policyName)
		tally("resource_policies", deleted)
		noteErr("resource_policy", policyName, err)
	}

	// VPC connector must drain before VPC can be deleted (async).
	if vpcSvc, err := vpcaccess.NewService(ctx, opts.ClientOptions...); err != nil {
		errs = append(errs, fmt.Sprintf("vpcaccess client: %v", err))
	} else {
		name := fmt.Sprintf("%s/connectors/%s-vpc-conn", parent, pfx)
		deleted, err := deleteAndWaitVPCConnector(ctx, vpcSvc, name)
		tally("connectors", deleted)
		noteErr("connector", name, err)
	}

	if computeErr == nil {
		for _, fw := range []string{"-egress-dns-internal", "-egress-ntp-internal", "-egress-web", "-allow-iap-ssh"} {
			name := pfx + fw
			deleted, err := deleteAndWaitFirewall(ctx, cSvc, opts.ProjectID, name)
			tally("firewalls", deleted)
			noteErr("firewall", name, err)
		}

		routerName := pfx + "-router"
		natName := pfx + "-nat"
		deleted, err := deleteRouterNAT(ctx, cSvc, opts.ProjectID, opts.Region, routerName, natName)
		tally("router_nats", deleted)
		noteErr("router_nat", natName, err)

		deleted, err = deleteAndWaitRouter(ctx, cSvc, opts.ProjectID, opts.Region, routerName)
		tally("routers", deleted)
		noteErr("router", routerName, err)

		subnetName := pfx + "-subnet"
		deleted, err = deleteAndWaitSubnet(ctx, cSvc, opts.ProjectID, opts.Region, subnetName)
		tally("subnets", deleted)
		noteErr("subnet", subnetName, err)

		vpcName := pfx + "-vpc"
		deleted, err = deleteAndWaitVPC(ctx, cSvc, opts.ProjectID, vpcName)
		tally("vpcs", deleted)
		noteErr("vpc", vpcName, err)
	}

	// Delete the log_router_sink that targets the audit-logs bucket BEFORE
	// touching the bucket itself. The sink continuously writes audit log
	// entries to the bucket — if we empty + delete the bucket while the sink
	// is still alive, in-flight writes race the deletion and either: (a)
	// repopulate the bucket so Buckets.Delete returns 409/not-empty, or (b)
	// recreate objects between our List and Delete. Sink name matches
	// dspm-cloud-autonomous-gcp-tf's log_router_sink module: `${pfx}-audit-sink`.
	// 404 is silent success (fresh install — no sink to remove).
	if logSvc, err := logging.NewService(ctx, opts.ClientOptions...); err != nil {
		errs = append(errs, fmt.Sprintf("logging client: %v", err))
	} else {
		sinkName := fmt.Sprintf("projects/%s/sinks/%s-audit-sink", opts.ProjectID, pfx)
		deleted, err := deleteLoggingSink(ctx, logSvc, sinkName)
		tally("sinks", deleted)
		noteErr("sink", sinkName, err)
	}

	// Two buckets follow opposite strategies based on what's in them:
	//
	//   -audit-logs      → PRESERVE (compliance trail). Reported via
	//                      result.OrphanBuckets so the downstream new-module
	//                      can adopt with `import { for_each = ... }`. We
	//                      can't safely delete this — GCP's audit-log
	//                      forwarding pipeline drains in-flight entries for
	//                      seconds-to-minutes after Sinks.Delete, racing
	//                      Buckets.Delete and producing 409 not-empty.
	//
	//   -trend-resources → DELETE. Contents are Terraform-regenerated
	//                      (CF source archives, startup scripts, .keep
	//                      placeholders) — no customer data. The new
	//                      architecture relocates this bucket to
	//                      shared_resources/ in primary with a different
	//                      `-shared` suffix, so the old per-member bucket
	//                      becomes orphan with no import target. No sink
	//                      writes here ⇒ no audit-pipeline race.
	if storageSvc, err := storagev1.NewService(ctx, opts.ClientOptions...); err != nil {
		errs = append(errs, fmt.Sprintf("storage client: %v", err))
	} else {
		projectNumber, numErr := resolveProjectNumber(ctx, opts.ProjectID, opts.ClientOptions...)
		if numErr != nil {
			errs = append(errs, fmt.Sprintf("resolve project number: %v", numErr))
		} else {
			// -audit-logs: preserve for import.
			auditBucket := fmt.Sprintf("%s-%s-audit-logs", pfx, projectNumber)
			if exists, err := gcsBucketExists(ctx, storageSvc, auditBucket); err != nil {
				noteErr("bucket-probe", auditBucket, err)
			} else if exists {
				result.OrphanBuckets = append(result.OrphanBuckets, auditBucket)
				result.ResourcesDeleted["orphan_buckets_preserved"]++
				tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] audit-logs bucket preserved for new-module import: %s", auditBucket))
			}

			// -trend-resources: kill Cloud Build first (it streams compile
			// logs into container-builds/logs/ at ~1 write/sec, which would
			// race empty+delete), then drop the bucket.
			if cancelled, err := cancelActiveCloudBuilds(ctx, opts.ProjectID, opts.ClientOptions...); err != nil {
				tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] cancel builds best-effort: %v", err))
			} else if cancelled > 0 {
				tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] cancelled %d in-flight Cloud Build(s) on %s before bucket cleanup", cancelled, opts.ProjectID))
			}

			trendBucket := fmt.Sprintf("%s-%s-trend-resources", pfx, projectNumber)
			deleted, err := deleteGCSBucketIfExists(ctx, storageSvc, trendBucket)
			tally("buckets", deleted)
			noteErr("bucket", trendBucket, err)
		}
	}

	// Janitor pass: clean up project-IAM bindings that point at soft-deleted
	// DSPM-feature roles for our SA. Best-effort — its failure must not turn
	// an otherwise-clean cleanup into a partial/failed status, so we log and
	// continue rather than appending to errs. Uses ADC (operator creds), NOT
	// the CAM SA, because the CAM SA must not have setIamPolicy.
	if opts.SAEmail != "" {
		purged, err := purgeOrphanDSPMFeatureRoleBindings(ctx, opts.ProjectID, opts.SAEmail)
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] janitor: %v", err))
		}
		result.ResourcesDeleted["orphan_bindings"] = purged
		tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] janitor purged %d orphan bindings on %s", purged, opts.SAEmail))
	}

	var combinedErr error
	if len(errs) > 0 {
		combinedErr = errors.New(strings.Join(errs, "; "))
	}
	return result, combinedErr
}

// purgeOrphanDSPMFeatureRoleBindings strips saEmail from project-IAM bindings
// that point at soft-deleted "Vision One DSPM Feature Role" custom roles.
//
// Why: each apply of `visionone_cam_iam_custom_role.dspm_feature` creates a new
// random-suffix role; previous incarnations get soft-deleted but their bindings
// linger on the project's IAM policy (DDR-3273 testing observed 22 stale
// bindings on a single SA in int). They grant nothing — the role is dead — but
// clutter `gcloud projects get-iam-policy` output and survive GCP's 7-day role
// garbage collect window because policy GC is decoupled from role GC.
//
// Credentials: ADC (operator), not the CAM SA. The CAM SA must NOT carry
// resourcemanager.projects.setIamPolicy. The operator running `terraform
// apply` already has project-IAM admin rights — they're the same principal
// that creates the role + binding via the upstream `iam_custom_role` /
// `google_project_iam_member` resources.
//
// Scope guards: (projectID, dspmFeatureRoleTitle, saEmail). Roles from other
// products / other SAs are never touched.
//
// Returns count of bindings stripped (best-effort; caller treats err as warning).
func purgeOrphanDSPMFeatureRoleBindings(ctx context.Context, projectID, saEmail string) (int, error) {
	if projectID == "" || saEmail == "" {
		return 0, nil
	}

	iamSvc, err := iam.NewService(ctx)
	if err != nil {
		return 0, fmt.Errorf("iam client (ADC): %w", err)
	}
	crmSvc, err := crm.NewService(ctx)
	if err != nil {
		return 0, fmt.Errorf("crm client (ADC): %w", err)
	}

	// 1. Build set of soft-deleted DSPM-feature roles in this project.
	parent := "projects/" + projectID
	deletedRoles := map[string]bool{}
	var pageToken string
	for {
		req := iamSvc.Projects.Roles.List(parent).ShowDeleted(true)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		resp, err := req.Context(ctx).Do()
		if err != nil {
			return 0, fmt.Errorf("list roles: %w", err)
		}
		for _, role := range resp.Roles {
			if !role.Deleted {
				continue
			}
			if role.Title != dspmFeatureRoleTitle {
				continue
			}
			deletedRoles[role.Name] = true
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	if len(deletedRoles) == 0 {
		return 0, nil
	}

	// 2. Strip saEmail from any binding whose role is in deletedRoles.
	member := "serviceAccount:" + saEmail
	policy, err := crmSvc.Projects.GetIamPolicy(projectID, &crm.GetIamPolicyRequest{}).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("get iam policy: %w", err)
	}

	removed := 0
	var newBindings []*crm.Binding
	for _, b := range policy.Bindings {
		if !deletedRoles[b.Role] {
			newBindings = append(newBindings, b)
			continue
		}
		var keep []string
		memberHit := false
		for _, m := range b.Members {
			if m == member {
				memberHit = true
				continue
			}
			keep = append(keep, m)
		}
		if memberHit {
			removed++
		}
		if len(keep) > 0 {
			b.Members = keep
			newBindings = append(newBindings, b)
		}
	}
	if removed == 0 {
		return 0, nil
	}

	policy.Bindings = newBindings
	if _, err := crmSvc.Projects.SetIamPolicy(projectID, &crm.SetIamPolicyRequest{Policy: policy}).Context(ctx).Do(); err != nil {
		return 0, fmt.Errorf("set iam policy: %w", err)
	}
	return removed, nil
}

// isGCPNotFound treats 404 / "notFound" as already-absent so delete is idempotent.
func isGCPNotFound(err error) bool {
	if err == nil {
		return false
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 404
	}
	msg := err.Error()
	return strings.Contains(msg, "404") || strings.Contains(msg, "notFound") || strings.Contains(msg, "doesn't exist")
}

func deleteAndWaitEventarcTrigger(ctx context.Context, svc *eventarc.Service, name string) (bool, error) {
	op, err := svc.Projects.Locations.Triggers.Delete(name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := waitEventarcOp(ctx, svc, op); err != nil {
		// Delete API accepted but async op didn't complete — resource may still exist.
		return false, err
	}
	return true, nil
}

func waitEventarcOp(ctx context.Context, svc *eventarc.Service, op *eventarc.GoogleLongrunningOperation) error {
	for i := 0; i < asyncOpMaxPolls; i++ {
		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("eventarc op error: %s", op.Error.Message)
			}
			return nil
		}
		time.Sleep(asyncOpPollInterval)
		fresh, err := svc.Projects.Locations.Operations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return err
		}
		op = fresh
	}
	return fmt.Errorf("eventarc op %s did not finish within %s", op.Name, asyncOpPollInterval*asyncOpMaxPolls)
}

func deleteAndWaitFunction(ctx context.Context, svc *cloudfunctions.Service, name string) (bool, error) {
	op, err := svc.Projects.Locations.Functions.Delete(name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := waitCloudFunctionsOp(ctx, svc, op); err != nil {
		return false, err
	}
	return true, nil
}

func waitCloudFunctionsOp(ctx context.Context, svc *cloudfunctions.Service, op *cloudfunctions.Operation) error {
	for i := 0; i < asyncOpMaxPolls; i++ {
		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("cloudfunctions op error: %s", op.Error.Message)
			}
			return nil
		}
		time.Sleep(asyncOpPollInterval)
		fresh, err := svc.Projects.Locations.Operations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return err
		}
		op = fresh
	}
	return fmt.Errorf("cloudfunctions op %s did not finish within %s", op.Name, asyncOpPollInterval*asyncOpMaxPolls)
}

func deleteAndWaitRunService(ctx context.Context, svc *run.Service, name string) (bool, error) {
	op, err := svc.Projects.Locations.Services.Delete(name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := waitRunOp(ctx, svc, op); err != nil {
		return false, err
	}
	return true, nil
}

func waitRunOp(ctx context.Context, svc *run.Service, op *run.GoogleLongrunningOperation) error {
	for i := 0; i < asyncOpMaxPolls; i++ {
		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("run op error: %s", op.Error.Message)
			}
			return nil
		}
		time.Sleep(asyncOpPollInterval)
		fresh, err := svc.Projects.Locations.Operations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return err
		}
		op = fresh
	}
	return fmt.Errorf("run op %s did not finish within %s", op.Name, asyncOpPollInterval*asyncOpMaxPolls)
}

func deleteSchedulerJob(ctx context.Context, svc *scheduler.Service, name string) (bool, error) {
	// Scheduler delete is synchronous (returns Empty on success).
	if _, err := svc.Projects.Locations.Jobs.Delete(name).Context(ctx).Do(); err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func computeDiskExists(ctx context.Context, svc *compute.Service, projectID, zone, name string) (bool, error) {
	_, err := svc.Disks.Get(projectID, zone, name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func snapshotDiskAndWait(ctx context.Context, svc *compute.Service, projectID, zone, diskName, snapName string) error {
	snap := &compute.Snapshot{
		Name:        snapName,
		Description: fmt.Sprintf("DSPM legacy pre-upgrade snapshot of %s", diskName),
	}
	op, err := svc.Disks.CreateSnapshot(projectID, zone, diskName, snap).Context(ctx).Do()
	if err != nil {
		// 409 (already exists) → treat as success; lets re-runs leave the prior snapshot in place.
		if isGCPAlreadyExists(err) {
			return nil
		}
		return err
	}
	if err := waitComputeZoneOp(ctx, svc, projectID, zone, op); err != nil {
		return err
	}
	// Poll snapshot until status=READY (matches bash status loop).
	for i := 0; i < asyncOpMaxPolls; i++ {
		s, err := svc.Snapshots.Get(projectID, snapName).Context(ctx).Do()
		if err == nil && s.Status == "READY" {
			return nil
		}
		time.Sleep(asyncOpPollInterval)
	}
	return fmt.Errorf("snapshot %s did not reach READY within %s", snapName, asyncOpPollInterval*asyncOpMaxPolls)
}

// isGCPAlreadyExists treats 409 / "alreadyExists" as no-op success. Symmetric with isGCPNotFound.
func isGCPAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 409
	}
	msg := err.Error()
	return strings.Contains(msg, "409") || strings.Contains(msg, "alreadyExists")
}

func deleteAndWaitComputeDisk(ctx context.Context, svc *compute.Service, projectID, zone, name string) error {
	op, err := svc.Disks.Delete(projectID, zone, name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return nil
		}
		return err
	}
	return waitComputeZoneOp(ctx, svc, projectID, zone, op)
}

func deleteAndWaitResourcePolicy(ctx context.Context, svc *compute.Service, projectID, region, name string) (bool, error) {
	op, err := svc.ResourcePolicies.Delete(projectID, region, name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := waitComputeRegionOp(ctx, svc, projectID, region, op); err != nil {
		return false, err
	}
	return true, nil
}

type computeInstanceRef struct {
	name string
	zone string
}

// listDSPMInstances enumerates VMs in the region's a/b/c zones whose name starts with "dspm-".
func listDSPMInstances(ctx context.Context, svc *compute.Service, projectID, region string) ([]computeInstanceRef, error) {
	var out []computeInstanceRef
	zones := []string{region + "-a", region + "-b", region + "-c"}
	for _, zone := range zones {
		err := svc.Instances.List(projectID, zone).
			Filter(`name eq "dspm-.*"`).
			Pages(ctx, func(page *compute.InstanceList) error {
				for _, inst := range page.Items {
					if strings.HasPrefix(inst.Name, "dspm-") {
						out = append(out, computeInstanceRef{name: inst.Name, zone: zone})
					}
				}
				return nil
			})
		if err != nil && !isGCPNotFound(err) {
			return out, err
		}
	}
	return out, nil
}

func deleteAndWaitComputeInstance(ctx context.Context, svc *compute.Service, projectID, zone, name string) error {
	op, err := svc.Instances.Delete(projectID, zone, name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return nil
		}
		return err
	}
	return waitComputeZoneOp(ctx, svc, projectID, zone, op)
}

func deleteAndWaitVPCConnector(ctx context.Context, svc *vpcaccess.Service, name string) (bool, error) {
	op, err := svc.Projects.Locations.Connectors.Delete(name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	// VPC connector deletion is the slowest async op (~2 min minimum per bash).
	for i := 0; i < asyncOpMaxPolls; i++ {
		if op.Done {
			if op.Error != nil {
				return true, fmt.Errorf("vpc connector op error: %s", op.Error.Message)
			}
			return true, nil
		}
		time.Sleep(asyncOpPollInterval)
		fresh, err := svc.Projects.Locations.Operations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return false, err
		}
		op = fresh
	}
	return true, fmt.Errorf("vpc connector op %s did not finish within %s", op.Name, asyncOpPollInterval*asyncOpMaxPolls)
}

func deleteAndWaitFirewall(ctx context.Context, svc *compute.Service, projectID, name string) (bool, error) {
	op, err := svc.Firewalls.Delete(projectID, name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := waitComputeGlobalOp(ctx, svc, projectID, op); err != nil {
		return false, err
	}
	return true, nil
}

func deleteRouterNAT(ctx context.Context, svc *compute.Service, projectID, region, routerName, natName string) (bool, error) {
	router, err := svc.Routers.Get(projectID, region, routerName).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	filtered := make([]*compute.RouterNat, 0, len(router.Nats))
	removed := false
	for _, n := range router.Nats {
		if n.Name == natName {
			removed = true
			continue
		}
		filtered = append(filtered, n)
	}
	if !removed {
		return false, nil
	}
	router.Nats = filtered
	op, err := svc.Routers.Patch(projectID, region, routerName, router).Context(ctx).Do()
	if err != nil {
		return false, err
	}
	if err := waitComputeRegionOp(ctx, svc, projectID, region, op); err != nil {
		return false, err
	}
	return true, nil
}

func deleteAndWaitRouter(ctx context.Context, svc *compute.Service, projectID, region, name string) (bool, error) {
	op, err := svc.Routers.Delete(projectID, region, name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := waitComputeRegionOp(ctx, svc, projectID, region, op); err != nil {
		return false, err
	}
	return true, nil
}

func deleteAndWaitSubnet(ctx context.Context, svc *compute.Service, projectID, region, name string) (bool, error) {
	op, err := svc.Subnetworks.Delete(projectID, region, name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := waitComputeRegionOp(ctx, svc, projectID, region, op); err != nil {
		return false, err
	}
	return true, nil
}

func deleteAndWaitVPC(ctx context.Context, svc *compute.Service, projectID, name string) (bool, error) {
	op, err := svc.Networks.Delete(projectID, name).Context(ctx).Do()
	if err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := waitComputeGlobalOp(ctx, svc, projectID, op); err != nil {
		return false, err
	}
	return true, nil
}

func waitComputeZoneOp(ctx context.Context, svc *compute.Service, projectID, zone string, op *compute.Operation) error {
	for i := 0; i < asyncOpMaxPolls; i++ {
		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("compute zone op error: %s", op.Error.Errors[0].Message)
			}
			return nil
		}
		time.Sleep(asyncOpPollInterval)
		fresh, err := svc.ZoneOperations.Get(projectID, zone, op.Name).Context(ctx).Do()
		if err != nil {
			return err
		}
		op = fresh
	}
	return fmt.Errorf("compute zone op %s did not finish within %s", op.Name, asyncOpPollInterval*asyncOpMaxPolls)
}

func waitComputeRegionOp(ctx context.Context, svc *compute.Service, projectID, region string, op *compute.Operation) error {
	for i := 0; i < asyncOpMaxPolls; i++ {
		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("compute region op error: %s", op.Error.Errors[0].Message)
			}
			return nil
		}
		time.Sleep(asyncOpPollInterval)
		fresh, err := svc.RegionOperations.Get(projectID, region, op.Name).Context(ctx).Do()
		if err != nil {
			return err
		}
		op = fresh
	}
	return fmt.Errorf("compute region op %s did not finish within %s", op.Name, asyncOpPollInterval*asyncOpMaxPolls)
}

// resolveProjectNumber looks up the numeric project number for a given project ID.
func resolveProjectNumber(ctx context.Context, projectID string, opts ...option.ClientOption) (string, error) {
	crmSvc, err := crm.NewService(ctx, opts...)
	if err != nil {
		return "", fmt.Errorf("cloudresourcemanager client: %w", err)
	}
	proj, err := crmSvc.Projects.Get(projectID).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("get project %s: %w", projectID, err)
	}
	return fmt.Sprintf("%d", proj.ProjectNumber), nil
}

// probeOrphanBuckets returns the names of GCS buckets that already exist for
// (projectID, region, stage) AND should be adopted-not-deleted by the new
// module. This is the set the root-module `import { for_each = ... }` blocks
// iterate over; the same value is exposed at plan time by ModifyPlan and at
// apply time by Create.
//
// CONTRACT: only `-audit-logs` buckets are reported. The legacy
// `-trend-resources` bucket lives in a DIFFERENT location in the new
// architecture (`shared_resources/` with `-shared` suffix), has no matching
// import target in the per-member `module/storage`, and its content is
// TF-regenerated (CF source archives, scripts) — so cleanup_region deletes it
// inline rather than reporting it for import. Mixing both into one list would
// (and did, in earlier development) cause both imports to target the single
// `audit_logs_bucket[0]` instance, importing the wrong bucket.
//
// Returns ([], nil) for fresh installs. Read-only — uses storage.buckets.get
// covered by roles/viewer.
func probeOrphanBuckets(ctx context.Context, projectID, region, stage string, opts ...option.ClientOption) ([]string, error) {
	pfx := fmt.Sprintf("%s%s-%s", config.LEGACY_GCP_DSPM_NAME_BASE, stageNameToLetter(stage), regionAbbreviation(region))

	storageSvc, err := storagev1.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}
	projectNumber, err := resolveProjectNumber(ctx, projectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("resolve project number: %w", err)
	}

	auditBucket := fmt.Sprintf("%s-%s-audit-logs", pfx, projectNumber)
	exists, err := gcsBucketExists(ctx, storageSvc, auditBucket)
	if err != nil {
		return nil, fmt.Errorf("probe %s: %w", auditBucket, err)
	}
	if !exists {
		return nil, nil
	}
	return []string{auditBucket}, nil
}

// deleteGCSBucketIfExists empties + deletes a GCS bucket. Treats 404 as
// success (already gone).
//
// Race: Cloud Build streams compile-job logs into this bucket (under
// container-builds/logs/) at ~1 write/sec while a CF deployment is
// in-flight. If we empty+delete while a build is running, log writes
// race the delete → 409 not-empty.
//
// Solution: caller MUST run cancelActiveCloudBuilds before this. With
// builds cancelled the writes stop within a few seconds; this function
// then does a single empty+delete pass with a tiny retry budget (3) to
// absorb the SIGKILL-to-write-stop lag. No IAM lock, no propagation
// polling — just kill the writer + do the work.
func deleteGCSBucketIfExists(ctx context.Context, svc *storagev1.Service, bucketName string) (bool, error) {
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := emptyGCSBucket(ctx, svc, bucketName); err != nil {
			if isGCSNotFound(err) {
				return false, nil
			}
			return false, err
		}
		err := svc.Buckets.Delete(bucketName).Context(ctx).Do()
		if err == nil {
			return true, nil
		}
		if isGCSNotFound(err) {
			return false, nil
		}
		if !isGCSBucketNotEmpty(err) {
			return false, fmt.Errorf("delete bucket %s: %w", bucketName, err)
		}
		if attempt == maxAttempts {
			return false, fmt.Errorf("delete bucket %s — 409 not-empty after %d empty+delete cycles and Cloud Build cancellation. A writer outside the standard set (Cloud Build / CF deploy) is still active. Identify and stop it — TF apply retry won't converge", bucketName, attempt)
		}
		tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] bucket %s 409 not-empty (attempt %d/%d) — re-emptying and retrying", bucketName, attempt, maxAttempts))
	}
	return false, fmt.Errorf("delete bucket %s: unreachable", bucketName)
}

// cancelActiveCloudBuilds cancels every WORKING / QUEUED Cloud Build in the
// project AND actively polls each cancelled build until its status leaves
// WORKING/QUEUED (i.e. the worker has actually stopped writing). Cloud
// Build's Cancel RPC is async: the API accepts the request immediately
// but the worker keeps running — and streaming logs to GCS — for ~10-30s
// until SIGKILL takes effect. Without the post-cancel poll, the bucket
// cleanup races those log writes.
//
// Project-wide scope is intentional — Cloud Build jobs are identified by
// UUID not by our prefix. We only cancel WORKING/QUEUED; terminal states
// (SUCCESS / FAILURE / CANCELLED) are no-op. Returns number of builds we
// confirmed have stopped.
//
// Active wait budget: 2 min. Each poll is one Get RPC per cancelled build
// — actual work, not blind sleep. Beyond 2 min something is structurally
// wrong (locked build worker, networking issue) and waiting longer just
// hides the diagnosis.
func cancelActiveCloudBuilds(ctx context.Context, projectID string, opts ...option.ClientOption) (int, error) {
	svc, err := cloudbuild.NewService(ctx, opts...)
	if err != nil {
		return 0, fmt.Errorf("cloudbuild client: %w", err)
	}

	cancelledIDs := []string{}
	for _, statusFilter := range []string{"status=WORKING", "status=QUEUED"} {
		var pageToken string
		for {
			req := svc.Projects.Builds.List(projectID).Filter(statusFilter).PageSize(50)
			if pageToken != "" {
				req = req.PageToken(pageToken)
			}
			resp, err := req.Context(ctx).Do()
			if err != nil {
				return len(cancelledIDs), fmt.Errorf("list builds (%s): %w", statusFilter, err)
			}
			for _, b := range resp.Builds {
				if _, err := svc.Projects.Builds.Cancel(projectID, b.Id, &cloudbuild.CancelBuildRequest{}).Context(ctx).Do(); err != nil {
					tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] cancel build %s: %v", b.Id, err))
					continue
				}
				cancelledIDs = append(cancelledIDs, b.Id)
			}
			if resp.NextPageToken == "" {
				break
			}
			pageToken = resp.NextPageToken
		}
	}

	if len(cancelledIDs) == 0 {
		return 0, nil
	}

	// Active wait: poll each cancelled build until its status leaves
	// WORKING/QUEUED. Then we know the worker has stopped streaming logs.
	deadline := time.Now().Add(2 * time.Minute)
	pending := make(map[string]struct{}, len(cancelledIDs))
	for _, id := range cancelledIDs {
		pending[id] = struct{}{}
	}
	attempt := 0
	for len(pending) > 0 {
		attempt++
		for id := range pending {
			b, err := svc.Projects.Builds.Get(projectID, id).Context(ctx).Do()
			if err != nil {
				// Treat error as "can't tell" — leave in pending; deadline catches it.
				continue
			}
			if b.Status != "WORKING" && b.Status != "QUEUED" {
				delete(pending, id)
			}
		}
		if len(pending) == 0 {
			tflog.Info(ctx, fmt.Sprintf("[DSPM Region Cleanup] %d Cloud Build(s) confirmed stopped (attempt %d)", len(cancelledIDs), attempt))
			break
		}
		if time.Now().After(deadline) {
			tflog.Warn(ctx, fmt.Sprintf("[DSPM Region Cleanup] %d Cloud Build(s) still WORKING/QUEUED after 2min cancel wait — proceeding anyway", len(pending)))
			break
		}
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return len(cancelledIDs) - len(pending), ctx.Err()
		}
	}
	return len(cancelledIDs), nil
}

// emptyGCSBucket lists+deletes ALL objects in a bucket, including noncurrent
// (versioned) generations. The default Objects.List omits noncurrent
// versions, so a bucket that ever had versioning enabled — even if
// versioning is now Suspended — keeps its old generations as ghost
// objects that block Bucket.Delete with 409 not-empty (root cause of
// the trend-resources delete failures: ~30 noncurrent log-*.txt
// generations from prior Cloud Build streaming runs).
//
// `Versions(true)` makes List include both current AND noncurrent
// generations. For each returned object we delete the SPECIFIC
// generation via .Generation(g) — without it, Delete only removes the
// live version, leaving noncurrent ones to block bucket delete.
func emptyGCSBucket(ctx context.Context, svc *storagev1.Service, bucketName string) error {
	var pageToken string
	for {
		req := svc.Objects.List(bucketName).Versions(true)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		objs, err := req.Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("list objects in %s: %w", bucketName, err)
		}
		for _, obj := range objs.Items {
			delErr := svc.Objects.Delete(bucketName, obj.Name).Generation(obj.Generation).Context(ctx).Do()
			if delErr != nil && !isGCSNotFound(delErr) {
				return fmt.Errorf("delete object %s/%s#%d: %w", bucketName, obj.Name, obj.Generation, delErr)
			}
		}
		if objs.NextPageToken == "" {
			return nil
		}
		pageToken = objs.NextPageToken
	}
}

// isGCSBucketNotEmpty matches GCP's 409 "bucket not empty" response.
func isGCSBucketNotEmpty(err error) bool {
	if err == nil {
		return false
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 409 && strings.Contains(gerr.Message, "not empty")
	}
	return strings.Contains(err.Error(), "is not empty")
}

// gcsBucketExists returns true if the named bucket is present in GCP, false on 404,
// and an error otherwise. Read-only — uses storage.buckets.get (covered by roles/viewer).
func gcsBucketExists(ctx context.Context, svc *storagev1.Service, bucketName string) (bool, error) {
	if _, err := svc.Buckets.Get(bucketName).Context(ctx).Do(); err != nil {
		if isGCSNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// deleteLoggingSink deletes a project-level log router sink. Treats 404 as success.
// sinkName must be the fully qualified resource name: projects/{project}/sinks/{sink}.
func deleteLoggingSink(ctx context.Context, svc *logging.Service, sinkName string) (bool, error) {
	if _, err := svc.Projects.Sinks.Delete(sinkName).Context(ctx).Do(); err != nil {
		if isGCPNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("delete sink %s: %w", sinkName, err)
	}
	return true, nil
}

// isGCSNotFound checks for GCS 404 responses.
func isGCSNotFound(err error) bool {
	if err == nil {
		return false
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 404
	}
	return strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "notFound")
}

func waitComputeGlobalOp(ctx context.Context, svc *compute.Service, projectID string, op *compute.Operation) error {
	for i := 0; i < asyncOpMaxPolls; i++ {
		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("compute global op error: %s", op.Error.Errors[0].Message)
			}
			return nil
		}
		time.Sleep(asyncOpPollInterval)
		fresh, err := svc.GlobalOperations.Get(projectID, op.Name).Context(ctx).Do()
		if err != nil {
			return err
		}
		op = fresh
	}
	return fmt.Errorf("compute global op %s did not finish within %s", op.Name, asyncOpPollInterval*asyncOpMaxPolls)
}
