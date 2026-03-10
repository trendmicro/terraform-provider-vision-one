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