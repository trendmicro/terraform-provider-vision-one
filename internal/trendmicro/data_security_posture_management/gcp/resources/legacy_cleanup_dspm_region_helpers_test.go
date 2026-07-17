package resources

import "testing"

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

func TestIsStateBucketNotFound(t *testing.T) {
	// Fail closed: only a genuine 404 counts as "no state yet".
	cases := []struct {
		msg  string
		want bool
	}{
		{"googleapi: Error 404: Not Found, notFound", true},
		{"googleapi: Error 403: Forbidden", false},
		{"some other error", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isStateBucketNotFound(fakeErr(tc.msg)); got != tc.want {
			t.Errorf("isStateBucketNotFound(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
	if isStateBucketNotFound(nil) {
		t.Error("isStateBucketNotFound(nil) should be false")
	}
}

func TestParseTfStateTrackedResources(t *testing.T) {
	stateJSON := []byte(`{
		"version": 4,
		"resources": [
			{
				"module": "module.data-security-posture-management[\"data-security-stg\"].module.data-security-posture-management[\"asia-south1\"].module.network",
				"type": "google_compute_network",
				"name": "dspm_vpc",
				"instances": [
					{"attributes": {"name": "dspm-s-ass1-vpc", "self_link": "projects/data-security-stg/global/networks/dspm-s-ass1-vpc"}}
				]
			},
			{
				"type": "google_compute_firewall",
				"name": "egress_web",
				"instances": [
					{"attributes": {"name": "dspm-s-ass1-egress-web"}}
				]
			},
			{
				"type": "google_monitoring_alert_policy",
				"name": "vm_termination_trigger",
				"instances": [
					{"attributes": {"display_name": "dspm-s-ass1-vm-termination", "name": "projects/123/alertPolicies/456"}}
				]
			}
		]
	}`)

	tracked, err := parseTfStateTrackedResources(stateJSON)
	if err != nil {
		t.Fatalf("parseTfStateTrackedResources returned error: %v", err)
	}

	if !tracked.has("google_compute_network", "dspm-s-ass1-vpc") {
		t.Error("expected google_compute_network/dspm-s-ass1-vpc to be tracked")
	}
	if !tracked.has("google_compute_firewall", "dspm-s-ass1-egress-web") {
		t.Error("expected google_compute_firewall/dspm-s-ass1-egress-web to be tracked")
	}
	// Alert policies match on display_name, not the state's `name` (an API resource path).
	if !tracked.has("google_monitoring_alert_policy", "dspm-s-ass1-vm-termination") {
		t.Error("expected google_monitoring_alert_policy tracked by display_name")
	}
	if tracked.has("google_compute_network", "dspm-s-ass1-subnet") {
		t.Error("did not expect an untracked resource to report as tracked")
	}
	if tracked.has("google_compute_subnetwork", "dspm-s-ass1-vpc") {
		t.Error("match must be scoped by resource type, not just name")
	}
}

func TestParseTfStateTrackedResources_Empty(t *testing.T) {
	tracked, err := parseTfStateTrackedResources([]byte(`{"version":4,"resources":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracked.has("google_compute_network", "anything") {
		t.Error("empty state should track nothing")
	}
}

func TestParseTfStateTrackedResources_InvalidJSON(t *testing.T) {
	if _, err := parseTfStateTrackedResources([]byte("not json")); err == nil {
		t.Error("expected an error for invalid JSON")
	}
}

func TestTrackedResourceSetHas_NilSafe(t *testing.T) {
	var tracked trackedResourceSet
	if tracked.has("google_compute_network", "dspm-s-ass1-vpc") {
		t.Error("nil trackedResourceSet must report nothing as tracked (state_bucket unset => today's unconditional-delete behavior)")
	}
	if tracked.has("google_compute_network", "") {
		t.Error("empty name must never match")
	}
}

func TestRegionAbbreviation(t *testing.T) {
	// Spot-check the explicit table and the bash fallback (`tr -d '-' | cut -c1-8`).
	// If a future GCP region is added to the table, append a case here so the
	// bash and Go implementations can't drift.
	cases := []struct {
		region string
		want   string
	}{
		// Explicit table entries (spot checks across continents)
		{"us-east1", "use1"},
		{"us-central1", "usc1"},
		{"europe-west9", "euw9"},
		{"asia-northeast1", "asne1"},
		{"australia-southeast2", "ause2"},
		{"northamerica-northeast2", "nane2"},
		{"me-west1", "mew1"},
		// Bash fallback: tr -d '-' | cut -c1-8
		{"unknown-region-99", "unknownr"}, // 17 chars stripped → "unknownregion99" → first 8 → "unknownr"
		{"short", "short"},                // no hyphens, < 8 chars
		{"abcdefghij", "abcdefgh"},        // > 8 chars truncated
		{"x-y-z", "xyz"},                  // hyphens stripped, < 8 chars
	}
	for _, tc := range cases {
		if got := regionAbbreviation(tc.region); got != tc.want {
			t.Errorf("regionAbbreviation(%q) = %q, want %q", tc.region, got, tc.want)
		}
	}
}
