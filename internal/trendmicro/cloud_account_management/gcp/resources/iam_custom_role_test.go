package resources

import (
	"context"
	"slices"
	"testing"

	"terraform-provider-vision-one/internal/trendmicro/cloud_account_management/gcp/resources/config"
)

func TestGCPScanRoleUsesReadOnlyProfile(t *testing.T) {
	r, ok := NewGCPScanRole().(*IAMCustomRole)
	if !ok {
		t.Fatalf("NewGCPScanRole did not return *IAMCustomRole")
	}
	if !slices.Contains(r.core, "cloudasset.assets.searchAllResources") {
		t.Errorf("scan role core missing cloudasset read permission")
	}
	if slices.Contains(r.core, "iam.serviceAccountKeys.create") {
		t.Errorf("scan role core must not contain deploy self-maintenance permissions")
	}

	got, err := r.aggregatePermissions(context.Background(), r.core, []string{config.FEATURE_DATA_SECURITY_POSTURE_MANAGEMENT}, r.featureTable)
	if err != nil {
		t.Fatalf("aggregatePermissions returned error: %v", err)
	}
	if !slices.Contains(got, "cloudasset.assets.searchAllResources") {
		t.Errorf("scan role missing scan core read permission")
	}
	// The read-only scan role must never inherit deploy/cleanup write permissions.
	for _, writePerm := range []string{"storage.buckets.delete", "compute.instances.delete", "logging.sinks.delete"} {
		if slices.Contains(got, writePerm) {
			t.Errorf("scan role must not contain deploy write permission %q", writePerm)
		}
	}
}

func TestIAMCustomRoleUsesDeployProfile(t *testing.T) {
	r, ok := NewIAMCustomRole().(*IAMCustomRole)
	if !ok {
		t.Fatalf("NewIAMCustomRole did not return *IAMCustomRole")
	}
	if !slices.Contains(r.core, "iam.serviceAccountKeys.create") {
		t.Errorf("deploy role core missing self-maintenance permission")
	}

	got, err := r.aggregatePermissions(context.Background(), r.core, []string{config.FEATURE_DATA_SECURITY_POSTURE_MANAGEMENT}, r.featureTable)
	if err != nil {
		t.Fatalf("aggregatePermissions returned error: %v", err)
	}
	if !slices.Contains(got, "iam.serviceAccountKeys.create") {
		t.Errorf("deploy role missing core self-maintenance permission")
	}
	if !slices.Contains(got, "storage.buckets.delete") {
		t.Errorf("deploy role missing DSPM feature permission")
	}
}
