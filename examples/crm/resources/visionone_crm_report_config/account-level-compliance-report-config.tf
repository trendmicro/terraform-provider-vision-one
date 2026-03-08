# Account-Level Compliance Standard Report Configuration
#
# Account-level compliance reports focus on a specific cloud account's compliance posture.
# Use Cloud Risk Management(CRM) account_id to specify the account. Do not specify group_id.
# Requires: applied_compliance_standard_id when report_type is "COMPLIANCE-STANDARD"
#

# Example 1: Account-level NIST compliance report WITHOUT filter
resource "visionone_crm_report_config" "account_nist_no_filter" {
  account_id                     = "0114cc1b-4b0c-4130-86e0-1ff4a13fcac5b"
  report_title                   = "AWS Account NIST 800-53 Compliance"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "all"
  include_checks                 = true
  email_recipients               = ["compliance@example.com"]
  report_formats_in_email        = ["PDF"]

  # Monthly reporting
  schedule {
    frequency = "1 * *" # 1st of every month
    timezone  = "America/New_York"
  }
}

# Example 2: Account-level NIST compliance report WITH filters
resource "visionone_crm_report_config" "account_nist_with_filter" {
  account_id                     = "0114cc1b-4b0c-4130-86e0-1ff4a13fcac5b"
  report_title                   = "AWS Account NIST Critical Controls"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "withChecksOnly" # Only controls with checks
  include_checks                 = true
  email_recipients               = ["security-team@example.com", "compliance@example.com"]
  report_formats_in_email        = ["PDF", "CSV"]

  # Weekly reporting
  schedule {
    frequency = "* * 1" # Every Monday
    timezone  = "America/New_York"
  }

  # Filter for critical compliance failures
  checks_filter {
    statuses = ["FAILURE"]

    risk_levels = ["HIGH", "VERY_HIGH", "EXTREME"]

    # Recent findings (last 90 days)
    newer_than_days = 90

    # Production regions
    regions = ["us-east-1", "us-west-2"]

    # Critical compliance resource types
    resource_types = [
      "s3-bucket",
      "kms-key",
      "iam-role",
      "iam-policy",
      "ec2-instance",
      "rds-instance"
    ]

    # Exclude suppressed
    suppressed = false
  }
}

# Example 3: Account-level controls without checks - gap analysis
resource "visionone_crm_report_config" "account_nist_gap_analysis" {
  account_id                     = "aws-account-123456789012"
  report_title                   = "NIST Controls Gap Analysis"
  report_type                    = "COMPLIANCE-STANDARD"
  applied_compliance_standard_id = "NIST4"
  controls_type                  = "noChecksOnly" # Only controls without checks
  include_checks                 = false
  email_recipients               = ["compliance-gap@example.com"]
  report_formats_in_email        = ["PDF"]

  # Monthly gap analysis
  schedule {
    frequency = "1 * *" # 1st of every month
    timezone  = "Australia/Sydney"
  }
}
