package cloud_risk_management_dto

type ReportConfig struct {
	ID                   string              `json:"id,omitempty"`
	CreatedDateTime      string              `json:"createdDateTime,omitempty"`
	UpdatedDateTime      string              `json:"updatedDateTime,omitempty"`
	Level                string              `json:"level,omitempty"`
	AccountID            string              `json:"accountId,omitempty"`
	GroupID              string              `json:"groupId,omitempty"`
	ReportTitle          string              `json:"reportTitle"`
	ReportType           string              `json:"reportType"`
	IncludeChecks        bool                `json:"includeChecks,omitempty"`
	IncludeAccountNames  *bool               `json:"includeAccountNames,omitempty"`
	EmailRecipients      []string            `json:"emailRecipients,omitempty"`
	ReportFormatsInEmail []string            `json:"reportFormatsInEmail,omitempty"`
	Language             string              `json:"language,omitempty"`
	ChecksFilter         *ReportConfigFilter `json:"checksFilter,omitempty"`
	Schedule             *ReportSchedule     `json:"schedule,omitempty"`
	// Compliance Standard Report specific fields
	AppliedComplianceStandardID string                    `json:"appliedComplianceStandardId,omitempty"`
	AppliedComplianceStandard   *ComplianceStandardObject `json:"appliedComplianceStandard,omitempty"`
	ControlsType                string                    `json:"controlsType,omitempty"`
}

type ReportConfigFilter struct {
	Categories            *[]string                   `json:"categories"`
	Tags                  *[]string                   `json:"tags"`
	Description           *string                     `json:"description"`
	NewerThanDays         *int                        `json:"newerThanDays"`
	OlderThanDays         *int                        `json:"olderThanDays"`
	Providers             *[]string                   `json:"providers"`
	Regions               *[]string                   `json:"regions"`
	ResourceID            *string                     `json:"resourceId"`
	ResourceSearchMode    *string                     `json:"resourceSearchMode"`
	ResourceTypes         *[]string                   `json:"resourceTypes"`
	RiskLevels            *[]string                   `json:"riskLevels"`
	RuleIds               *[]string                   `json:"ruleIds"`
	Services              *[]string                   `json:"services"`
	Statuses              *[]string                   `json:"statuses"`
	Suppressed            *bool                       `json:"suppressed"`
	ComplianceStandardIds *[]string                   `json:"complianceStandardIds"`         // Used in POST/PATCH requests
	ComplianceStandards   *[]ComplianceStandardObject `json:"complianceStandards,omitempty"` // Returned by GET API (read-only)
}

type ReportSchedule struct {
	Enabled   *bool  `json:"enabled,omitempty"`
	Frequency string `json:"frequency,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
}

type ComplianceStandardObject struct {
	ID string `json:"id"`
}

type CreateReportConfigRequest struct {
	// Account/Group/Company level
	AccountID string `json:"accountId,omitempty"`
	GroupID   string `json:"groupId,omitempty"`

	// Common fields
	ReportTitle          string              `json:"reportTitle"`
	ReportType           string              `json:"reportType"`
	IncludeChecks        *bool               `json:"includeChecks,omitempty"`
	IncludeAccountNames  *bool               `json:"includeAccountNames,omitempty"`
	EmailRecipients      []string            `json:"emailRecipients,omitempty"`
	ReportFormatsInEmail []string            `json:"reportFormatsInEmail,omitempty"`
	Language             string              `json:"language,omitempty"`
	ChecksFilter         *ReportConfigFilter `json:"checksFilter,omitempty"`
	Schedule             *ReportSchedule     `json:"schedule,omitempty"`

	// Compliance Standard Report specific
	AppliedComplianceStandardID string `json:"appliedComplianceStandardId,omitempty"`
	ControlsType                string `json:"controlsType,omitempty"`
}

type UpdateReportConfigRequest struct {
	ReportTitle          string              `json:"reportTitle,omitempty"`
	ReportType           string              `json:"reportType,omitempty"`
	IncludeChecks        *bool               `json:"includeChecks,omitempty"`
	IncludeAccountNames  *bool               `json:"includeAccountNames,omitempty"`
	EmailRecipients      []string            `json:"emailRecipients,omitempty"`
	ReportFormatsInEmail []string            `json:"reportFormatsInEmail,omitempty"`
	Language             string              `json:"language,omitempty"`
	ChecksFilter         *ReportConfigFilter `json:"checksFilter,omitempty"`
	Schedule             *ReportSchedule     `json:"schedule,omitempty"`

	// Compliance Standard Report specific
	AppliedComplianceStandardID string `json:"appliedComplianceStandardId,omitempty"`
	ControlsType                string `json:"controlsType,omitempty"`
}
