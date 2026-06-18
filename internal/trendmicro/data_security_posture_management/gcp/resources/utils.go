package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const gcpCloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// saEmailFromEncodedKey decodes the base64 SA key and returns its client_email.
// Used by the cleanup_region janitor to know which principal's stale role
// bindings to purge.
func saEmailFromEncodedKey(encodedKey string) (string, error) {
	keyJSON, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return "", fmt.Errorf("invalid base64-encoded service account key: %w", err)
	}
	var payload struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(keyJSON, &payload); err != nil {
		return "", fmt.Errorf("invalid service account key JSON: %w", err)
	}
	if payload.ClientEmail == "" {
		return "", fmt.Errorf("service account key JSON missing client_email")
	}
	return payload.ClientEmail, nil
}

// newClientOptionFromEncodedServiceAccountKey base64-decodes the provided
// service-account key and returns a GCP client option carrying its credentials.
//
// Duplicated from cloud_account_management/gcp/resources/utils.go so this
// package does not depend on CAM. The two copies are intentionally identical;
// no shared "legacy utils" package is needed at this scale.
func newClientOptionFromEncodedServiceAccountKey(ctx context.Context, encodedKey string) (option.ClientOption, error) {
	keyJSON, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid base64-encoded service account key: %w", err)
	}

	creds, err := google.CredentialsFromJSON(ctx, keyJSON, gcpCloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("invalid service account key JSON: %w", err)
	}

	return option.WithCredentials(creds), nil
}
