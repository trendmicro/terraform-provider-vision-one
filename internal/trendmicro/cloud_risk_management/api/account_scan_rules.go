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
	accountRuleSettingsGetPath    = "/beta/cloudPosture/accounts/%s/scanRules"
	accountRuleSettingsUpdatePath = "/beta/cloudPosture/accounts/%s/scanRules/update"
	accountRuleSettingsDeletePath = "/beta/cloudPosture/accounts/%s/scanRules/delete"
)

// PartialFailureError represents a partial failure in a multi-status API response.
// Some rules were successfully processed while others failed.
type PartialFailureError struct {
	FailedRuleIDs []string
	FailCount     int
	TotalCount    int
	Details       string
	Operation     string // e.g. "update" or "reset"
}

func (e *PartialFailureError) Error() string {
	return fmt.Sprintf("%d scan rule setting(s) failed to %s:\n%s",
		e.FailCount, e.Operation, e.Details)
}

// GetAccountRuleSettings retrieves all rule settings for a CRM account, handling pagination.
func (c *CrmClient) GetAccountRuleSettings(accountID string) ([]cloud_risk_management_dto.AccountRuleSetting, error) {
	var allRuleSettings []cloud_risk_management_dto.AccountRuleSetting
	requestURL := fmt.Sprintf("%s%s?top=100", c.HostURL, fmt.Sprintf(accountRuleSettingsGetPath, accountID))

	for requestURL != "" {
		httpReq, err := http.NewRequest("GET", requestURL, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		httpReq.Header.Set("TMV1-Filter", "isCustomized eq 'true'")

		body, err := c.DoRequest(httpReq)
		if err != nil {
			// 404 means no customized rules found for this account — return empty slice.
			if IsNotFoundError(err) {
				return allRuleSettings, nil
			}
			return nil, fmt.Errorf("failed to get account scan rule settings: %w", err)
		}

		var response cloud_risk_management_dto.AccountRuleSettingsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal account scan rule settings response: %w", err)
		}

		allRuleSettings = append(allRuleSettings, response.Items...)
		requestURL = response.NextLink
	}

	return allRuleSettings, nil
}

// UpdateAccountRuleSettings updates rule settings for a CRM account.
// The API returns a 207 Multi-Status response with per-rule results.
// If any rule update fails (non-2xx status), the entire operation is treated as failed.
func (c *CrmClient) UpdateAccountRuleSettings(accountID string, ruleSettings []cloud_risk_management_dto.AccountRuleSettingUpdate) error {
	body, err := json.Marshal(ruleSettings)
	if err != nil {
		return fmt.Errorf("failed to marshal update account scan rule settings request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s%s", c.HostURL, fmt.Sprintf(accountRuleSettingsUpdatePath, accountID)), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	respBody, err := c.DoRequest(httpReq)
	if err != nil {
		return fmt.Errorf("failed to update account scan rule settings: %w", err)
	}

	// Parse the 207 multi-status response and check for per-rule failures
	var responses []cloud_risk_management_dto.MultiStatusResponseItem
	if err := json.Unmarshal(respBody, &responses); err != nil {
		return fmt.Errorf("failed to unmarshal update response: %w", err)
	}

	var failedRuleIDs []string
	var errors []string
	for i, resp := range responses {
		if resp.Status != http.StatusNoContent {
			ruleID := "unknown"
			if i < len(ruleSettings) {
				ruleID = ruleSettings[i].ID
			}
			failedRuleIDs = append(failedRuleIDs, ruleID)
			errors = append(errors, fmt.Sprintf("rule %s (status %d): %s", ruleID, resp.Status, string(resp.Body)))
		}
	}

	if len(errors) > 0 {
		return &PartialFailureError{
			FailedRuleIDs: failedRuleIDs,
			FailCount:     len(errors),
			TotalCount:    len(responses),
			Details:       joinErrors(errors),
			Operation:     "update",
		}
	}

	return nil
}

// DeleteAccountRuleSettings resets (deletes) rule settings for the specified rule IDs on a CRM account.
// The API returns a 207 Multi-Status response with per-rule results.
// Status 204 means reset successful, 404 means rule was not customized (no reset needed).
// Any other status is treated as a failure.
func (c *CrmClient) DeleteAccountRuleSettings(accountID string, ruleIDs []string) error {
	payload := make([]cloud_risk_management_dto.AccountRuleSettingDeleteItem, len(ruleIDs))
	for i, id := range ruleIDs {
		payload[i] = cloud_risk_management_dto.AccountRuleSettingDeleteItem{ID: id}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal delete account scan rule settings request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s%s", c.HostURL, fmt.Sprintf(accountRuleSettingsDeletePath, accountID)), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	respBody, err := c.DoRequest(httpReq)
	if err != nil {
		return fmt.Errorf("failed to delete account scan rule settings: %w", err)
	}

	var responses []cloud_risk_management_dto.MultiStatusResponseItem
	if err := json.Unmarshal(respBody, &responses); err != nil {
		return fmt.Errorf("failed to unmarshal delete response: %w", err)
	}

	var failedRuleIDs []string
	var errors []string
	for i, resp := range responses {
		// 204 = reset successful, 404 = not customized (no reset needed) — both are acceptable
		if resp.Status != http.StatusNoContent && resp.Status != http.StatusNotFound {
			ruleID := "unknown"
			if i < len(ruleIDs) {
				ruleID = ruleIDs[i]
			}
			failedRuleIDs = append(failedRuleIDs, ruleID)
			errors = append(errors, fmt.Sprintf("rule %s (status %d): %s", ruleID, resp.Status, string(resp.Body)))
		}
	}

	if len(errors) > 0 {
		return &PartialFailureError{
			FailedRuleIDs: failedRuleIDs,
			FailCount:     len(errors),
			TotalCount:    len(responses),
			Details:       joinErrors(errors),
			Operation:     "reset",
		}
	}

	return nil
}

func joinErrors(errs []string) string {
	var b strings.Builder
	for i, e := range errs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("  - ")
		b.WriteString(e)
	}
	return b.String()
}
