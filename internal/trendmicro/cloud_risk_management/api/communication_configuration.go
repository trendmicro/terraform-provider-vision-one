package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
)

// Note: API accepts array and returns 207 with array response
func (c *CrmClient) CreateCommunicationConfiguration(data *cloud_risk_management_dto.CreateCommunicationConfigurationRequest) (*cloud_risk_management_dto.CommunicationConfiguration, error) {
	requestBody := []cloud_risk_management_dto.CreateCommunicationConfigurationRequest{*data}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/communicationConfigurations", c.Client.HostURL)
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resBody, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	// Parse 207 multi-status response
	var responses []cloud_risk_management_dto.MultiStatusResponseItem
	err = json.Unmarshal(resBody, &responses)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(responses) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	resp := responses[0]
	if resp.Status != 201 {
		return nil, fmt.Errorf("failed to create communication configuration (status %d): %s", resp.Status, string(resp.Body))
	}

	// Extract ID from Location header in response body
	if len(resp.Headers) == 0 {
		return nil, fmt.Errorf("no headers in response body")
	}

	parsedURL, err := url.Parse(resp.Headers[0].Value)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Location header: %w", err)
	}

	id := path.Base(parsedURL.Path)
	if id == "" {
		return nil, fmt.Errorf("failed to extract ID from response body")
	}

	return &cloud_risk_management_dto.CommunicationConfiguration{
		ID: id,
	}, nil
}

func (c *CrmClient) GetCommunicationConfiguration(configID string) (*cloud_risk_management_dto.CommunicationConfiguration, error) {
	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/communicationConfigurations/%s", c.Client.HostURL, configID)

	req, err := http.NewRequest("GET", apiUrl, http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.Client.DoRequest(req)
	if err != nil {
		return nil, err
	}

	config := cloud_risk_management_dto.CommunicationConfiguration{}
	err = json.Unmarshal(body, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *CrmClient) UpdateCommunicationConfiguration(configID string, data *cloud_risk_management_dto.UpdateCommunicationConfigurationRequest) error {
	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/communicationConfigurations/%s", c.Client.HostURL, configID)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	_, err = c.Client.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}

func (c *CrmClient) DeleteCommunicationConfiguration(configID string) error {
	apiUrl := fmt.Sprintf("%s/beta/cloudPosture/communicationConfigurations/%s", c.Client.HostURL, configID)
	req, err := http.NewRequest("DELETE", apiUrl, http.NoBody)
	if err != nil {
		return err
	}

	_, err = c.Client.DoRequest(req)
	if err != nil {
		return err
	}

	return nil
}
