package api

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"terraform-provider-vision-one/internal/trendmicro"
	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDeleteProjectTreatsNotFoundAsSuccess(t *testing.T) {
	originalJitter := cam.GCPJitterConfig
	cam.GCPJitterConfig = cam.JitterConfig{}
	t.Cleanup(func() { cam.GCPJitterConfig = originalJitter })

	client := newTestCAMClient(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":{"code":"NotFound","message":"not found"}}`), nil
	})

	if err := client.DeleteProject("123"); err != nil {
		t.Fatalf("DeleteProject returned error for 404: %v", err)
	}
}

func newTestCAMClient(roundTrip roundTripFunc) *CamClient {
	return &CamClient{Client: &trendmicro.Client{
		HostURL:    "https://unit.test",
		HTTPClient: &http.Client{Transport: roundTrip},
	}}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
