package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	cam "terraform-provider-vision-one/internal/trendmicro/cloud_account_management"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type CAMCloudAccountsResponse struct {
	TotalCount    int               `json:"totalCount"`
	Count         int               `json:"count"`
	CloudAccounts []CAMCloudAccount `json:"items"`
	NextLink      string            `json:"nextLink,omitempty"`
	PreviousLink  string            `json:"previousLink,omitempty"`
}

type CAMCloudAccount struct {
	CamDeployedRegion         string                         `json:"camDeployedRegion,omitempty"`
	CloudAssetCount           int                            `json:"cloudAssetCount,omitempty"`
	ConnectedSecurityServices []cam.ConnectedSecurityService `json:"connectedSecurityServices,omitempty"`
	CreatedTime               string                         `json:"createdDateTime,omitempty"`
	Description               string                         `json:"description,omitempty"`
	Features                  interface{}                    `json:"features,omitempty"`
	IsCAMCloudASRMEnabled     *bool                          `json:"isCAMCloudASRMEnabled,omitempty"`
	IsCloudASRMEditable       *bool                          `json:"isCloudASRMEditable,omitempty"`
	IsCloudASRMEnabled        *bool                          `json:"isCloudASRMEnabled,omitempty"`
	LastSyncedDateTime        string                         `json:"lastSyncedDateTime,omitempty"`
	Name                      string                         `json:"name,omitempty"`
	OidcProviderID            string                         `json:"oidcProviderId,omitempty"` // GCP Workload Identity Provider ID
	ProjectID                 string                         `json:"projectId,omitempty"`
	ProjectName               string                         `json:"projectName,omitempty"`
	ProjectNumber             string                         `json:"id,omitempty"`
	ServiceAccountEmail       string                         `json:"serviceAccountEmail,omitempty"` // GCP Service Account Email
	ServiceAccountID          string                         `json:"serviceAccountId,omitempty"`    // GCP Service Account Unique ID
	State                     string                         `json:"state,omitempty"`
	Sources                   []string                       `json:"sources,omitempty"`
	UpdatedDateTime           string                         `json:"updatedDateTime,omitempty"`
	WorkloadIdentityPoolID    string                         `json:"workloadIdentityPoolId,omitempty"` // GCP Workload Identity Pool ID
	Organization              *OrganizationDetailsResponse   `json:"organization,omitempty"`
}

type OrganizationDetailsResponse struct {
	ID               string   `json:"id,omitempty"`
	DisplayName      string   `json:"displayName,omitempty"`
	ExcludedProjects []string `json:"excludedProjects,omitempty"`
}

type FeatureDetail struct {
	ID              types.String   `json:"id"`
	Regions         []types.String `json:"regions"`
	TemplateVersion types.String   `json:"template_version"`
}

func (c *CamClient) ListGCPProjects(projectIds []string, top int64, state string) (*CAMCloudAccountsResponse, error) {
	if len(projectIds) > 0 {
		var allCloudAccounts []CAMCloudAccount
		for _, projectId := range projectIds {
			cloudAccount, err := c.DescribeGCPProject(projectId)
			if err != nil {
				return nil, err
			}
			if cloudAccount != nil {
				allCloudAccounts = append(allCloudAccounts, *cloudAccount)
			}
		}
		// filter by state if provided
		if state != "" {
			var filteredCloudAccounts []CAMCloudAccount
			for i := range allCloudAccounts {
				if allCloudAccounts[i].State == state {
					filteredCloudAccounts = append(filteredCloudAccounts, allCloudAccounts[i])
				}
			}
			allCloudAccounts = filteredCloudAccounts
		}
		return &CAMCloudAccountsResponse{CloudAccounts: allCloudAccounts}, nil
	}

	url := fmt.Sprintf("%s/beta/cam/gcpProjects?%s", c.Client.HostURL, "top="+fmt.Sprintf("%d", top))

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
	err = json.Unmarshal(body, &cloudAccountsResponse)
	if err != nil {
		return nil, err
	}
	// filter by state if provided
	if state != "" {
		var filteredCloudAccounts []CAMCloudAccount
		for i := range cloudAccountsResponse.CloudAccounts {
			if cloudAccountsResponse.CloudAccounts[i].State == state {
				filteredCloudAccounts = append(filteredCloudAccounts, cloudAccountsResponse.CloudAccounts[i])
			}
		}
		cloudAccountsResponse.CloudAccounts = filteredCloudAccounts
	}

	return &cloudAccountsResponse, nil
}

func (c *CamClient) DescribeGCPProject(projectId string) (*CAMCloudAccount, error) {
	if projectId == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}
	url := fmt.Sprintf("%s/beta/cam/gcpProjects/%s", c.Client.HostURL, projectId)

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// when the error is caused by 404 Not Found, return nil without error
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	var cloudAccount CAMCloudAccount
	err = json.Unmarshal(body, &cloudAccount)
	if err != nil {
		return nil, err
	}

	return &cloudAccount, nil
}
