package cloud_risk_management_dto

// The request payload for updating a check
type UpdateCheckRequest struct {
	Suppressed bool   `json:"suppressed"`
	Note       string `json:"note"`

	SuppressedUntilDateTime string `json:"suppressedUntilDateTime,omitempty"` // ISO 8601 format with UTC timezone
}

// The response from GET /checks/{id} (Partial)
type GetCheckResponse struct {
	AccountID               string `json:"accountId"`
	Service                 string `json:"service"`
	RuleID                  string `json:"ruleId"`
	Region                  string `json:"region"`
	ResourceID              string `json:"resource"`
	Suppressed              bool   `json:"suppressed"`
	Note                    string `json:"note,omitempty"`
	SuppressedUntilDateTime string `json:"suppressedUntilDateTime,omitempty"`
}
