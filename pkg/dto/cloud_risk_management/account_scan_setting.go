package cloud_risk_management_dto

// The request payload for updating scan settings
type AccountScanSetting struct {
	DisabledRegions       []string `json:"disabledRegions"`
	DisabledUntilDateTime *string  `json:"disabledUntilDateTime"`
	Enabled               bool     `json:"enabled"`
	Interval              int      `json:"interval"` // in hours
}
