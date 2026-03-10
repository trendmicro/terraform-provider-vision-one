---
page_title: "visionone_crm_communication_configuration Resource - visionone"
subcategory: "Cloud Risk Management"
description: |-
  Manages a Cloud Risk Management Communication Configuration.
  Exactly one channel configuration must be specified: email_configuration, sms_configuration, ms_teams_configuration, slack_configuration, sns_configuration, pagerduty_configuration, webhook_configuration, jira_configuration, zendesk_configuration, or servicenow_configuration.
---

# visionone_crm_communication_configuration (Resource)

Manages a Cloud Risk Management Communication Configuration.

Exactly one channel configuration must be specified: `email_configuration`, `sms_configuration`, `ms_teams_configuration`, `slack_configuration`, `sns_configuration`, `pagerduty_configuration`, `webhook_configuration`, `jira_configuration`, `zendesk_configuration`, or `servicenow_configuration`.

## Example Usage

### Email Notification

```terraform
resource "visionone_crm_communication_configuration" "email" {
  enabled       = true
  channel_label = "Security Alerts"

  email_configuration = {
    user_ids = ["identifier-id-123#company-id-456"]
  }

  checks_filter = {
    regions    = ["us-east-1", "us-west-2"]
    categories = ["security", "reliability"]
  }
}
```

**Note:** The `user_ids` format is `{identifierId}#{companyId}`. Use the `/v3.0/iam/accounts` API to retrieve identifier IDs. See the API specification in Vision One console under **Workflow and Automation > API Automation Center**.

**Important:** Users must have visited Cloud Risk Management in the Vision One console at least once before they can be added to email notifications. This provisions the user in the CRM system.

### SMS Notification

```terraform
resource "visionone_crm_communication_configuration" "sms" {
  enabled       = true
  channel_label = "Critical Alerts"

  sms_configuration = {
    user_ids = ["identifier-id-456#company-id-789"]
  }

  checks_filter = {
    services    = ["S3", "IAM", "EC2"]
    risk_levels = ["EXTREME", "VERY_HIGH"]
  }
}
```

**Note:** The `user_ids` format is `{identifierId}#{companyId}`. Use the `/v3.0/iam/accounts` API to retrieve identifier IDs. See the API specification in Vision One console under **Workflow and Automation > Automation Center**.

**Important:** Users must configure mobile notifications in Cloud Risk Management settings via the
Vision One console before they can receive SMS notifications.

### Microsoft Teams Notification

```terraform
resource "visionone_crm_communication_configuration" "ms_teams" {
  enabled       = true
  channel_label = "Cloud Security Channel"

  ms_teams_configuration = {
    url                   = "https://outlook.office.com/webhook/your-webhook-url"
    include_introduced_by = true
    include_resource      = true
    include_tags          = true
    include_extra_data    = false
  }

  checks_filter = {
    rule_ids = ["EC2-001", "S3-002", "IAM-003"]
    tags     = ["production", "pci-dss"]
  }
}
```

### Slack Notification

```terraform
resource "visionone_crm_communication_configuration" "slack" {
  enabled       = true
  channel_label = "Compliance Alerts"

  slack_configuration = {
    url                   = "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXX"
    channel               = "#compliance-alerts"
    include_introduced_by = true
    include_resource      = true
    include_tags          = true
    include_extra_data    = true
  }

  checks_filter = {
    compliance_standard_ids = ["AWAF-2025", "CIS-V8", "PCI"]
  }
}
```

### Amazon SNS Notification

```terraform
resource "visionone_crm_communication_configuration" "amazon_sns" {
  enabled       = true
  channel_label = "AWS Event Stream"
  manual        = true

  sns_configuration = {
    arn = "arn:aws:sns:us-east-1:123456789012:cloud-security-events"
  }

  checks_filter = {
    statuses = ["SUCCESS", "FAILURE"]
    regions  = ["us-east-1", "eu-west-1", "ap-southeast-2"]
  }
}
```

### PagerDuty Notification

```terraform
resource "visionone_crm_communication_configuration" "pagerduty" {
  enabled       = true
  channel_label = "On-Call Incidents"

  pagerduty_configuration = {
    service_name = "https://my-pagerduty.pagerduty.com"
    service_key  = "your-pagerduty-integration-key"
  }

  checks_filter = {
    categories  = ["security", "reliability", "operational-excellence"]
    risk_levels = ["EXTREME"]
  }
}
```

### Webhook Notification

```terraform
# Account-level webhook configuration
data "visionone_crm_account" "aws_production" {
  aws_account_id = "123456789012"
}

resource "visionone_crm_communication_configuration" "webhook" {
  enabled       = true
  channel_label = "SIEM Integration"
  account_id    = data.visionone_crm_account.aws_production.id

  webhook_configuration = {
    url            = "https://siem.example.com/api/v1/events"
    security_token = "your-secret-token"
    headers = [
      {
        key   = "Authorization"
        value = "Bearer your-api-token"
      },
      {
        key   = "Content-Type"
        value = "application/json"
      }
    ]
  }

  checks_filter = {
    statuses = ["FAILURE"]
    services = ["Lambda", "RDS", "CloudTrail"]
  }
}
```

**Note:** When importing a webhook configuration, the `headers` and `security_token` attributes are not retrieved from the API. You must define these in your Terraform configuration to match what exists on the server.

### Jira Notification

```terraform
# Account-level Jira configuration
data "visionone_crm_account" "aws_workload" {
  aws_account_id = "987654321098"
}

resource "visionone_crm_communication_configuration" "jira" {
  enabled       = true
  channel_label = "Compliance Tickets"
  manual        = true
  account_id    = data.visionone_crm_account.aws_workload.id

  jira_configuration = {
    url         = "https://your-domain.atlassian.net"
    username    = "your-email@example.com"
    api_token   = "your-jira-api-token"
    project     = "COMPLY"
    type        = "Task"
    assignee_id = "user-account-id"
    priority    = "Medium"
  }

  checks_filter = {
    compliance_standard_ids = ["AWAF-2025", "SOC2"]
    tags                    = ["compliance-required", "audit-scope"]
  }
}
```

**Note:** When importing a Jira configuration, the `api_token` attribute is not retrieved from the API. You must define the API token in your Terraform configuration to match what exists on the server.

### Zendesk Notification

```terraform
resource "visionone_crm_communication_configuration" "zendesk" {
  enabled       = true
  channel_label = "Customer Support Tickets"
  manual        = true

  zendesk_configuration = {
    url         = "https://your-subdomain.zendesk.com"
    username    = "agent@example.com"
    api_token   = "your-zendesk-api-token"
    type        = "incident"
    priority    = "high"
    group_id    = 12345678
    assignee_id = 87654321
  }

  checks_filter = {
    risk_levels = ["HIGH", "VERY_HIGH", "EXTREME"]
  }
}
```

**Note:** Either `password` or `api_token` must be provided, but not both. When importing a Zendesk configuration, the `password` and `api_token` attributes are not retrieved from the API. You must define the credential in your Terraform configuration to match what exists on the server.

### ServiceNow Notification

```terraform
# Account-level ServiceNow configuration
data "visionone_crm_account" "azure_subscription" {
  azure_subscription_id = "12345678-1234-1234-1234-123456789012"
}

resource "visionone_crm_communication_configuration" "servicenow" {
  enabled       = true
  channel_label = "Azure Incident Tickets"
  manual        = true
  account_id    = data.visionone_crm_account.azure_subscription.id

  servicenow_configuration = {
    type     = "incident"
    url      = "https://your-instance.service-now.com"
    username = "admin"
    password = "your-password"

    dictionary_overrides = [
      {
        trigger = "creation"
        key_value_pairs = [
          {
            key   = "impact"
            value = "1"
          },
          {
            key   = "urgency"
            value = "1"
          },
          {
            key   = "priority"
            value = "1"
          },
          {
            key   = "category"
            value = "Security"
          },
          {
            key   = "subcategory"
            value = "Cloud Misconfiguration"
          }
        ]
      },
      {
        trigger = "resolution"
        key_value_pairs = [
          {
            key   = "close_code"
            value = "Solved (Permanently)"
          },
          {
            key   = "close_notes"
            value = "Issue resolved via Cloud Risk Management remediation."
          }
        ]
      }
    ]
  }

  checks_filter = {
    risk_levels = ["EXTREME", "VERY_HIGH"]
    categories  = ["security"]
  }
}
```

**Note:** When importing a ServiceNow configuration, the `password` attribute is not retrieved from the API. You must define the password in your Terraform configuration to match what exists on the server.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `enabled` (Boolean) Whether the communication configuration is enabled

### Optional

- `account_id` (String) The CRM account ID. If provided, the configuration applies to that account only. If omitted, it applies globally to all accounts (company level).
- `channel_label` (String) A label to distinguish between multiple instances of the same channel type.
- `checks_filter` (Attributes) Filter to apply to checks for this communication configuration. (see [below for nested schema](#nestedatt--checks_filter))
- `email_configuration` (Attributes) Email channel configuration. (see [below for nested schema](#nestedatt--email_configuration))
- `jira_configuration` (Attributes) Jira channel configuration for creating tickets. (see [below for nested schema](#nestedatt--jira_configuration))
- `manual` (Boolean) Whether to use manual mode. Available only for SNS and ticketing channels (ServiceNow, Jira, Zendesk).
- `ms_teams_configuration` (Attributes) MS Teams channel configuration. (see [below for nested schema](#nestedatt--ms_teams_configuration))
- `pagerduty_configuration` (Attributes) PagerDuty channel configuration. (see [below for nested schema](#nestedatt--pagerduty_configuration))
- `servicenow_configuration` (Attributes) ServiceNow channel configuration for creating tickets. (see [below for nested schema](#nestedatt--servicenow_configuration))
- `slack_configuration` (Attributes) Slack channel configuration. (see [below for nested schema](#nestedatt--slack_configuration))
- `sms_configuration` (Attributes) SMS channel configuration. (see [below for nested schema](#nestedatt--sms_configuration))
- `sns_configuration` (Attributes) Amazon SNS channel configuration. (see [below for nested schema](#nestedatt--sns_configuration))
- `webhook_configuration` (Attributes) Webhook channel configuration. (see [below for nested schema](#nestedatt--webhook_configuration))
- `zendesk_configuration` (Attributes) Zendesk channel configuration for creating tickets. Either `password` or `api_token` must be provided, but not both. (see [below for nested schema](#nestedatt--zendesk_configuration))

### Read-Only

- `channel_type` (String) The channel type. Automatically set based on the channel configuration block.
- `id` (String) The unique ID of the communication configuration.
- `level` (String) The communication configuration level (company or account).

<a id="nestedatt--checks_filter"></a>
### Nested Schema for `checks_filter`

Optional:

- `categories` (Set of String) Filter by category.
- `compliance_standard_ids` (Set of String) Filter by compliance standard ID.
- `regions` (Set of String) Filter by cloud region.
- `risk_levels` (Set of String) Filter by risk level (LOW, MEDIUM, HIGH, VERY_HIGH, EXTREME).
- `rule_ids` (Set of String) Filter by specific rule ID.
- `services` (Set of String) Filter by cloud service.
- `statuses` (Set of String) Filter by check statuses (SUCCESS, FAILURE). Available only for webhook and sns communication configurations.
- `tags` (Set of String) Filter by tag.


<a id="nestedatt--email_configuration"></a>
### Nested Schema for `email_configuration`

Required:

- `user_ids` (Set of String) List of user identifiers to receive notifications. Format: `{identifierId}#{companyId}`.


<a id="nestedatt--jira_configuration"></a>
### Nested Schema for `jira_configuration`

Required:

- `api_token` (String, Sensitive) The Jira API token.
- `project` (String) The Jira project key.
- `type` (String) The Jira issue type (e.g., Bug, Task, Story).
- `url` (String) The Jira URL.
- `username` (String) The Jira username.

Optional:

- `assignee_id` (String) The Jira assignee ID.
- `priority` (String) The Jira priority (e.g., High, Medium, Low).


<a id="nestedatt--ms_teams_configuration"></a>
### Nested Schema for `ms_teams_configuration`

Required:

- `url` (String, Sensitive) The Microsoft Teams incoming webhook URL.

Optional:

- `include_extra_data` (Boolean) Whether to include extra data associated with a check in the notification.
- `include_introduced_by` (Boolean) Whether to include information about what introduced the check in the notification.
- `include_resource` (Boolean) Whether to include information about the resource in the notification.
- `include_tags` (Boolean) Whether to include check tags in the notification.


<a id="nestedatt--pagerduty_configuration"></a>
### Nested Schema for `pagerduty_configuration`

Required:

- `service_key` (String, Sensitive) The PagerDuty service integration key.
- `service_name` (String) The PagerDuty service name.


<a id="nestedatt--servicenow_configuration"></a>
### Nested Schema for `servicenow_configuration`

Required:

- `password` (String, Sensitive) The ServiceNow password.
- `type` (String) The ServiceNow ticket type. Must be `problem`, `incident`, or `configurationTestResult`.
- `url` (String) The ServiceNow URL.
- `username` (String) The ServiceNow username.

Optional:

- `assignee` (String) The assignee of the ServiceNow ticket.
- `dictionary_overrides` (Attributes List) JSON payload overriding ticket creation POST body and resolution PATCH body. (see [below for nested schema](#nestedatt--servicenow_configuration--dictionary_overrides))
- `impact` (String) The impact of the ServiceNow ticket.
- `urgency` (String) The urgency of the ServiceNow ticket.

<a id="nestedatt--servicenow_configuration--dictionary_overrides"></a>
### Nested Schema for `servicenow_configuration.dictionary_overrides`

Required:

- `trigger` (String) The override action type. Must be `creation` or `resolution`.

Optional:

- `key_value_pairs` (Attributes List) Key value pairs of overrides. (see [below for nested schema](#nestedatt--servicenow_configuration--dictionary_overrides--key_value_pairs))

<a id="nestedatt--servicenow_configuration--dictionary_overrides--key_value_pairs"></a>
### Nested Schema for `servicenow_configuration.dictionary_overrides.key_value_pairs`

Required:

- `key` (String) The override key.
- `value` (String) The override value.




<a id="nestedatt--slack_configuration"></a>
### Nested Schema for `slack_configuration`

Required:

- `channel` (String) The Slack channel to post to (e.g., #security-alerts).
- `url` (String, Sensitive) The Slack incoming webhook URL.

Optional:

- `include_extra_data` (Boolean) Whether to include extra data associated with a check in the notification. Defaults to false.
- `include_introduced_by` (Boolean) Whether to include information about what introduced the check in the notification. Defaults to false.
- `include_resource` (Boolean) Whether to include information about the resource in the notification. Defaults to false.
- `include_tags` (Boolean) Whether to include check tags in the notification. Defaults to false.


<a id="nestedatt--sms_configuration"></a>
### Nested Schema for `sms_configuration`

Required:

- `user_ids` (Set of String) List of user identifiers to receive notifications. Format: `{identifierId}#{companyId}`.


<a id="nestedatt--sns_configuration"></a>
### Nested Schema for `sns_configuration`

Required:

- `arn` (String) The Amazon SNS topic ARN.


<a id="nestedatt--webhook_configuration"></a>
### Nested Schema for `webhook_configuration`

Required:

- `url` (String, Sensitive) The webhook URL to send notifications to.

Optional:

- `headers` (Attributes List) Custom headers to include in the webhook request. (see [below for nested schema](#nestedatt--webhook_configuration--headers))
- `security_token` (String, Sensitive) Secret token for HMAC-SHA256 webhook payload signing.

<a id="nestedatt--webhook_configuration--headers"></a>
### Nested Schema for `webhook_configuration.headers`

Required:

- `key` (String) The header name.
- `value` (String, Sensitive) The header value.



<a id="nestedatt--zendesk_configuration"></a>
### Nested Schema for `zendesk_configuration`

Required:

- `url` (String) The Zendesk URL.
- `username` (String) The Zendesk username (agent email).

Optional:

- `api_token` (String, Sensitive) The Zendesk API token. Either `password` or `api_token` must be provided.
- `assignee_id` (Number) The Zendesk assignee ID.
- `group_id` (Number) The Zendesk group ID.
- `password` (String, Sensitive) The Zendesk password. Either `password` or `api_token` must be provided.
- `priority` (String) The Zendesk ticket priority.
- `type` (String) The Zendesk ticket type.

## Import

```shell
terraform import visionone_crm_communication_configuration.example {configuration_id}
```