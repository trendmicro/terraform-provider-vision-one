package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"
)

type CAMCloudAccountsResponse struct {
	TotalCount    int               `json:"totalCount"`
	Count         int               `json:"count"`
	CloudAccounts []CAMCloudAccount `json:"items"`
	NextLink      string            `json:"nextLink,omitempty"`
	PreviousLink  string            `json:"previousLink,omitempty"`
}

// CAMCloudAccount CAM V1 list/describe API response
// list:     AwsCloudAccountDto (list-aws/dtos/response.go)
// describe: CloudAccountDetailAWSResponse (describe-aws/dtos/response.go)
type CAMCloudAccount struct {
	ID                              string                                 `json:"id,omitempty"`
	RoleArn                         string                                 `json:"roleArn,omitempty"`
	Name                            string                                 `json:"name,omitempty"`
	Description                     string                                 `json:"description,omitempty"`
	State                           string                                 `json:"state,omitempty"`
	CreatedDateTime                 string                                 `json:"createdDateTime,omitempty"`
	UpdatedDateTime                 string                                 `json:"updatedDateTime,omitempty"`
	LastSyncedDateTime              string                                 `json:"lastSyncedDateTime,omitempty"`
	OrganizationID                  string                                 `json:"organizationID,omitempty"`
	Features                        any                                    `json:"features,omitempty"`
	ConnectedSecurityServices       []cam.ConnectedSecurityService         `json:"connectedSecurityServices,omitempty"`
	Sources                         []string                               `json:"sources,omitempty"`
	CustomTags                      []cam.CustomTag                        `json:"customTags,omitempty"`
	OrgFeatureGroupName             string                                 `json:"orgFeatureGroupName,omitempty"`
	ServerWorkloadProtectionRegions []string                               `json:"serverWorkloadProtectionRegions,omitempty"`
	IsCAMCloudASRMEnabled           *bool                                  `json:"isCAMCloudASRMEnabled,omitempty"`
	IsCloudASRMEditable             *bool                                  `json:"isCloudASRMEditable,omitempty"`
	IsCloudASRMEnabled              *bool                                  `json:"isCloudASRMEnabled,omitempty"`
	IsTerraformDeployed             bool                                   `json:"isTerraformDeployed,omitempty"`
	CloudAssetCount                 int                                    `json:"cloudAssetCount,omitempty"`
}

// CloudAccountFeatureDetailAWSResponse CAM V1 CloudAccountFeatureDetailAWSResponse / FeatureDto
type CloudAccountFeatureDetailAWSResponse struct {
	Id                    string   `json:"id,omitempty"`
	Regions               []string `json:"regions,omitempty"`
	MissingAwsPermissions []string `json:"missingAwsPermissions,omitempty"`
	TemplateVersion       string   `json:"templateVersion,omitempty"`
}

func filterByState(accounts []CAMCloudAccount, state string) []CAMCloudAccount {
	if state == "" {
		return accounts
	}
	filtered := make([]CAMCloudAccount, 0, len(accounts))
	for i := range accounts {
		if accounts[i].State == state {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}

func (c *CamClient) ListCloudAccounts(cloudAccountIDs []string, top int64, state string) (*CAMCloudAccountsResponse, error) {
	if len(cloudAccountIDs) > 0 {
		var allCloudAccounts []CAMCloudAccount
		for _, cloudaccountID := range cloudAccountIDs {
			cloudAccount, err := c.DescribeCloudAccount(cloudaccountID)
			if err != nil {
				return nil, err
			}
			if cloudAccount != nil {
				allCloudAccounts = append(allCloudAccounts, *cloudAccount)
			}
		}
		return &CAMCloudAccountsResponse{CloudAccounts: filterByState(allCloudAccounts, state)}, nil
	}

	url := fmt.Sprintf("%s/beta/cam/awsAccounts?%s", c.Client.HostURL, "top="+fmt.Sprintf("%d", top))

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

	var cloudAccountsResponse CAMCloudAccountsResponse
	if err := json.Unmarshal(body, &cloudAccountsResponse); err != nil {
		return nil, err
	}
	cloudAccountsResponse.CloudAccounts = filterByState(cloudAccountsResponse.CloudAccounts, state)

	return &cloudAccountsResponse, nil
}

func (c *CamClient) DescribeCloudAccount(cloudAccountID string) (*CAMCloudAccount, error) {
	url := fmt.Sprintf("%s/beta/cam/awsAccounts/%s", c.Client.HostURL, cloudAccountID)

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		if strings.Contains(err.Error(), `"code": "NotFound"`) {
			return nil, nil
		}
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var cloudAccount CAMCloudAccount
	err = json.Unmarshal(body, &cloudAccount)
	if err != nil {
		return nil, err
	}

	return &cloudAccount, nil
}
