# Group-Level Generic Report Configuration
#
# Group-level reports aggregate data from multiple cloud accounts within a group.
# Use Cloud Risk Management(CRM) group_id to specify the group. Do not specify account_id.

# Example 1: Group-level generic report WITHOUT filter
resource "visionone_crm_report_config" "group_generic_no_filter" {
  group_id                = "1234abcd-4b0c-4130-86e0-1ff4a13fcacba"
  report_title            = "Production Group Security Report"
  report_type             = "GENERIC"
  include_checks          = true
  include_account_names   = true # Show individual account names
  email_recipients        = ["prod-team@example.com"]
  report_formats_in_email = ["PDF"]

  # Daily reporting
  schedule {
    frequency = "* * *" # Every day
    timezone  = "America/New_York"
  }
}

# Example 2: Group-level generic report WITH comprehensive filters
resource "visionone_crm_report_config" "group_generic_with_filter" {
  group_id                = "1234abcd-4b0c-4130-86e0-1ff4a13fcacba"
  report_title            = "Production Group High-Risk Report"
  report_type             = "GENERIC"
  include_checks          = true
  include_account_names   = true
  email_recipients        = ["security@example.com", "devops@example.com"]
  report_formats_in_email = ["PDF", "CSV"]

  # Weekly reporting
  schedule {
    frequency = "* * 2" # Every Tuesday
    timezone  = "Australia/Sydney"
  }

  # Filter for production-specific concerns
  checks_filter {
    categories = ["security", "reliability", "performance-efficiency"]

    risk_levels = ["HIGH", "VERY_HIGH", "EXTREME"]

    statuses = ["FAILURE"]

    # Recent findings
    newer_than_days = 14

    # Production regions only
    regions = [
      "us-east-1",
      "us-west-2",
      "eu-west-1"
    ]

    # Critical production services
    services = [
      "EC2",
      "RDS",
      "ELB",
      "S3",
      "Lambda"
    ]

    # Filter by production tags
    tags = ["Environment:Production", "Severity:High"]

    # Exclude suppressed items
    suppressed = false
  }
}

# Example 3: Multi-cloud group-level report
resource "visionone_crm_report_config" "group_generic_multicloud" {
  group_id                = "group-multicloud-003"
  report_title            = "Multi-Cloud Group Security Report"
  report_type             = "GENERIC"
  include_checks          = true
  include_account_names   = true
  email_recipients        = ["cloud-ops@example.com"]
  report_formats_in_email = ["PDF"]

  schedule {
    frequency = "1 * *" # 1st of every month
    timezone  = "Australia/Sydney"
  }

  checks_filter {
    # Multi-cloud providers
    providers = ["aws", "azure", "gcp"]

    categories = ["security", "cost-optimisation"]

    risk_levels = ["MEDIUM", "HIGH", "VERY_HIGH", "EXTREME"]

    statuses = ["FAILURE"]

    # Filter by compliance standards
    compliance_standard_ids = ["NIST4", "AWAF-2025"]

    newer_than_days = 30
  }
}
