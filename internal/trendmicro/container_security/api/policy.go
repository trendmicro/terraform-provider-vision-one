package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"terraform-provider-vision-one/pkg/dto"
)

func (c *CsClient) CreatePolicy(data *dto.CreatePolicyRequest) (*dto.PolicyResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v3.0/containerSecurity/policies", c.Client.HostURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")

	parsedURL, err := url.Parse(location)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return nil, err
	}

	// Use the path.Base function to get the last element of the path
	policyID := path.Base(parsedURL.Path)

	// workaround for setting malwareScanEnable = false but give schedule in the request
	if data.MalwareScan != nil {
		if data.MalwareScan.Schedule != nil {
			if !*data.MalwareScan.Schedule.Enabled {
				_, err = c.UpdatePolicy(policyID, &dto.UpdatePolicyRequest{
					MalwareScan: &dto.MalwareScan{
						Schedule: &dto.MalwareSchedule{
							Enabled: data.MalwareScan.Schedule.Enabled,
						},
					},
					XdrEnabled: data.XdrEnabled,
				})
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// workaround for setting secretScanEnable = false but give schedule in the request
	if data.SecretScan != nil {
		if data.SecretScan.Schedule != nil {
			if !*data.SecretScan.Schedule.Enabled {
				_, err = c.UpdatePolicy(policyID, &dto.UpdatePolicyRequest{
					SecretScan: &dto.SecretScan{
						Schedule: &dto.SecretSchedule{
							Enabled: data.SecretScan.Schedule.Enabled,
						},
					},
				})
				if err != nil {
					return nil, err
				}
			}
		}
	}

	createdPolicy, err := c.GetPolicy(policyID)
	if err != nil {
		return nil, err
	}

	return createdPolicy, nil
}

func (c *CsClient) GetPolicy(id string) (*dto.PolicyResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3.0/containerSecurity/policies/%s", c.Client.HostURL, id), http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	resp := dto.PolicyResponse{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *CsClient) GetPolicyList() (*dto.ListPolicyResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3.0/containerSecurity/policies", c.Client.HostURL), http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	resp := dto.ListPolicyResponse{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *CsClient) UpdatePolicy(id string, data *dto.UpdatePolicyRequest) (*dto.PolicyResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v3.0/containerSecurity/policies/%s", c.Client.HostURL, id), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.DoRequestWithFullResponse(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	updatedPolicy, err := c.GetPolicy(id)
	if err != nil {
		return nil, err
	}

	return updatedPolicy, nil
}

func (c *CsClient) DeletePolicy(id string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v3.0/containerSecurity/policies/%s", c.Client.HostURL, id), http.NoBody)
	if err != nil {
		return err
	}

	_, err = c.Client.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}
