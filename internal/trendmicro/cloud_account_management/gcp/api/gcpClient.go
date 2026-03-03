package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1"
	cloudresourcemanagerv2 "google.golang.org/api/cloudresourcemanager/v2"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

type GCPClients struct {
	ProjectID   string
	CRMClient   *cloudresourcemanager.Service
	CRMClientV2 *cloudresourcemanagerv2.Service
	IAMClient   *iam.Service
	Credential  *google.Credentials
}

func GetGCPClients(ctx context.Context, projectID string) (*GCPClients, diag.Diagnostics) {
	var diags diag.Diagnostics

	projID, err := resolveProjectID(projectID)
	if err != nil {
		diags.AddError("Missing Project ID", err.Error())
		return nil, diags
	}

	cred, err := GetGCPCredential(ctx)
	if err != nil {
		diags.AddError("GCP Credential Error", fmt.Sprintf("Failed to get credential: %s", err))
		return nil, diags
	}

	crmClient, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(cred))
	if err != nil {
		diags.AddError("GCP Client Error", fmt.Sprintf("Failed to create Cloud Resource Manager client: %s", err))
		return nil, diags
	}

	crmClientV2, err := cloudresourcemanagerv2.NewService(ctx, option.WithCredentials(cred))
	if err != nil {
		diags.AddError("GCP Client Error", fmt.Sprintf("Failed to create Cloud Resource Manager v2 client: %s", err))
		return nil, diags
	}

	iamClient, err := iam.NewService(ctx, option.WithCredentials(cred))
	if err != nil {
		diags.AddError("IAM Client Error", fmt.Sprintf("Failed to create IAM client: %s", err))
		return nil, diags
	}

	return &GCPClients{
		ProjectID:   projID,
		CRMClient:   crmClient,
		CRMClientV2: crmClientV2,
		IAMClient:   iamClient,
		Credential:  cred,
	}, diags
}

func GetGCPCredential(ctx context.Context) (*google.Credentials, error) {
	cred, err := google.FindDefaultCredentials(ctx,
		cloudresourcemanager.CloudPlatformScope,
		iam.CloudPlatformScope,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find default GCP credentials: %w", err)
	}
	return cred, nil
}

// resolveProjectID resolves the GCP project ID from multiple sources in order of priority:
// 1. Provided projectID parameter
// 2. GOOGLE_PROJECT environment variable
// 3. GCP_PROJECT environment variable
// 4. GOOGLE_CLOUD_PROJECT environment variable
// 5. gcloud CLI configuration
func resolveProjectID(projectID string) (string, error) {
	if projectID != "" {
		return projectID, nil
	}

	// Check environment variables in order
	envVars := []string{"GOOGLE_PROJECT", "GCP_PROJECT", "GOOGLE_CLOUD_PROJECT"}
	for _, envVar := range envVars {
		if id := os.Getenv(envVar); id != "" {
			return id, nil
		}
	}

	// Try gcloud CLI as last resort
	if cliProjectID, err := GetGCPCLIProject(); err == nil {
		return cliProjectID, nil
	}

	return "", fmt.Errorf("project ID not found: set GOOGLE_PROJECT, GCP_PROJECT, or GOOGLE_CLOUD_PROJECT environment variable, provide project_id attribute, or configure gcloud CLI")
}

type gcpConfig struct {
	Core struct {
		Project string `json:"project"`
		Account string `json:"account"`
	} `json:"core"`
}

func GetGCPCLIProject() (string, error) {
	cmd := exec.Command("gcloud", "config", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute 'gcloud config list': %w", err)
	}

	var config gcpConfig
	if err := json.Unmarshal(output, &config); err != nil {
		return "", fmt.Errorf("failed to parse gcloud CLI output: %w", err)
	}

	projectID := config.Core.Project
	if projectID == "" {
		return "", fmt.Errorf("no project ID found in gcloud CLI configuration")
	}

	return projectID, nil
}
