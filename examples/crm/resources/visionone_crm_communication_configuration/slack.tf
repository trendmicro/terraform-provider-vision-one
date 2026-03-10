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
