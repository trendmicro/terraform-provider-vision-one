package resources

import (
	"context"
	"encoding/base64"
	"fmt"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const gcpCloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

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
