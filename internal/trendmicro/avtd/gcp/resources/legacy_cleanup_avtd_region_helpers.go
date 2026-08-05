package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"terraform-provider-vision-one/internal/trendmicro/avtd/gcp/resources/config"
	camconfig "terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	cloudbuild "google.golang.org/api/cloudbuild/v1"
	crm "google.golang.org/api/cloudresourcemanager/v1"
	scheduler "google.golang.org/api/cloudscheduler/v1"
	compute "google.golang.org/api/compute/v1"
	eventarc "google.golang.org/api/eventarc/v1"
	firestore "google.golang.org/api/firestore/v1"
	"google.golang.org/api/googleapi"
	logging "google.golang.org/api/logging/v2"
	"google.golang.org/api/option"
	pubsub "google.golang.org/api/pubsub/v1"
	run "google.golang.org/api/run/v2"
	secretmanager "google.golang.org/api/secretmanager/v1"
	storagev1 "google.golang.org/api/storage/v1"
	workflows "google.golang.org/api/workflows/v1"
)

const (
	cleanupPermsWaitMaxDuration = 3 * time.Minute
	cleanupPermsPollStart       = 5 * time.Second
	cleanupPermsPollCap         = 20 * time.Second
	cleanupPermsPollFactor      = 2.0

	cleanupServiceCacheMaxDuration = 90 * time.Second
	cleanupServiceCachePollStart   = 5 * time.Second
	cleanupServiceCachePollCap     = 15 * time.Second

	asyncOpPollInterval = 10 * time.Second
	asyncOpMaxPolls     = 30
)

type avtdRegionCleanupOptions struct {
	ProjectID              string
	CustomerProjectID      string
	Region                 string
	Prefixes               []string
	PreserveResourceBucket bool
	PreserveVPC            bool
	PreserveFirestore      bool
	ClientOptions          []option.ClientOption
}

type avtdRegionCleanupResult struct {
	ResourcesDeleted map[string]int
	OrphanBuckets    []string
}

func matchesAnyPrefix(resourceName string, prefixes []string) bool {
	short := resourceName
	if i := strings.LastIndex(short, "/"); i >= 0 {
		short = short[i+1:]
	}
	for _, p := range prefixes {
		if strings.HasPrefix(short, p+"-") {
			return true
		}
	}
	return false
}

func isResourceBucket(bucketName, region string, prefixes []string) bool {
	if !matchesAnyPrefix(bucketName, prefixes) {
		return false
	}
	return strings.Contains(bucketName, config.RESOURCE_BUCKET_INFIX) && strings.Contains(bucketName, "-"+region+"-")
}

func isAccessLogsBucket(bucketName, region string, prefixes []string) bool {
	if !matchesAnyPrefix(bucketName, prefixes) {
		return false
	}
	return strings.Contains(bucketName, config.ACCESS_LOGS_BUCKET_INFIX) && strings.Contains(bucketName, "-"+region+"-")
}

func runAVTDRegionCleanup(ctx context.Context, opts avtdRegionCleanupOptions) (avtdRegionCleanupResult, error) {
	result := avtdRegionCleanupResult{ResourcesDeleted: newCleanupTally()}

	if !probeForLegacyAVTDResources(ctx, opts.ProjectID, opts.Region, opts.Prefixes, opts.ClientOptions...) {
		tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] no legacy resources found on %s/%s — skipping IAM wait", opts.ProjectID, opts.Region))
		if opts.PreserveResourceBucket {
			if orphans, perr := probeOrphanResourceBuckets(ctx, opts.ProjectID, opts.Region, opts.Prefixes, opts.ClientOptions...); perr != nil {
				tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] fast-path orphan bucket probe failed for %s/%s: %v", opts.ProjectID, opts.Region, perr))
			} else {
				result.OrphanBuckets = orphans
			}
		}
		return result, nil
	}

	if err := waitForCleanupPermsReady(ctx, opts.ProjectID, opts.ClientOptions...); err != nil {
		return result, err
	}
	if err := warmupServiceCaches(ctx, opts.ProjectID, opts.Region, opts.ClientOptions...); err != nil {
		return result, err
	}

	var errs []string
	errs = append(errs, cleanupEventarcTriggers(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupCloudRun(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupSchedulerJobs(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupPubSub(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupWorkflows(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupLoggingSinks(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupSecrets(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupNetworking(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupFirestore(ctx, opts, result.ResourcesDeleted)...)
	errs = append(errs, cleanupBuckets(ctx, opts, &result)...)

	if cust := opts.CustomerProjectID; cust != "" && cust != opts.ProjectID {
		custOpts := opts
		custOpts.ProjectID = cust
		if permErr := waitForCleanupPermsReady(ctx, cust, opts.ClientOptions...); permErr != nil {
			errs = append(errs, fmt.Sprintf("customer-project cleanup perms not ready (%s): %v", cust, permErr))
		} else {
			if werr := warmupServiceCaches(ctx, cust, opts.Region, opts.ClientOptions...); werr != nil {
				tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] customer-project warmup %s: %v", cust, werr))
			}
			errs = append(errs, cleanupEventarcTriggers(ctx, custOpts, result.ResourcesDeleted)...)
			errs = append(errs, cleanupCloudRun(ctx, custOpts, result.ResourcesDeleted)...)
			errs = append(errs, cleanupSchedulerJobs(ctx, custOpts, result.ResourcesDeleted)...)
			errs = append(errs, cleanupPubSub(ctx, custOpts, result.ResourcesDeleted)...)
			errs = append(errs, cleanupWorkflows(ctx, custOpts, result.ResourcesDeleted)...)
			errs = append(errs, cleanupLoggingSinks(ctx, custOpts, result.ResourcesDeleted)...)
			errs = append(errs, cleanupSecrets(ctx, custOpts, result.ResourcesDeleted)...)
		}
	}

	var combinedErr error
	if len(errs) > 0 {
		combinedErr = errors.New(strings.Join(errs, "; "))
	}
	return result, combinedErr
}

func newCleanupTally() map[string]int {
	return map[string]int{
		"triggers":                 0,
		"run_services":             0,
		"run_jobs":                 0,
		"schedulers":               0,
		"subscriptions":            0,
		"topics":                   0,
		"workflows":                0,
		"sinks":                    0,
		"secrets":                  0,
		"firewalls":                0,
		"subnets":                  0,
		"networks":                 0,
		"firestore_databases":      0,
		"firestore_preserved":      0,
		"buckets":                  0,
		"orphan_buckets_preserved": 0,
	}
}

func noteErr(ctx context.Context, family, name string, err error) string {
	if err == nil || isGCPNotFound(err) || isRegionUnsupported(err) {
		return ""
	}
	tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] %s/%s failed: %v", family, name, err))
	return fmt.Sprintf("%s/%s: %v", family, name, err)
}

func cleanupEventarcTriggers(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	parent := fmt.Sprintf("projects/%s/locations/%s", opts.ProjectID, opts.Region)
	svc, err := eventarc.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("eventarc client: %v", err)}
	}
	listResp, err := svc.Projects.Locations.Triggers.List(parent).Context(ctx).Do()
	if err != nil {
		return collect(noteErr(ctx, "triggers_list", opts.Region, err))
	}
	var errs []string
	for _, t := range listResp.Triggers {
		if !matchesAnyPrefix(t.Name, opts.Prefixes) {
			continue
		}
		_, delErr := svc.Projects.Locations.Triggers.Delete(t.Name).Context(ctx).Do()
		if delErr == nil || isGCPNotFound(delErr) {
			tally["triggers"]++
			continue
		}
		errs = append(errs, noteErr(ctx, "trigger", t.Name, delErr))
	}
	return errs
}

func cleanupCloudRun(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	parent := fmt.Sprintf("projects/%s/locations/%s", opts.ProjectID, opts.Region)
	svc, err := run.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("run client: %v", err)}
	}
	var errs []string
	if listResp, listErr := svc.Projects.Locations.Services.List(parent).Context(ctx).Do(); listErr != nil {
		errs = append(errs, noteErr(ctx, "run_services_list", opts.Region, listErr))
	} else {
		for _, s := range listResp.Services {
			if !matchesAnyPrefix(s.Name, opts.Prefixes) {
				continue
			}
			_, delErr := svc.Projects.Locations.Services.Delete(s.Name).Context(ctx).Do()
			if delErr == nil || isGCPNotFound(delErr) {
				tally["run_services"]++
				continue
			}
			errs = append(errs, noteErr(ctx, "run_service", s.Name, delErr))
		}
	}
	if listResp, listErr := svc.Projects.Locations.Jobs.List(parent).Context(ctx).Do(); listErr != nil {
		errs = append(errs, noteErr(ctx, "run_jobs_list", opts.Region, listErr))
	} else {
		for _, j := range listResp.Jobs {
			if !matchesAnyPrefix(j.Name, opts.Prefixes) {
				continue
			}
			_, delErr := svc.Projects.Locations.Jobs.Delete(j.Name).Context(ctx).Do()
			if delErr == nil || isGCPNotFound(delErr) {
				tally["run_jobs"]++
				continue
			}
			errs = append(errs, noteErr(ctx, "run_job", j.Name, delErr))
		}
	}
	return errs
}

func cleanupSchedulerJobs(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	parent := fmt.Sprintf("projects/%s/locations/%s", opts.ProjectID, opts.Region)
	svc, err := scheduler.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("scheduler client: %v", err)}
	}

	var jobNames []string
	listErr := svc.Projects.Locations.Jobs.List(parent).Pages(ctx, func(page *scheduler.ListJobsResponse) error {
		for _, j := range page.Jobs {
			if matchesAnyPrefix(j.Name, opts.Prefixes) {
				jobNames = append(jobNames, j.Name)
			}
		}
		return nil
	})
	if listErr != nil {
		return collect(noteErr(ctx, "schedulers_list", opts.Region, listErr))
	}

	var errs []string
	for _, name := range jobNames {
		_, delErr := svc.Projects.Locations.Jobs.Delete(name).Context(ctx).Do()
		if delErr == nil || isGCPNotFound(delErr) {
			tally["schedulers"]++
			continue
		}
		errs = append(errs, noteErr(ctx, "scheduler", name, delErr))
	}
	return errs
}

func cleanupPubSub(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	projParent := fmt.Sprintf("projects/%s", opts.ProjectID)
	svc, err := pubsub.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("pubsub client: %v", err)}
	}
	var errs []string
	if listResp, listErr := svc.Projects.Subscriptions.List(projParent).Context(ctx).Do(); listErr != nil {
		errs = append(errs, noteErr(ctx, "subscriptions_list", opts.Region, listErr))
	} else {
		for _, s := range listResp.Subscriptions {
			if !matchesAnyPrefix(s.Name, opts.Prefixes) {
				continue
			}
			_, delErr := svc.Projects.Subscriptions.Delete(s.Name).Context(ctx).Do()
			if delErr == nil || isGCPNotFound(delErr) {
				tally["subscriptions"]++
				continue
			}
			errs = append(errs, noteErr(ctx, "subscription", s.Name, delErr))
		}
	}
	if listResp, listErr := svc.Projects.Topics.List(projParent).Context(ctx).Do(); listErr != nil {
		errs = append(errs, noteErr(ctx, "topics_list", opts.Region, listErr))
	} else {
		for _, t := range listResp.Topics {
			if !matchesAnyPrefix(t.Name, opts.Prefixes) {
				continue
			}
			_, delErr := svc.Projects.Topics.Delete(t.Name).Context(ctx).Do()
			if delErr == nil || isGCPNotFound(delErr) {
				tally["topics"]++
				continue
			}
			errs = append(errs, noteErr(ctx, "topic", t.Name, delErr))
		}
	}
	return errs
}

func cleanupWorkflows(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	parent := fmt.Sprintf("projects/%s/locations/%s", opts.ProjectID, opts.Region)
	svc, err := workflows.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("workflows client: %v", err)}
	}
	listResp, err := svc.Projects.Locations.Workflows.List(parent).Context(ctx).Do()
	if err != nil {
		return collect(noteErr(ctx, "workflows_list", opts.Region, err))
	}
	var errs []string
	for _, w := range listResp.Workflows {
		if !matchesAnyPrefix(w.Name, opts.Prefixes) {
			continue
		}
		_, delErr := svc.Projects.Locations.Workflows.Delete(w.Name).Context(ctx).Do()
		if delErr == nil || isGCPNotFound(delErr) {
			tally["workflows"]++
			continue
		}
		errs = append(errs, noteErr(ctx, "workflow", w.Name, delErr))
	}
	return errs
}

func cleanupLoggingSinks(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	projParent := fmt.Sprintf("projects/%s", opts.ProjectID)
	svc, err := logging.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("logging client: %v", err)}
	}
	listResp, err := svc.Projects.Sinks.List(projParent).Context(ctx).Do()
	if err != nil {
		return collect(noteErr(ctx, "sinks_list", opts.Region, err))
	}
	var errs []string
	for _, s := range listResp.Sinks {
		if !matchesAnyPrefix(s.Name, opts.Prefixes) {
			continue
		}
		sinkName := fmt.Sprintf("%s/sinks/%s", projParent, s.Name)
		_, delErr := svc.Projects.Sinks.Delete(sinkName).Context(ctx).Do()
		if delErr == nil || isGCPNotFound(delErr) {
			tally["sinks"]++
			continue
		}
		errs = append(errs, noteErr(ctx, "sink", sinkName, delErr))
	}
	return errs
}

func cleanupSecrets(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	projParent := fmt.Sprintf("projects/%s", opts.ProjectID)
	svc, err := secretmanager.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("secretmanager client: %v", err)}
	}
	listResp, err := svc.Projects.Secrets.List(projParent).Context(ctx).Do()
	if err != nil {
		return collect(noteErr(ctx, "secrets_list", opts.Region, err))
	}
	var errs []string
	for _, s := range listResp.Secrets {
		if !matchesAnyPrefix(s.Name, opts.Prefixes) {
			continue
		}
		_, delErr := svc.Projects.Secrets.Delete(s.Name).Context(ctx).Do()
		if delErr == nil || isGCPNotFound(delErr) {
			tally["secrets"]++
			continue
		}
		errs = append(errs, noteErr(ctx, "secret", s.Name, delErr))
	}
	return errs
}

func cleanupNetworking(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	if opts.PreserveVPC {
		tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] preserve_vpc=true — skipping firewall/subnet/network deletion on %s/%s (adopted via import)", opts.ProjectID, opts.Region))
		return nil
	}

	svc, err := compute.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("compute client: %v", err)}
	}
	var errs []string

	if fwList, listErr := svc.Firewalls.List(opts.ProjectID).Context(ctx).Do(); listErr != nil {
		errs = append(errs, noteErr(ctx, "firewalls_list", opts.Region, listErr))
	} else {
		for _, fw := range fwList.Items {
			if !matchesAnyPrefix(fw.Name, opts.Prefixes) {
				continue
			}
			op, delErr := svc.Firewalls.Delete(opts.ProjectID, fw.Name).Context(ctx).Do()
			if delErr != nil && !isGCPNotFound(delErr) {
				errs = append(errs, noteErr(ctx, "firewall", fw.Name, delErr))
				continue
			}
			if delErr == nil {
				errs = appendErr(errs, noteErr(ctx, "firewall", fw.Name, waitComputeGlobalOp(ctx, svc, opts.ProjectID, op)))
			}
			tally["firewalls"]++
		}
	}

	if snList, listErr := svc.Subnetworks.List(opts.ProjectID, opts.Region).Context(ctx).Do(); listErr != nil {
		errs = append(errs, noteErr(ctx, "subnets_list", opts.Region, listErr))
	} else {
		for _, sn := range snList.Items {
			if !matchesAnyPrefix(sn.Name, opts.Prefixes) {
				continue
			}
			op, delErr := svc.Subnetworks.Delete(opts.ProjectID, opts.Region, sn.Name).Context(ctx).Do()
			if delErr != nil && !isGCPNotFound(delErr) {
				errs = append(errs, noteErr(ctx, "subnet", sn.Name, delErr))
				continue
			}
			if delErr == nil {
				errs = appendErr(errs, noteErr(ctx, "subnet", sn.Name, waitComputeRegionOp(ctx, svc, opts.ProjectID, opts.Region, op)))
			}
			tally["subnets"]++
		}
	}

	if nwList, listErr := svc.Networks.List(opts.ProjectID).Context(ctx).Do(); listErr != nil {
		errs = append(errs, noteErr(ctx, "networks_list", opts.Region, listErr))
	} else {
		for _, nw := range nwList.Items {
			if !matchesAnyPrefix(nw.Name, opts.Prefixes) {
				continue
			}
			op, delErr := svc.Networks.Delete(opts.ProjectID, nw.Name).Context(ctx).Do()
			if delErr != nil && !isGCPNotFound(delErr) {
				errs = append(errs, noteErr(ctx, "network", nw.Name, delErr))
				continue
			}
			if delErr == nil {
				errs = appendErr(errs, noteErr(ctx, "network", nw.Name, waitComputeGlobalOp(ctx, svc, opts.ProjectID, op)))
			}
			tally["networks"]++
		}
	}
	return errs
}

func cleanupFirestore(ctx context.Context, opts avtdRegionCleanupOptions, tally map[string]int) []string {
	projParent := fmt.Sprintf("projects/%s", opts.ProjectID)
	svc, err := firestore.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("firestore client: %v", err)}
	}
	listResp, err := svc.Projects.Databases.List(projParent).Context(ctx).Do()
	if err != nil {
		return collect(noteErr(ctx, "firestore_list", opts.Region, err))
	}
	var errs []string
	for _, db := range listResp.Databases {
		if !matchesAnyPrefix(db.Name, opts.Prefixes) {
			continue
		}
		consolidated := isConsolidatedScanTrackingDB(db.Name, opts.Prefixes)

		if consolidated && opts.PreserveFirestore {
			tally["firestore_preserved"]++
			continue
		}

		if !consolidated && !isThisRegionScanTrackingDB(db.Name, opts.Prefixes, opts.Region) {
			continue
		}

		if delErr := deleteFirestoreDBWithRetry(ctx, svc, db.Name); delErr != nil {
			errs = append(errs, noteErr(ctx, "firestore_database", db.Name, delErr))
			continue
		}
		tally["firestore_databases"]++
	}
	return errs
}

const (
	firestoreDeleteMaxAttempts = 6
	firestoreDeleteRetryStart  = 3 * time.Second
	firestoreDeleteRetryCap    = 20 * time.Second
)

func deleteFirestoreDBWithRetry(ctx context.Context, svc *firestore.Service, dbName string) error {
	backoff := firestoreDeleteRetryStart
	var lastErr error
	for attempt := 1; attempt <= firestoreDeleteMaxAttempts; attempt++ {
		_, err := svc.Projects.Databases.Delete(dbName).Context(ctx).Do()
		if err == nil || isGCPNotFound(err) {
			return nil
		}
		if !isFirestoreConcurrentChange(err) {
			return err
		}
		lastErr = err
		tflog.Warn(ctx, fmt.Sprintf(
			"[AVTD Region Cleanup] Firestore delete %s hit a concurrent-change 409 (attempt %d/%d); retrying in %s",
			dbName, attempt, firestoreDeleteMaxAttempts, backoff,
		))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff = time.Duration(float64(backoff) * 2); backoff > firestoreDeleteRetryCap {
			backoff = firestoreDeleteRetryCap
		}
	}
	return lastErr
}

func isFirestoreConcurrentChange(err error) bool {
	if err == nil {
		return false
	}
	var gErr *googleapi.Error
	if errors.As(err, &gErr) && gErr.Code == 409 && strings.Contains(strings.ToLower(gErr.Message), "concurrent") {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "concurrent database changes")
}

func isThisRegionScanTrackingDB(resourceName string, prefixes []string, region string) bool {
	short := resourceName
	if i := strings.LastIndex(short, "/"); i >= 0 {
		short = short[i+1:]
	}
	for _, p := range prefixes {
		if short == fmt.Sprintf("%s-scan-tracking-%s", p, region) {
			return true
		}
	}
	return false
}

func isConsolidatedScanTrackingDB(resourceName string, prefixes []string) bool {
	short := resourceName
	if i := strings.LastIndex(short, "/"); i >= 0 {
		short = short[i+1:]
	}
	for _, p := range prefixes {
		if short == p+"-scan-tracking" {
			return true
		}
	}
	return false
}

func cleanupBuckets(ctx context.Context, opts avtdRegionCleanupOptions, result *avtdRegionCleanupResult) []string {
	svc, err := storagev1.NewService(ctx, opts.ClientOptions...)
	if err != nil {
		return []string{fmt.Sprintf("storage client: %v", err)}
	}
	bktList, err := svc.Buckets.List(opts.ProjectID).Context(ctx).Do()
	if err != nil {
		return collect(noteErr(ctx, "buckets_list", opts.Region, err))
	}

	if cancelled, cancelErr := cancelActiveCloudBuilds(ctx, opts.ProjectID, opts.ClientOptions...); cancelErr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] cancel builds best-effort: %v", cancelErr))
	} else if cancelled > 0 {
		tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] cancelled %d in-flight Cloud Build(s) on %s before bucket cleanup", cancelled, opts.ProjectID))
	}

	var errs []string
	for _, b := range bktList.Items {
		if !matchesAnyPrefix(b.Name, opts.Prefixes) {
			continue
		}
		if !strings.Contains(b.Name, "-"+opts.Region+"-") {
			continue
		}
		if opts.PreserveResourceBucket && isResourceBucket(b.Name, opts.Region, opts.Prefixes) {
			result.OrphanBuckets = append(result.OrphanBuckets, b.Name)
			result.ResourcesDeleted["orphan_buckets_preserved"]++
			tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] scan resource bucket preserved for new-module import: %s", b.Name))
			continue
		}
		if opts.PreserveResourceBucket && isAccessLogsBucket(b.Name, opts.Region, opts.Prefixes) {
			result.ResourcesDeleted["orphan_buckets_preserved"]++
			tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] access-logs bucket preserved for new-module import: %s", b.Name))
			continue
		}
		deleted, delErr := deleteGCSBucketIfExists(ctx, svc, b.Name)
		if deleted {
			result.ResourcesDeleted["buckets"]++
		}
		errs = appendErr(errs, noteErr(ctx, "bucket", b.Name, delErr))
	}
	return errs
}

func collect(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func appendErr(errs []string, s string) []string {
	if s == "" {
		return errs
	}
	return append(errs, s)
}

func probeForLegacyAVTDResources(ctx context.Context, projectID, region string, prefixes []string, opts ...option.ClientOption) bool {
	cSvc, err := compute.NewService(ctx, opts...)
	if err != nil {
		return true
	}
	nwList, err := cSvc.Networks.List(projectID).Context(ctx).Do()
	if err != nil {
		return true
	}
	for _, nw := range nwList.Items {
		if matchesAnyPrefix(nw.Name, prefixes) {
			return true
		}
	}

	rSvc, rerr := run.NewService(ctx, opts...)
	if rerr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] probe: Cloud Run client init failed on %s/%s — assuming legacy resources may exist: %v", projectID, region, rerr))
		return true
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)
	svcList, lerr := rSvc.Projects.Locations.Services.List(parent).Context(ctx).Do()
	if lerr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] probe: Cloud Run list failed on %s/%s (likely IAM propagation lag) — assuming legacy resources may exist: %v", projectID, region, lerr))
		return true
	}
	for _, s := range svcList.Services {
		if matchesAnyPrefix(s.Name, prefixes) {
			return true
		}
	}

	sSvc, serr := storagev1.NewService(ctx, opts...)
	if serr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] probe: storage client init failed on %s — assuming legacy resources may exist: %v", projectID, serr))
		return true
	}
	bktList, blerr := sSvc.Buckets.List(projectID).Context(ctx).Do()
	if blerr != nil {
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] probe: bucket list failed on %s — assuming legacy resources may exist: %v", projectID, blerr))
		return true
	}
	for _, b := range bktList.Items {
		if matchesAnyPrefix(b.Name, prefixes) {
			return true
		}
	}

	return false
}

func summarisePermissions(perms []string, limit int) string {
	if len(perms) == 0 {
		return "none"
	}
	if len(perms) <= limit {
		return strings.Join(perms, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(perms[:limit], ", "), len(perms)-limit)
}

func waitForCleanupPermsReady(ctx context.Context, projectID string, opts ...option.ClientOption) error {
	required := camconfig.FEATURE_PERMISSIONS[camconfig.FEATURE_CLOUD_SENTRY]
	if len(required) == 0 {
		return nil
	}
	crmSvc, err := crm.NewService(ctx, opts...)
	if err != nil {
		return fmt.Errorf("crm client: %w", err)
	}

	deadline := time.Now().Add(cleanupPermsWaitMaxDuration)
	backoff := cleanupPermsPollStart
	attempt := 0
	var lastMissing []string

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
				tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] central IAM ready on %s — all %d cleanup perms granted (attempt %d)", projectID, len(required), attempt))
				return nil
			}
			lastMissing = missing
			tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] IAM not ready on %s: %d/%d granted, %d missing (e.g. %s) — attempt %d", projectID, len(required)-len(missing), len(required), len(missing), missing[0], attempt))
		} else {
			tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] testIamPermissions on %s failed (attempt %d): %v — will retry", projectID, attempt, callErr))
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			if callErr != nil {
				return fmt.Errorf("IAM propagation timeout on project %s after %s — testIamPermissions kept failing: %w", projectID, cleanupPermsWaitMaxDuration, callErr)
			}
			granted := len(required) - len(lastMissing)
			hint := "check that the CAM service account's role bindings on this project were created"
			if granted == 0 {
				hint = fmt.Sprintf("none of the required permissions are granted, so the CAM service account has no effective role binding on this project at all — inspect `gcloud projects get-iam-policy %s` rather than the permission set", projectID)
			}
			return fmt.Errorf("IAM propagation timeout on project %s after %s — CAM SA has %d/%d cleanup permissions (missing: %s); %s",
				projectID, cleanupPermsWaitMaxDuration, granted, len(required), summarisePermissions(lastMissing, 5), hint)
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

func warmupServiceCaches(ctx context.Context, projectID, region string, opts ...option.ClientOption) error {
	probes := buildWarmupProbes(ctx, projectID, region, opts...)

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
			err := p.fn()
			if err == nil || isGCPNotFound(err) {
				continue
			}
			if isRegionUnsupported(err) {
				tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] warmup: %s not available in %s — skipping", p.name, region))
				continue
			}
			if is403(err) {
				notReady = append(notReady, p.name)
			} else {
				return fmt.Errorf("warmup probe %s unexpected error: %w", p.name, err)
			}
		}
		if len(notReady) == 0 {
			tflog.Info(ctx, fmt.Sprintf("[AVTD Region Cleanup] per-service IAM caches warm on %s (attempt %d)", projectID, attempt))
			return nil
		}
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] per-service IAM cache lag on %s: %v still 403 (attempt %d)", projectID, notReady, attempt))

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("per-service IAM cache warmup timeout on project %s after %s — delete perms still 403: %v", projectID, cleanupServiceCacheMaxDuration, notReady)
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

type warmupProbe struct {
	name string
	fn   func() error
}

func buildWarmupProbes(ctx context.Context, projectID, region string, opts ...option.ClientOption) []warmupProbe {
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)
	projParent := fmt.Sprintf("projects/%s", projectID)
	suffix := fmt.Sprintf("avtd-warmup-probe-%d", time.Now().UnixNano())
	return []warmupProbe{
		{"eventarc.triggers.delete", func() error {
			svc, err := eventarc.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Triggers.Delete(fmt.Sprintf("%s/triggers/%s", parent, suffix)).Context(ctx).Do()
			return err
		}},
		{"run.services.delete", func() error {
			svc, err := run.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Services.Delete(fmt.Sprintf("%s/services/%s", parent, suffix)).Context(ctx).Do()
			return err
		}},
		{"cloudscheduler.jobs.delete", func() error {
			svc, err := scheduler.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Jobs.Delete(fmt.Sprintf("%s/jobs/%s", parent, suffix)).Context(ctx).Do()
			return err
		}},
		{"pubsub.topics.delete", func() error {
			svc, err := pubsub.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Topics.Delete(fmt.Sprintf("%s/topics/%s", projParent, suffix)).Context(ctx).Do()
			return err
		}},
		{"workflows.workflows.delete", func() error {
			svc, err := workflows.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Locations.Workflows.Delete(fmt.Sprintf("%s/workflows/%s", parent, suffix)).Context(ctx).Do()
			return err
		}},
		{"compute.firewalls.delete", func() error {
			svc, err := compute.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Firewalls.Delete(projectID, suffix).Context(ctx).Do()
			return err
		}},
		{"logging.sinks.delete", func() error {
			svc, err := logging.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Sinks.Delete(fmt.Sprintf("%s/sinks/%s", projParent, suffix)).Context(ctx).Do()
			return err
		}},
		{"secretmanager.secrets.delete", func() error {
			svc, err := secretmanager.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = svc.Projects.Secrets.Delete(fmt.Sprintf("%s/secrets/%s", projParent, suffix)).Context(ctx).Do()
			return err
		}},
		{"storage.buckets.delete", func() error {
			svc, err := storagev1.NewService(ctx, opts...)
			if err != nil {
				return err
			}
			return svc.Buckets.Delete(suffix).Context(ctx).Do()
		}},
	}
}

func probeOrphanResourceBuckets(ctx context.Context, projectID, region string, prefixes []string, opts ...option.ClientOption) ([]string, error) {
	storageSvc, err := storagev1.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}
	bktList, err := storageSvc.Buckets.List(projectID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}
	var out []string
	for _, b := range bktList.Items {
		if isResourceBucket(b.Name, region, prefixes) {
			out = append(out, b.Name)
		}
	}
	return out, nil
}

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
			resp, listErr := req.Context(ctx).Do()
			if listErr != nil {
				return len(cancelledIDs), fmt.Errorf("list builds (%s): %w", statusFilter, listErr)
			}
			for _, b := range resp.Builds {
				if _, cancelErr := svc.Projects.Builds.Cancel(projectID, b.Id, &cloudbuild.CancelBuildRequest{}).Context(ctx).Do(); cancelErr != nil {
					tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] cancel build %s: %v", b.Id, cancelErr))
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
	return len(cancelledIDs), nil
}

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
			return false, fmt.Errorf("delete bucket %s — 409 not-empty after %d empty+delete cycles. A writer outside the standard set is still active; stop it and re-run apply", bucketName, attempt)
		}
		tflog.Warn(ctx, fmt.Sprintf("[AVTD Region Cleanup] bucket %s 409 not-empty (attempt %d/%d) — re-emptying", bucketName, attempt, maxAttempts))
	}
	return false, fmt.Errorf("delete bucket %s: unreachable", bucketName)
}

func emptyGCSBucket(ctx context.Context, svc *storagev1.Service, bucketName string) error {
	return svc.Objects.List(bucketName).Pages(ctx, func(page *storagev1.Objects) error {
		for _, obj := range page.Items {
			if err := svc.Objects.Delete(bucketName, obj.Name).Context(ctx).Do(); err != nil && !isGCSNotFound(err) {
				return fmt.Errorf("delete object %s/%s: %w", bucketName, obj.Name, err)
			}
		}
		return nil
	})
}

func isGCSBucketNotEmpty(err error) bool {
	if err == nil {
		return false
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 409
	}
	return strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "not empty")
}

func isGCSNotFound(err error) bool {
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

func isRegionUnsupported(err error) bool {
	if err == nil {
		return false
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		if gerr.Code == 400 && strings.Contains(strings.ToLower(gerr.Message), "not a valid location") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "not a valid location")
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
