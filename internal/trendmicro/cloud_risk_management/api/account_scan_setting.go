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
	accountScanSettingsPath = "/beta/cloudPosture/accounts/{id}/scanSetting"
)

// GetAccountScanSetting retrieves an account scan setting by ID.
func (c *CrmClient) GetAccountScanSetting(accountID string) (*cloud_risk_management_dto.AccountScanSetting, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s%s", c.HostURL, strings.Replace(accountScanSettingsPath, "{id}", accountID, 1)), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	body, err := c.DoRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get account scan setting: %w", err)
	}

	var setting cloud_risk_management_dto.AccountScanSetting
	if err := json.Unmarshal(body, &setting); err != nil {
		return nil, fmt.Errorf("failed to unmarshal account scan setting response: %w", err)
	}

	return &setting, nil
}

// UpdateAccountScanSetting updates an existing account scan setting.
func (c *CrmClient) UpdateAccountScanSetting(accountID string, req *cloud_risk_management_dto.AccountScanSetting) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal update account scan setting request: %w", err)
	}

	httpReq, err := http.NewRequest("PATCH", fmt.Sprintf("%s%s", c.HostURL, strings.Replace(accountScanSettingsPath, "{id}", accountID, 1)), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	_, err = c.DoRequest(httpReq)
	if err != nil {
		return fmt.Errorf("failed to update account scan setting: %w", err)
	}

	return nil
}
