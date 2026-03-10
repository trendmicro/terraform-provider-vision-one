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
