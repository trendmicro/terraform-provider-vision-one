package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
)

// ApplyProfile applies a profile to accounts.
func (c *CrmClient) ApplyProfile(profileID string, request *cloud_risk_management_dto.ApplyProfileRequest) (*cloud_risk_management_dto.ApplyProfileResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal apply profile request: %w", err)
	}

	url := fmt.Sprintf("%s/beta/cloudPosture/profiles/%s/apply", c.HostURL, profileID)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	body, err := c.DoRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to apply profile: %w", err)
	}

	response := &cloud_risk_management_dto.ApplyProfileResponse{}
	err = json.Unmarshal(body, response)
	if err == nil {
		return response, nil
	}

	var results []cloud_risk_management_dto.ApplyProfileResult
	err = json.Unmarshal(body, &results)
	if err == nil {
		response.Results = results
		return response, nil
	}

	return nil, fmt.Errorf("failed to unmarshal apply profile response: %w", err)
}