package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
)

const (
	reportConfigsPath = "/beta/cloudPosture/reportConfigurations"
)

// CreateReportConfig creates a new report configuration
func (c *CrmClient) CreateReportConfig(req *cloud_risk_management_dto.CreateReportConfigRequest) (*cloud_risk_management_dto.ReportConfig, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create report config request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s%s", c.HostURL, reportConfigsPath), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	response, err := c.DoRequestWithFullResponse(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create report config: %w", err)
	}
	defer response.Body.Close()

	// Extract the report config ID from response's Location header
	id, err := extractIDFromLocation(&response.Header)
	if err != nil {
		return nil, err
	}

	if id == "" {
		return nil, fmt.Errorf("failed to extract report config ID from response header")
	}

	// Fetch the created resource to get the actual state from API
	return c.GetReportConfig(id)
}

// Retrieves a report configuration by ID
func (c *CrmClient) GetReportConfig(reportConfigID string) (*cloud_risk_management_dto.ReportConfig, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s%s/%s", c.HostURL, reportConfigsPath, reportConfigID), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	body, err := c.DoRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get report config: %w", err)
	}

	var reportConfig cloud_risk_management_dto.ReportConfig
	if err := json.Unmarshal(body, &reportConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal report config response: %w", err)
	}

	reportConfig.ID = reportConfigID

	return &reportConfig, nil
}

// Updates an existing report configuration
func (c *CrmClient) UpdateReportConfig(reportConfigID string, req *cloud_risk_management_dto.UpdateReportConfigRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal update report config request: %w", err)
	}

	httpReq, err := http.NewRequest("PATCH", fmt.Sprintf("%s%s/%s", c.HostURL, reportConfigsPath, reportConfigID), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	_, err = c.DoRequest(httpReq)
	if err != nil {
		return fmt.Errorf("failed to update report config: %w", err)
	}

	return nil
}

// Deletes a report configuration by ID
func (c *CrmClient) DeleteReportConfig(reportConfigID string) error {
	httpReq, err := http.NewRequest("DELETE", fmt.Sprintf("%s%s/%s", c.HostURL, reportConfigsPath, reportConfigID), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	_, err = c.DoRequest(httpReq)
	if err != nil {
		return fmt.Errorf("failed to delete report config: %w", err)
	}

	return nil
}
