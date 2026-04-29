package cloud_risk_management_dto

// Profile represents a Cloud Risk Management profile
type Profile struct {
	ID          string     `json:"id,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	ScanRules   []ScanRule `json:"scanRules,omitempty"`
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
