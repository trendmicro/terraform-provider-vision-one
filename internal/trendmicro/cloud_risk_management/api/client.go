package api

import (
	"net/http"
	"time"

	"terraform-provider-vision-one/internal/trendmicro"
)

type CrmClient struct {
	*trendmicro.Client
}

// NewCrmClient creates a new CRM client
func NewCrmClient(host, token, version string) *CrmClient {
	return &CrmClient{
		Client: &trendmicro.Client{
			HTTPClient:      &http.Client{Timeout: 60 * time.Second},
			HostURL:         host,
			BearerToken:     token,
			TMUserAgent:     "TMCRMTerraform",
			ProviderVersion: version,
		},
	}
}
