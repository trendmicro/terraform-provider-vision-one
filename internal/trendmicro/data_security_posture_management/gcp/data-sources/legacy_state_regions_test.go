package data_sources

import "testing"

func TestRegionTokenRE_MatchesAndFilters(t *testing.T) {
	// Mirrors the bash python:
	//   re.finditer(r'\["([^"]+)"\]', module) then filter on last-char-digit
	// to drop non-region tokens like ["data"] / ["service_account"].
	cases := []struct {
		module string
		want   []string
	}{
		{`module.dspm["us-east1"]`, []string{"us-east1"}},
		{`module.dspm["us-east1"].module.inner["europe-west4"]`, []string{"us-east1", "europe-west4"}},
		{`module.dspm["bucket"].module.dspm["us-west2"]`, []string{"us-west2"}}, // "bucket" filtered (not digit-end)
		{`module.dspm["data-security-int"]`, nil},                                // project_id filtered
		{`module.dspm`, nil},                                                     // no tokens
		{`module.dspm[""]`, nil},                                                 // empty token
	}
	for _, tc := range cases {
		var got []string
		for _, m := range regionTokenRE.FindAllStringSubmatch(tc.module, -1) {
			tok := m[1]
			if tok == "" {
				continue
			}
			last := tok[len(tok)-1]
			if last < '0' || last > '9' {
				continue
			}
			got = append(got, tok)
		}
		if !equalSlices(got, tc.want) {
			t.Errorf("module=%q got=%v want=%v", tc.module, got, tc.want)
		}
	}
}

func TestIsStorageNotFound(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"googleapi: Error 404: Not Found, notFound", true},
		{"object doesn't exist", true},
		{"some other error", false},
		{"", false},
	}
	for _, tc := range cases {
		err := fakeErr(tc.msg)
		if got := isStorageNotFound(err); got != tc.want {
			t.Errorf("isStorageNotFound(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
	if isStorageNotFound(nil) {
		t.Error("isStorageNotFound(nil) should be false")
	}
}

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
