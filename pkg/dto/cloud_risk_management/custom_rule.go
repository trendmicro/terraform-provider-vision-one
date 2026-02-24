package cloud_risk_management_dto

// CustomRule represents a Cloud Risk Management custom rule
type CustomRule struct {
	ID                      string              `json:"id,omitempty"`
	Name                    string              `json:"name"`
	Description             string              `json:"description"`
	Categories              []string            `json:"categories"`
	RiskLevel               string              `json:"riskLevel"`
	Provider                string              `json:"provider"`
	ResolutionReferenceLink string              `json:"resolutionReferenceLink,omitempty"`
	RemediationNote         string              `json:"remediationNote,omitempty"`
	Enabled                 bool                `json:"enabled"`
	Service                 string              `json:"service"`
	ResourceType            string              `json:"resourceType"`
	Attributes              []ResourceAttribute `json:"attributes"`
	EventRules              []EventRule         `json:"eventRules"`
	Slug                    string              `json:"slug,omitempty"`
}

// ResourceAttribute represents an attribute to be evaluated
type ResourceAttribute struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Required bool   `json:"required"`
}

// Conditions represents the conditions for evaluation
// Uses Any or All arrays
type Conditions struct {
	Any []Condition `json:"any,omitempty"` // For event rule conditions (oneOf)
	All []Condition `json:"all,omitempty"` // For event rule conditions (oneOf)
}

// Condition represents a single condition (and optionally nested Any/All)
type Condition struct {
	Fact     string      `json:"fact"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
	Path     string      `json:"path,omitempty"`
	Any      []Condition `json:"any,omitempty"` // Nested any conditions
	All      []Condition `json:"all,omitempty"` // Nested all conditions
}

// EventRule represents an event to be evaluated
type EventRule struct {
	Description string      `json:"description"`
	Conditions  *Conditions `json:"conditions"`
}

// CreateCustomRuleRequest represents the request to create a custom rule
type CreateCustomRuleRequest struct {
	Name                    string              `json:"name"`
	Description             string              `json:"description"`
	Categories              []string            `json:"categories"`
	RiskLevel               string              `json:"riskLevel"`
	Provider                string              `json:"provider"`
	ResolutionReferenceLink string              `json:"resolutionReferenceLink,omitempty"`
	RemediationNote         string              `json:"remediationNote,omitempty"`
	Enabled                 bool                `json:"enabled"`
	Service                 string              `json:"service"`
	ResourceType            string              `json:"resourceType"`
	Attributes              []ResourceAttribute `json:"attributes"`
	EventRules              []EventRule         `json:"eventRules"`
	Slug                    string              `json:"slug,omitempty"`
}

// UpdateCustomRuleRequest represents the request to update a custom rule
type UpdateCustomRuleRequest struct {
	Name                    string              `json:"name,omitempty"`
	Description             string              `json:"description,omitempty"`
	Categories              []string            `json:"categories,omitempty"`
	RiskLevel               string              `json:"riskLevel,omitempty"`
	Provider                string              `json:"provider,omitempty"`
	ResolutionReferenceLink string              `json:"resolutionReferenceLink,omitempty"`
	RemediationNote         string              `json:"remediationNote,omitempty"`
	Enabled                 *bool               `json:"enabled,omitempty"`
	Service                 string              `json:"service,omitempty"`
	ResourceType            string              `json:"resourceType,omitempty"`
	Attributes              []ResourceAttribute `json:"attributes,omitempty"`
	EventRules              []EventRule         `json:"eventRules,omitempty"`
}
