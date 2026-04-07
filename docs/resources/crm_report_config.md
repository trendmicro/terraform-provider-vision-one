---
page_title: "visionone_crm_report_config Resource - visionone"
subcategory: "Cloud Risk Management"
description: |-
  Manages a Cloud Risk Management report configuration for scheduled or on-demand compliance reports.
---

# visionone_crm_report_config (Resource)

Manages a Cloud Risk Management report configuration for scheduled or on-demand compliance reports.

## Example Usage

### Basic Company-Level Generic Report

```terraform
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
```

### Company-Level Compliance Report

```terraform
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
```
### Group-Level Generic Report

```terraform
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
```

### Group-Level Compliance Report

```terraform
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
```

### Account-Level Generic Report

```terraform
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
```

### Account-Level Compliance Report

```terraform
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
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `report_title` (String) The title of the report.

### Optional

- `account_id` (String) The Cloud Risk Management account ID to generate reports for. Omit both account_id and group_id for company-level reports. Cannot specify both account_id and group_id together.
- `applied_compliance_standard_id` (String) The ID of the compliance standard to apply (e.g., 'NIST4', 'AWAF-2025'). Required when report_type is COMPLIANCE-STANDARD.
- `checks_filter` (Block List) Filters to determine which checks appear in the report. Multiple conditions within a field use OR logic. Different fields use AND logic. (see [below for nested schema](#nestedblock--checks_filter))
- `controls_type` (String) The type of controls to display in PDF reports. Only available for COMPLIANCE-STANDARD reports, not for GENERIC reports. Allowed values: withChecksOnly (controls with checks), noChecksOnly (controls without checks), all (all controls). Default: all
- `email_recipients` (Set of String) List of email addresses to send the report to. Defaults to empty list if not specified.
- `group_id` (String) The Cloud Risk Management group ID to generate reports for. Omit both account_id and group_id for company-level reports. Cannot specify both account_id and group_id together.
- `include_account_names` (Boolean) Whether to include cloud account names in PDF reports. Only available for group-level and company-level reports. Cannot be used when account_id is provided.
- `include_checks` (Boolean) Whether to include individual checks in PDF reports. Default: false. Note: If the total number of checks exceeds 10,000, not all checks are included.
- `language` (String) The language for the report. Allowed values: en (English), ja (Japanese). Defaults to 'en' if not specified.
- `report_formats_in_email` (Set of String) The format of emailed reports. Allowed values: PDF, CSV, all. Default: ["all"].
- `report_type` (String) Type of report to generate. Allowed values: GENERIC, COMPLIANCE-STANDARD.
- `schedule` (Block List) Schedule configuration for automated report generation. (see [below for nested schema](#nestedblock--schedule))

### Read-Only

- `id` (String) The unique ID of the report configuration.
- `level` (String) The level of the report (account, group, or company). This is computed based on whether account_id or group_id is specified.

<a id="nestedblock--checks_filter"></a>
### Nested Schema for `checks_filter`

Optional:

- `categories` (Set of String) Filter by compliance categories. Allowed values: security, cost-optimisation, reliability, performance-efficiency, operational-excellence, sustainability.
- `compliance_standard_ids` (Set of String) Filter by compliance standard IDs (for GENERIC reports only).
- `description` (String) The filter for including checks in the report based on the description of a check.
- `newer_than_days` (Number) Include checks from the last N days (max 365). Example: 5 includes checks from the last 5 days.
- `older_than_days` (Number) Include checks older than N days (max 365). Example: 5 includes checks older than 5 days.
- `providers` (Set of String) Filter by cloud providers.
- `regions` (Set of String) Filter by cloud regions.
- `resource_id` (String) Filter by resource ID.
- `resource_search_mode` (String) Resource search mode. Allowed values: text, regex.
- `resource_types` (Set of String) Filter by resource types (e.g., 'kms-key', 's3-bucket').
- `risk_levels` (Set of String) Filter by risk levels.
- `rule_ids` (Set of String) Filter by specific rule IDs (e.g., 'S3-001', 'IAM-045').
- `services` (Set of String) Filter by cloud services.
- `statuses` (Set of String) Filter by check statuses. Allowed values: SUCCESS, FAILURE.
- `suppressed` (Boolean) Whether to include suppressed or regular checks only. If not provided, both suppressed and unsuppressed checks are included.
- `tags` (Set of String) Filter by tags.


<a id="nestedblock--schedule"></a>
### Nested Schema for `schedule`

Optional:

- `enabled` (Boolean) Whether the report is scheduled to run automatically. Defaults to false if not specified.
- `frequency` (String) Cron expression for schedule frequency. Format: '(day of month) (month) (day of week)'. Examples: '* * 2' (every Tuesday), '1 * *' (1st of every month). Required when enabled is true.
- `timezone` (String) Required when enabled is true. It's used as which timezone the report schedule is based on, when the attribute scheduled is true. If this attribute was provided, it must be string that is a valid value of timezone database name such as Australia/Sydney. Available timzezones https://en.wikipedia.org/wiki/List_of_tz_database_time_zones.

## Report Levels

Reports can be created at three different levels:

1. **Company-Level Report Configurations**
   - Aggregate data across ALL accounts in the company
   - Configuration: Omit both `account_id` and `group_id`
   - Only ADMIN users can create company-level report configurations.
   - Use `include_account_names = true` to show account names in reports
   - Use case: Executive reporting, company-wide compliance

2. **Group-Level Report Configurations**
   - Aggregate data from multiple accounts within a Cloud Risk Management group
   - Configuration: Specify Cloud Risk Management(CRM) `group_id` only (not `account_id`)
   - Only ADMIN users can create group-level report configurations.
   - Use `include_account_names = true` to show account breakdown
   - Use case: Team/department reporting, environment-specific reports

3. **Account-Level Report Configurations**
   - Focus on a specific Cloud Risk Management account
   - Configuration: Specify Cloud Risk Management(CRM) `account_id` only (not `group_id`)
   - Use case: Account-specific compliance, detailed security reviews

## Report Types

### GENERIC Reports

- Flexible security and compliance reporting
- Can filter by multiple compliance standards using `checks_filter.compliance_standards`
- All filtering options available
- Use cases: Security overview, cost optimization, multi-category analysis

### COMPLIANCE-STANDARD Reports

- Specific compliance framework reports
- Requires `applied_compliance_standard_id` (e.g., `NIST4`, `AWAF-2025`, `GCPWAF`)
- Use `controls_type` to specify scope:
  - `all` - All controls (default)
  - `withChecksOnly` - Only controls with checks
  - `noChecksOnly` - Only controls without checks (useful for gap analysis)
- Use cases: NIST 800-53 compliance, AWS Well-Architected Framework review

## Schedule Frequency Examples

The `frequency` field uses a 3-field cron format: `(day of month) (month) (day of week)`

Common patterns:

- `* * *` - Every day
- `* * 1` - Every Monday (1=Mon, 2=Tue, ..., 7=Sun)
- `* * 1,3,5` - Every Monday, Wednesday, and Friday
- `1 * *` - 1st of every month
- `1,15 * *` - 1st and 15th of every month
- `1 1,4,7,10 *` - 1st day of Jan, Apr, Jul, Oct (quarterly)
- `15 * *` - 15th of every month

For on-demand reports, omit the `schedule` block entirely.

## Import

Import is supported using the following syntax:

```shell
terraform import visionone_crm_report_config.example <report_config_id>
```
