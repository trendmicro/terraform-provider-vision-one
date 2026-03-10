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
