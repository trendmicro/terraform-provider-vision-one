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
