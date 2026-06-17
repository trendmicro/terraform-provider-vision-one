package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	cloudfunctions "google.golang.org/api/cloudfunctions/v2"
	scheduler "google.golang.org/api/cloudscheduler/v1"
	compute "google.golang.org/api/compute/v1"
	crm "google.golang.org/api/cloudresourcemanager/v1"
	eventarc "google.golang.org/api/eventarc/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	run "google.golang.org/api/run/v2"
	storagev1 "google.golang.org/api/storage/v1"
	vpcaccess "google.golang.org/api/vpcaccess/v1"
)

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
}

type dspmRegionCleanupResult struct {
	ResourcesDeleted map[string]int
	SnapshotName     string
}

const (
	// 30 polls × 10s matches the bash retry loop (`for i in $(seq 1 30); do … sleep 10; done`).
	asyncOpPollInterval = 10 * time.Second
	asyncOpMaxPolls     = 30
)

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
			"buckets":           0,
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

		instances, err := listDSPMInstances(ctx, cSvc, opts.ProjectID, opts.Region)
		noteErr("instances_list", opts.Region, err)
		for _, inst := range instances {
			err := deleteAndWaitComputeInstance(ctx, cSvc, opts.ProjectID, inst.zone, inst.name)
			tally("vms", err == nil)
			noteErr("vm", inst.name, err)
		}
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

	// Delete orphaned new-module GCS buckets (audit_logs + trend_resources) that may
	// have been created by a prior partial apply and left without TF state. These use
	// the new naming: {namePrefix}-{projectNumber}-{audit-logs,trend-resources}.
	// A fresh install has no such buckets — the GCS 404 is silent success.
	if storageSvc, err := storagev1.NewService(ctx, opts.ClientOptions...); err != nil {
		errs = append(errs, fmt.Sprintf("storage client: %v", err))
	} else {
		projectNumber, numErr := resolveProjectNumber(ctx, opts.ProjectID, opts.ClientOptions...)
		if numErr != nil {
			errs = append(errs, fmt.Sprintf("resolve project number: %v", numErr))
		} else {
			for _, suffix := range []string{"-audit-logs", "-trend-resources"} {
				bucketName := fmt.Sprintf("%s-%s%s", pfx, projectNumber, suffix)
				deleted, err := deleteGCSBucketIfExists(ctx, storageSvc, bucketName)
				tally("buckets", deleted)
				noteErr("bucket", bucketName, err)
			}
		}
	}

	var combinedErr error
	if len(errs) > 0 {
		combinedErr = errors.New(strings.Join(errs, "; "))
	}
	return result, combinedErr
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

// deleteGCSBucketIfExists empties and deletes a GCS bucket. Treats 404 as success (already gone).
func deleteGCSBucketIfExists(ctx context.Context, svc *storagev1.Service, bucketName string) (bool, error) {
	// List and delete all objects first (bucket must be empty before deletion).
	var pageToken string
	for {
		req := svc.Objects.List(bucketName)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		objs, err := req.Context(ctx).Do()
		if err != nil {
			if isGCSNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("list objects in %s: %w", bucketName, err)
		}
		for _, obj := range objs.Items {
			if delErr := svc.Objects.Delete(bucketName, obj.Name).Context(ctx).Do(); delErr != nil && !isGCSNotFound(delErr) {
				return false, fmt.Errorf("delete object %s/%s: %w", bucketName, obj.Name, delErr)
			}
		}
		if objs.NextPageToken == "" {
			break
		}
		pageToken = objs.NextPageToken
	}
	if err := svc.Buckets.Delete(bucketName).Context(ctx).Do(); err != nil {
		if isGCSNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("delete bucket %s: %w", bucketName, err)
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
