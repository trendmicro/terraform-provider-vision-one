package api

import (
	"net/http"
	"strings"
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

// IsNotFoundError checks if an error is a 404 NotFound error from the API
// The CRM API returns 404 errors with a JSON structure containing "NotFound" code
// This helper detects such errors and returns the dto.ErrorNotFound sentinel
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// Check if the error message contains the NotFound code from the API response
	return strings.Contains(errMsg, "\"code\": \"NotFound\"") ||
		strings.Contains(errMsg, "\"code\":\"NotFound\"")
}
