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