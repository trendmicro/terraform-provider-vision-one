package cloud_risk_management_dto

// ApplyProfileRequest represents the request body for applying a profile to accounts.
type ApplyProfileRequest struct {
	AccountIDs []string             `json:"accountIds"`
	Types      string               `json:"types"`
	Mode       string               `json:"mode"`
	Note       string               `json:"note,omitempty"`
	Include    *ApplyProfileInclude `json:"include,omitempty"`
}

// ApplyProfileInclude controls which fields are included when applying a profile.
type ApplyProfileInclude struct {
	Exceptions *bool `json:"exceptions,omitempty"`
}

// ApplyProfileResponse represents the response from applying a profile.
type ApplyProfileResponse struct {
	Meta    ApplyProfileResponseMeta `json:"meta,omitempty"`
	Results []ApplyProfileResult     `json:"-"`
}

// ApplyProfileResponseMeta contains status details for an apply operation.
type ApplyProfileResponseMeta struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ApplyProfileResult represents per-account apply status for multi-status responses.
type ApplyProfileResult struct {
	Status int `json:"status"`
}