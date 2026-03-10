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
