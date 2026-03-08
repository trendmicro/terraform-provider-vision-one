# Company-Level Compliance Standard Report Configuration
#
# Company-level compliance reports aggregate compliance data across all accounts.
# To create a company-level report, omit both account_id and group_id.
# Requires: applied_compliance_standard_id when report_type is "COMPLIANCE-STANDARD"
#

# Example 1: Company-level NIST compliance report WITHOUT filter
resource "visionone_crm_report_config" "company_nist_no_filter" {
  # No account_id or group_id = company level
  report_title                   = "Company-Wide NIST 800-53 Compliance Report"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "all" # Options: withChecksOnly, noChecksOnly, all
  include_checks                 = true
  include_account_names          = true
  email_recipients               = ["compliance@example.com", "ciso@example.com"]
  report_formats_in_email        = ["PDF"]

  # Quarterly reporting
  schedule {
    frequency = "1 1,4,7,10 *" # 1st day of Jan, Apr, Jul, Oct
    timezone  = "America/New_York"
  }
}

# Example 2: Company-level NIST compliance report WITH filters
resource "visionone_crm_report_config" "company_nist_with_filter" {
  # No account_id or group_id = company level
  report_title                   = "Company-Wide NIST Critical Controls Report"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "withChecksOnly" # Only controls with checks
  include_checks                 = true
  include_account_names          = true
  email_recipients               = ["audit@example.com", "compliance-team@example.com"]
  report_formats_in_email        = ["PDF", "CSV"]

  # Monthly reporting
  schedule {
    frequency = "1 * *" # 1st of every month
    timezone  = "America/New_York"
  }

  # Filter for critical compliance failures
  checks_filter {
    # Only show failures
    statuses = ["FAILURE"]

    # High-risk items
    risk_levels = ["HIGH", "VERY_HIGH", "EXTREME"]

    # Recent findings
    newer_than_days = 90

    # Critical infrastructure
    resource_types = [
      "s3-bucket",
      "kms-key",
      "iam-role",
      "ec2-instance",
      "rds-instance"
    ]

    # Exclude suppressed items
    suppressed = false
  }
}

# Example 3: On-demand company-level compliance audit
resource "visionone_crm_report_config" "company_compliance_audit" {
  # No account_id or group_id = company level
  report_title                   = "Company-Wide Audit Preparation Report"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "all"
  include_checks                 = true
  include_account_names          = true
  email_recipients               = ["audit-prep@example.com"]
  report_formats_in_email        = ["all"]

  # No schedule block = on-demand report

  checks_filter {
    # Show both successes and failures for complete audit view
    statuses = ["SUCCESS", "FAILURE"]

    # Include all findings from the last year
    newer_than_days = 365

    # Multi-cloud environment
    providers = ["aws", "azure", "gcp"]

    # Include suppressed items for audit visibility
    suppressed = true
  }
}
