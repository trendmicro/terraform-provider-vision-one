resource "visionone_crm_account_scan_rules" "basic" {
  account_id = "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d" # Vision One Cloud Risk Management account UUID

  scan_rule {
    id         = "EC2-001"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"
  }

  scan_rule {
    id         = "RTM-002"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"

    extra_settings {
      name  = "ttl"
      type  = "ttl"
      value = 72
    }
  }

  scan_rule {
    id         = "SNS-002"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"

    exceptions {
      filter_tags  = ["ignore_this_tag"]
      resource_ids = []
    }

    extra_settings {
      name      = "friendlyAccounts"
      type      = "multiple-aws-account-values"
      value_set = ["123456789012"]
    }

    extra_settings {
      name      = "accountTags"
      type      = "tags"
      value_set = ["env_prod", "team_devops"]
    }
  }
}
