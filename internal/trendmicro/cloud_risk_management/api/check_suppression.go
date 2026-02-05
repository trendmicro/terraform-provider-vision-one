package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
)

// Updates the suppression status of a check
// Returns 204 No Content on success
func (c *CrmClient) UpdateCheck(checkID string, data *cloud_risk_management_dto.UpdateCheckRequest) error {
	jsonData, err := json.Marshal(data)

	if err != nil {
		return fmt.Errorf("failed to marshal check suppression request: %w", err)
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/beta/cloudPosture/checks/%s", c.HostURL, url.PathEscape(checkID)), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	_, err = c.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}

// Retrieves a check by its ID to verify check suppression status
func (c *CrmClient) GetCheck(checkID string) (*cloud_risk_management_dto.GetCheckResponse, error) {
	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/checks/%s", c.HostURL, url.PathEscape(checkID))

	req, err := http.NewRequest("GET", apiUrl, http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.DoRequest(req)
	if err != nil {
		return nil, err
	}

	resp := &cloud_risk_management_dto.GetCheckResponse{}
	err = json.Unmarshal(body, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
