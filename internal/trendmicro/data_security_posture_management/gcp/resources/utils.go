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

// saEmailFromEncodedKey decodes the base64 SA key and returns its client_email for the cleanup_region janitor.
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

// newClientOptionFromEncodedServiceAccountKey decodes a base64 SA key into a GCP client option. Intentional copy from CAM utils — no shared package at this scale.
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
