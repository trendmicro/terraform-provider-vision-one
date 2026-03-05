# Group-Level Compliance Standard Report Configuration
#
# Group-level compliance reports aggregate compliance data from multiple accounts within a group.
# Use Cloud Risk Management(CRM) group_id to specify the group. Do not specify account_id.
# Requires: applied_compliance_standard_id when report_type is "COMPLIANCE-STANDARD"
#

# Example 1: Group-level NIST compliance report WITHOUT filter
resource "visionone_crm_report_config" "group_nist_no_filter" {
  group_id                       = "1234abcd-4b0c-4130-86e0-1ff4a13fcacba"
  report_title                   = "Production Group NIST Compliance"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "all"
  include_checks                 = true
  include_account_names          = true
  email_recipients               = ["compliance@example.com"]
  report_formats_in_email        = ["PDF"]

  # Monthly reporting
  schedule {
    frequency = "1 * *" # 1st of every month
    timezone  = "America/New_York"
  }
}

# Example 2: Group-level NIST compliance report WITH filters
resource "visionone_crm_report_config" "group_nist_with_filter" {
  group_id                       = "1234abcd-4b0c-4130-86e0-1ff4a13fcacba"
  report_title                   = "Production Group NIST Critical Findings"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "withChecksOnly"
  include_checks                 = true
  include_account_names          = true
  email_recipients               = ["security-team@example.com", "compliance@example.com"]
  report_formats_in_email        = ["PDF", "CSV"]

  # Weekly reporting
  schedule {
    frequency = "* * 1" # Every Monday
    timezone  = "America/New_York"
  }

  checks_filter {
    # Only failures
    statuses = ["FAILURE"]

    # High-risk items only
    risk_levels = ["HIGH", "VERY_HIGH", "EXTREME"]

    # Recent findings (last 60 days)
    newer_than_days = 60

    # Production regions
    regions = ["us-east-1", "us-west-2", "eu-west-1"]

    # Critical resource types for compliance
    resource_types = [
      "s3-bucket",
      "kms-key",
      "iam-role",
      "iam-policy",
      "ec2-instance",
      "ct-trail"
    ]

    # Exclude suppressed
    suppressed = false
  }
}

# Example 3: On-demand group-level compliance audit
resource "visionone_crm_report_config" "group_compliance_audit" {
  group_id                       = "1234abcd-4b0c-4130-86e0-1ff4a13fcacba"
  report_title                   = "Production Group Compliance Audit"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "all"
  include_checks                 = true
  include_account_names          = true
  email_recipients               = ["audit@example.com", "compliance-audit@example.com"]
  report_formats_in_email        = ["all"]

  # No schedule block = on-demand report

  checks_filter {
    # Include both successes and failures for audit
    statuses = ["SUCCESS", "FAILURE"]

    # All findings from last 12 months
    newer_than_days = 365

    # Include suppressed items for complete audit trail
    suppressed = true
  }
}
