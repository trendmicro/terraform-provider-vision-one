package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
)

const (
	customRulesPath = "/beta/cloudPosture/customRules"
)

// IsNotFoundError checks if the error is a 404 Not Found error
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") || strings.Contains(err.Error(), "NotFound")
}

// CreateCustomRule creates a new custom rule
func (c *CrmClient) CreateCustomRule(req *cloud_risk_management_dto.CreateCustomRuleRequest) (*cloud_risk_management_dto.CustomRule, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s%s", c.HostURL, customRulesPath), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	response, err := c.DoRequestWithFullResponse(httpReq)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Extract the ID from the Location header
	customRuleID, err := extractIDFromLocation(&response.Header)
	if err != nil {
		return nil, err
	}

	if customRuleID == "" {
		return nil, fmt.Errorf("failed to extract custom rule ID from response header")
	}

	// Build the response from the request data
	customRule := &cloud_risk_management_dto.CustomRule{
		ID:              customRuleID,
		Name:            req.Name,
		Description:     req.Description,
		Categories:      req.Categories,
		RiskLevel:       req.RiskLevel,
		Provider:        req.Provider,
		RemediationNote: req.RemediationNote,
		Enabled:         req.Enabled,
		Service:         req.Service,
		ResourceType:    req.ResourceType,
		Attributes:      req.Attributes,
		EventRules:      req.EventRules,
		Slug:            req.Slug,
	}

	return customRule, nil
}

// GetCustomRule retrieves a custom rule by ID
func (c *CrmClient) GetCustomRule(customRuleID string) (*cloud_risk_management_dto.CustomRule, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s%s/%s", c.HostURL, customRulesPath, customRuleID), http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.DoRequest(httpReq)
	if err != nil {
		return nil, err
	}

	var customRule cloud_risk_management_dto.CustomRule
	if err := json.Unmarshal(body, &customRule); err != nil {
		return nil, err
	}

	return &customRule, nil
}

// UpdateCustomRule updates an existing custom rule
func (c *CrmClient) UpdateCustomRule(customRuleID string, req *cloud_risk_management_dto.UpdateCustomRuleRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("PATCH", fmt.Sprintf("%s%s/%s", c.HostURL, customRulesPath, customRuleID), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	_, err = c.DoRequest(httpReq)
	if err != nil {
		return err
	}

	return nil
}

// DeleteCustomRule deletes a custom rule
func (c *CrmClient) DeleteCustomRule(customRuleID string) error {
	httpReq, err := http.NewRequest("DELETE", fmt.Sprintf("%s%s/%s", c.HostURL, customRulesPath, customRuleID), http.NoBody)
	if err != nil {
		return err
	}

	_, err = c.DoRequest(httpReq)
	if err != nil {
		return err
	}

	return nil
}
