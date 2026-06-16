resource "visionone_crm_profile" "with_multiple_settings" {
  name        = "crm-profile-multiple-settings"
  description = "Profile with multiple setting types"

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
