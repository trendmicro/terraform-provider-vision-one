# Company-Level Generic Report Configuration
#
# Company-level reports aggregate data across all accounts in the company.
# To create a company-level report, omit both account_id and group_id.

# Example 1: Company-level generic report WITHOUT filter
resource "visionone_crm_report_config" "company_generic_no_filter" {
  # No account_id or group_id = company level
  report_title            = "Company-Wide Security Overview"
  report_type             = "GENERIC"
  include_checks          = true
  include_account_names   = true # Show individual account names
  email_recipients        = ["ciso@example.com", "executive@example.com"]
  report_formats_in_email = ["PDF"]

  # Monthly scheduled report
  schedule {
    frequency = "1 * *" # 1st of every month
    timezone  = "America/New_York"
  }
}

# Example 2: Company-level generic report WITH comprehensive filters
resource "visionone_crm_report_config" "company_generic_with_filter" {
  # No account_id or group_id = company level
  report_title            = "Company-Wide High-Risk Security Report"
  report_type             = "GENERIC"
  include_checks          = true
  include_account_names   = true
  email_recipients        = ["security-team@example.com", "compliance@example.com"]
  report_formats_in_email = ["PDF", "CSV"]

  # Weekly scheduled report
  schedule {
    frequency = "* * 1" # Every Monday
    timezone  = "America/New_York"
  }

  # Comprehensive filtering for high-risk items
  checks_filter {
    # Filter by security category only
    categories = ["security"]

    # High-risk items only
    risk_levels = ["HIGH", "VERY_HIGH", "EXTREME"]

    # Only show failures
    statuses = ["FAILURE"]

    # Recent findings (last 30 days)
    newer_than_days = 30

    # Multi-cloud environment
    providers = ["aws", "azure", "gcp"]

    # Critical regions
    regions = [
      "us-east-1",
      "us-west-2",
      "eu-west-1",
      "ap-southeast-1"
    ]

    # Critical services
    services = [
      "S3",
      "IAM",
      "EC2",
      "KMS",
      "RDS"
    ]

    # Exclude suppressed items
    suppressed = false

    # Filter by compliance standards
    compliance_standard_ids = ["NIST4", "AWAF-2025"]
  }
}

# Example 3: On-demand company-level report (no schedule)
resource "visionone_crm_report_config" "company_generic_on_demand" {
  # No account_id or group_id = company level
  report_title            = "Company-Wide Incident Report"
  report_type             = "GENERIC"
  include_checks          = true
  include_account_names   = true
  email_recipients        = ["incident-response@example.com"]
  report_formats_in_email = ["all"]

  # No schedule block = on-demand report

  checks_filter {
    # Extreme risk only for incident investigation
    risk_levels = ["EXTREME"]
    statuses    = ["FAILURE"]

    # Last 24 hours only
    newer_than_days = 1

    # All categories for comprehensive view
    categories = [
      "security",
      "cost-optimisation",
      "reliability",
      "performance-efficiency",
      "operational-excellence",
      "sustainability"
    ]
  }
}
