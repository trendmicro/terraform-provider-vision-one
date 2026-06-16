package resources

import "testing"

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
		{"unknown-region-99", "unknownr"},  // 17 chars stripped → "unknownregion99" → first 8 → "unknownr"
		{"short", "short"},                  // no hyphens, < 8 chars
		{"abcdefghij", "abcdefgh"},          // > 8 chars truncated
		{"x-y-z", "xyz"},                    // hyphens stripped, < 8 chars
	}
	for _, tc := range cases {
		if got := regionAbbreviation(tc.region); got != tc.want {
			t.Errorf("regionAbbreviation(%q) = %q, want %q", tc.region, got, tc.want)
		}
	}
}
