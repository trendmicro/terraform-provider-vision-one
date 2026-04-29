package cloud_risk_management_dto

// AccountRuleSettingsResponse represents the paginated response from GET account rule settings API.
type AccountRuleSettingsResponse struct {
	Count    int                  `json:"count"`
	NextLink string               `json:"nextLink"`
	Items    []AccountRuleSetting `json:"items"`
}

// AccountRuleSetting represents a rule setting item in the GET account rule settings API response.
type AccountRuleSetting struct {
	ScanRule
	IsCustomized bool `json:"isCustomized"`
}

// AccountRuleSettingUpdate extends ScanRule with a Note field for the update API request.
type AccountRuleSettingUpdate struct {
	ScanRule
	Note string `json:"note"`
}

// AccountRuleSettingDeleteItem represents a single item in the delete/reset request payload.
type AccountRuleSettingDeleteItem struct {
	ID string `json:"id"`
}
