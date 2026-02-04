package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
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
	ApplicationID             string                     `json:"applicationId,omitempty"`
	CamDeployedRegion         string                     `json:"camDeployedRegion,omitempty"`
	CloudAssetCount           int                        `json:"cloudAssetCount,omitempty"`
	ConnectedSecurityServices []ConnectedSecurityService `json:"connectedSecurityServices,omitempty"`
	CreatedDateTime           string                     `json:"createdDateTime,omitempty"`
	Description               string                     `json:"description,omitempty"`
	Features                  interface{}                `json:"features,omitempty"`
	ID                        string                     `json:"id,omitempty"`
	IsCAMCloudASRMEnabled     bool                       `json:"isCAMCloudASRMEnabled,omitempty"`
	IsCloudASRMEditable       bool                       `json:"isCloudASRMEditable,omitempty"`
	IsCloudASRMEnabled        bool                       `json:"isCloudASRMEnabled,omitempty"`
	IsTerraformDeployed       bool                       `json:"isTerraformDeployed,omitempty"`
	LastSyncedDateTime        string                     `json:"lastSyncedDateTime,omitempty"`
	Name                      string                     `json:"name,omitempty"`
	Region                    string                     `json:"region,omitempty"`
	State                     string                     `json:"state,omitempty"`
	Sources                   []string                   `json:"sources,omitempty"`
	TenantID                  string                     `json:"tenantId,omitempty"`
	UpdatedDateTime           string                     `json:"updatedDateTime,omitempty"`
}

type ConnectedSecurityService struct {
	Name        string   `json:"name"`
	InstanceIds []string `json:"instanceIds"`
}

type FeatureDetail struct {
	ID              types.String   `json:"id"`
	Regions         []types.String `json:"regions"`
	TemplateVersion types.String   `json:"template_version"`
}

func (c *CamClient) ListAzureSubscriptions(subscriptionIds []string, top int64, state string) (*CAMCloudAccountsResponse, error) {
	if len(subscriptionIds) > 0 {
		var allCloudAccounts []CAMCloudAccount
		for _, subscriptionId := range subscriptionIds {
			cloudAccount, err := c.DescribeAzureSubscription(subscriptionId)
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
				if allCloudAccounts[i].State != state {
					continue
				}
				filteredCloudAccounts = append(filteredCloudAccounts, allCloudAccounts[i])
			}
			allCloudAccounts = filteredCloudAccounts
		}
		return &CAMCloudAccountsResponse{CloudAccounts: allCloudAccounts}, nil
	}

	url := fmt.Sprintf("%s/beta/cam/azureSubscriptions?%s", c.Client.HostURL, "top="+fmt.Sprintf("%d", top))

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
			if cloudAccountsResponse.CloudAccounts[i].State != state {
				continue
			}
			filteredCloudAccounts = append(filteredCloudAccounts, cloudAccountsResponse.CloudAccounts[i])
		}
		cloudAccountsResponse.CloudAccounts = filteredCloudAccounts
	}

	return &cloudAccountsResponse, nil
}

func (c *CamClient) DescribeAzureSubscription(subscriptionId string) (*CAMCloudAccount, error) {
	if _, err := uuid.Parse(subscriptionId); err != nil {
		return nil, fmt.Errorf("%s is invalid subscription ID format", subscriptionId)
	}
	url := fmt.Sprintf("%s/beta/cam/azureSubscriptions/%s", c.Client.HostURL, subscriptionId)

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
