package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const callerIdentity = "tf-provider-aws-connector"

type SecurityService struct {
	Name        string   `json:"name,omitempty"`
	InstanceIDs []string `json:"instanceIds,omitempty"`
	Regions     []string `json:"regions,omitempty"`
}

// CreateCloudAccountRequest — CAM POST /public/cam/api/v1/awsAccounts (AWSPostBodyV1)
type CreateCloudAccountRequest struct {
	RoleArn                         string            `json:"roleArn"`
	Name                            string            `json:"name,omitempty"`
	Description                     string            `json:"description,omitempty"`
	Features                        []interface{}     `json:"features,omitempty"`
	ConnectedSecurityServices       []SecurityService `json:"connectedSecurityServices,omitempty"`
	ServerWorkloadProtectionRegions []string          `json:"serverWorkloadProtectionRegions,omitempty"`
	FeaturesConfigFilePath          string            `json:"featuresConfigFilePath,omitempty"`

	OrganizationExcludedAccounts    []string        `json:"organizationExcludedAccounts,omitempty"`
	TargetOrganizationalUnitIDs     []string        `json:"targetOrganizationalUnitIds,omitempty"`
	IsAwsOrgMgmtAccount             *bool           `json:"isAwsOrgMgmtAccount,omitempty"`
	CustomTags                      []cam.CustomTag `json:"customTags,omitempty"`
	IsCremEnabled                   *bool           `json:"isCAMCloudASRMEnabled,omitempty"`
	IsTFProviderDeployed            *bool           `json:"isTFProviderDeployed,omitempty"`
}

// CloudAccountFeatureDetailAWSResponse — CAM CloudAccountFeatureDetailAWSResponse
type CloudAccountFeatureDetailAWSResponse struct {
	Id                    string   `json:"id,omitempty"`
	Regions               []string `json:"regions,omitempty"`
	MissingAwsPermissions []string `json:"missingAwsPermissions,omitempty"`
	TemplateVersion       string   `json:"templateVersion,omitempty"`
}

// ModifyCloudAccountRequest —  CAM PATCH /public/cam/api/v1/awsAccounts/{id} (PatchBodyPublicV1)
type ModifyCloudAccountRequest struct {
	RoleArn                         *string                `json:"roleArn,omitempty"`
	Name                            *string                `json:"name,omitempty"`
	Description                     *string                `json:"description,omitempty"`
	Features                        []interface{}          `json:"features,omitempty"`
	FeaturesConfigFilePath          string                 `json:"featuresConfigFilePath,omitempty"`
	FeaturesConfigBody              map[string]interface{} `json:"configurations,omitempty"`
	OrganizationExcludedAccounts    []string               `json:"organizationExcludedAccounts,omitempty"`
	TargetOrganizationalUnitIDs     []string               `json:"targetOrganizationalUnitIds,omitempty"`
	IsAwsOrgMgmtAccount             *bool                  `json:"isAwsOrgMgmtAccount,omitempty"`
	ConnectedSecurityServices       []SecurityService      `json:"connectedSecurityServices,omitempty"`
	CustomTags                      []cam.CustomTag        `json:"customTags,omitempty"`
	IsCremEnabled                   *bool                  `json:"isCAMCloudASRMEnabled,omitempty"`
	IsTFProviderDeployed            *bool                  `json:"isTFProviderDeployed,omitempty"`
	ServerWorkloadProtectionRegions *[]string              `json:"serverWorkloadProtectionRegions,omitempty"`
}

// CloudAccountResponse — CAM GET /public/cam/api/v1/awsAccounts/{id} (CloudAccountDetailAWSResponse V1)
type CloudAccountResponse struct {
	CloudAccountID                  string                                 `json:"id"`
	RoleArn                         string                                 `json:"roleArn"`
	Name                            string                                 `json:"name,omitempty"`
	Description                     string                                 `json:"description,omitempty"`
	CreatedTime                     string                                 `json:"createdDateTime"`
	UpdatedTime                     string                                 `json:"updatedDateTime"`
	State                           string                                 `json:"state"`
	OrganizationID                  string                                 `json:"organizationID,omitempty"`
	Features                        []CloudAccountFeatureDetailAWSResponse `json:"features,omitempty"`
	LastSyncTime                    string                                 `json:"lastSyncedDateTime,omitempty"`
	IsCremEnabled                   *bool                                  `json:"isCAMCloudASRMEnabled,omitempty"`
	IsTerraformDeployed             bool                                   `json:"isTerraformDeployed,omitempty"`
	IsTFProviderDeployed            bool                                   `json:"isTFProviderDeployed,omitempty"`
	ServerWorkloadProtectionRegions []string                               `json:"serverWorkloadProtectionRegions,omitempty"`
	Sources                         []string                               `json:"sources,omitempty"`
	CustomTags                      []cam.CustomTag                        `json:"customTags,omitempty"`
	ConnectedSecurityServices       []SecurityService                      `json:"connectedSecurityServices,omitempty"`
}

// CreateCloudAccount submits a POST request to register a new AWS account in CAM.
// organizationID is injected as a tmv1-organizationID header when non-empty.
func (c *CamClient) CreateCloudAccount(ctx context.Context, organizationID string, data *CreateCloudAccountRequest) (string, error) {
	cam.JitterSleep(cam.AWSJitterConfig)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	tflog.Debug(ctx, "[CAM Connector] creating AWS account", map[string]interface{}{
		"role_arn_suffix": data.RoleArn[max(0, len(data.RoleArn)-12):],
	})

	var resp *http.Response
	var postRequestErr error

	maxRetries := 3
	baseDelay := 5 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, reqErr := http.NewRequest("POST", fmt.Sprintf("%s/beta/cam/awsAccounts", c.Client.HostURL), bytes.NewBuffer(jsonData))
		if reqErr != nil {
			return "", reqErr
		}
		req.Header.Set("tmv1-callerIdentity", callerIdentity)
		if organizationID != "" {
			req.Header.Set("tmv1-organizationID", organizationID)
		}

		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1))
			time.Sleep(delay)
		}

		resp, postRequestErr = c.Client.DoRequestWithFullResponse(req)
		if postRequestErr == nil {
			break
		}
		tflog.Debug(ctx, "[CAM Connector] AWS account creation attempt failed", map[string]interface{}{
			"attempt":     attempt + 1,
			"max_retries": maxRetries + 1,
			"error":       postRequestErr.Error(),
		})
	}

	if postRequestErr != nil {
		return "", fmt.Errorf("AWS account creation failed after retries: %v", postRequestErr)
	}

	defer resp.Body.Close()

	cloudAccountID, err := c.Client.ExtractIDFromLocationHeader(resp.Header)
	if err != nil {
		return "", fmt.Errorf("CreateAwsAccount: %w", err)
	}
	return cloudAccountID, nil
}

func (c *CamClient) ReadCloudAccount(cloudAccountID string, excludeCloudAssets bool) (*CloudAccountResponse, error) {
	cam.JitterSleep(cam.AWSJitterConfig)
	url := fmt.Sprintf("%s/beta/cam/awsAccounts/%s", c.Client.HostURL, cloudAccountID)
	if excludeCloudAssets {
		url += "?excludeCloudAssets=true"
	}

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result CloudAccountResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *CamClient) UpdateCloudAccounts(cloudAccountID, organizationID string, data *ModifyCloudAccountRequest) error {
	cam.JitterSleep(cam.AWSJitterConfig)
	url := fmt.Sprintf("%s/beta/cam/awsAccounts/%s", c.Client.HostURL, cloudAccountID)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("tmv1-callerIdentity", callerIdentity)
	if organizationID != "" {
		req.Header.Set("tmv1-organizationID", organizationID)
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (c *CamClient) DeleteCloudAccounts(cloudAccountID string) error {
	cam.JitterSleep(cam.AWSJitterConfig)
	url := fmt.Sprintf("%s/beta/cam/awsAccounts/%s", c.Client.HostURL, cloudAccountID)

	req, err := http.NewRequest("DELETE", url, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil
		}
		return err
	}

	defer resp.Body.Close()

	return nil
}
