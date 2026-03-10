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
