package cloud_risk_management_dto

// ScanRule represents a rule configuration shared across profiles and accounts.
type ScanRule struct {
	ID            string             `json:"id"`
	Provider      string             `json:"provider"`
	Enabled       bool               `json:"enabled"`
	RiskLevel     string             `json:"riskLevel"`
	Deprecated    bool               `json:"deprecated,omitempty"`
	ExtraSettings []RuleExtraSetting `json:"extraSettings,omitempty"`
	Exceptions    *RuleExceptions    `json:"exceptions,omitempty"`
}

// RuleExceptions represents exceptions for a scan rule.
type RuleExceptions struct {
	FilterTags  []string `json:"tags"`
	ResourceIds []string `json:"resourceIds"`
}

// RuleExtraSetting represents additional configuration for a scan rule.
// Values is a pointer-to-slice so that omitempty distinguishes nil (omit field)
// from empty slice (serialize as "values": []).
type RuleExtraSetting struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Values *[]any `json:"values,omitempty"`
	Value  any    `json:"value,omitempty"`
}

// RuleExtraSettingMapping wraps a slice of RuleExtraSetting.
type RuleExtraSettingMapping struct {
	Values []RuleExtraSetting `json:"values"`
}
