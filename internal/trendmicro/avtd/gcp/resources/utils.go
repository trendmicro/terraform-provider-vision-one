package resources

import (
	"context"
	"encoding/base64"
	"fmt"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const gcpCloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

func newClientOptionFromEncodedServiceAccountKey(ctx context.Context, encodedKey string) (option.ClientOption, error) {
	keyJSON, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid base64-encoded service account key: %w", err)
	}
	creds, err := google.CredentialsFromJSON(ctx, keyJSON, gcpCloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("credentials from service account key: %w", err)
	}
	return option.WithCredentials(creds), nil
}
