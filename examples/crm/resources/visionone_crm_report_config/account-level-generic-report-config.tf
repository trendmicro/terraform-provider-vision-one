# Account-Level Generic Report Configuration
#
# Account-level reports focus on a specific cloud account.
# Use Cloud Risk Management(CRM) account_id to specify the account. Do not specify group_id.

# Example 1: Account-level generic report WITHOUT filter
resource "visionone_crm_report_config" "account_generic_no_filter" {
  account_id              = "0114cc1b-4b0c-4130-86e0-1ff4a13fcac5b"
  report_title            = "AWS Account Security Overview"
  report_type             = "GENERIC"
  include_checks          = true
  email_recipients        = ["account-owner@example.com"]
  report_formats_in_email = ["PDF"]

  # Daily reporting
  schedule {
    frequency = "* * *" # Every day
    timezone  = "America/New_York"
  }
}

# Example 2: Account-level generic report WITH comprehensive filters
resource "visionone_crm_report_config" "account_generic_with_filter" {
  account_id              = "0114cc1b-4b0c-4130-86e0-1ff4a13fcac5b"
  report_title            = "AWS Account High-Risk Security Report"
  report_type             = "GENERIC"
  include_checks          = true
  email_recipients        = ["security@example.com", "devops@example.com"]
  report_formats_in_email = ["PDF", "CSV"]

  # Weekly reporting
  schedule {
    frequency = "* * 1" # Every Monday
    timezone  = "America/New_York"
  }

  # Comprehensive filtering
  checks_filter {
    categories = ["security", "cost-optimisation", "reliability"]

    risk_levels = ["HIGH", "VERY_HIGH", "EXTREME"]

    statuses = ["FAILURE"]

    # Recent findings (last 7 days)
    newer_than_days = 7

    # Specific regions
    regions = ["us-east-1", "us-west-2"]

    # Critical services
    services = ["S3", "IAM", "EC2", "KMS", "RDS", "Lambda"]

    # Exclude suppressed items
    suppressed = false

    # Filter by compliance standards
    compliance_standard_ids = ["NIST4", "AWAF-2025"]
  }
}

# Example 3: On-demand report with resource filtering
resource "visionone_crm_report_config" "account_on_demand" {
  account_id              = "0114cc1b-4b0c-4130-86e0-1ff4a13fcac5b"
  report_title            = "Production Resources Security"
  report_type             = "GENERIC"
  include_checks          = true
  email_recipients        = ["prod-team@example.com"]
  report_formats_in_email = ["all"]

  # No schedule block = on-demand report

  checks_filter {
    # Filter resources with "prod-" prefix
    resource_id          = "prod-"
    resource_search_mode = "text"

    # Filter by production tags
    tags = ["Environment:Production", "Team:Platform"]

    categories = ["security"]

    statuses = ["FAILURE"]
  }
}
