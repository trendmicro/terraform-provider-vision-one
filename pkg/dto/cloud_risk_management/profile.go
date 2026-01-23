package cloud_risk_management_dto

// Profile represents a Cloud Risk Management profile
type Profile struct {
	ID          string     `json:"id,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	ScanRules   []ScanRule `json:"scanRules,omitempty"`
}

// ScanRule represents a rule configuration within a profile
type ScanRule struct {
	ID            string             `json:"id"`
	Provider      string             `json:"provider"`
	Enabled       bool               `json:"enabled"`
	RiskLevel     string             `json:"riskLevel"`
	Deprecated    bool               `json:"deprecated,omitempty"`
	ExtraSettings []RuleExtraSetting `json:"extraSettings,omitempty"`
	Exceptions    *RuleExceptions    `json:"exceptions,omitempty"`
}

// RuleExceptions represents exceptions for a scan rule
type RuleExceptions struct {
	FilterTags  []string `json:"tags,omitempty"`
	ResourceIds []string `json:"resourceIds,omitempty"`
}

// RuleExtraSetting represents additional configuration for a scan rule
type RuleExtraSetting struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Values []any  `json:"values,omitempty"`
	Value  any    `json:"value,omitempty"`
}

type RuleExtraSettingMapping struct {
	Values []RuleExtraSetting `json:"values"`
}

// CreateProfileRequest represents the request to create a profile
type CreateProfileRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	ScanRules   []ScanRule `json:"scanRules,omitempty"`
}

// UpdateProfileRequest represents the request to update a profile
type UpdateProfileRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	ScanRules   []ScanRule `json:"scanRules,omitempty"`
}

type CreateProfileResponse struct {
	Location string `json:"location,omitempty"`
}
