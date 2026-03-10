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