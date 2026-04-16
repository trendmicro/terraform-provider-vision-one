# Example: Basic account scan setting configuration with default values
# This example manages scan settings for a cloud account with default values

resource "visionone_crm_account_scan_setting" "basic" {
  account_id = "aws-123456789012" # The CRM account ID

  # Using default values:
  # - enabled: true
  # - interval: 1 hour
  # - disabled_regions: [] (empty list)
  # - disabled_until_datetime: "" (not set)
}

output "basic_scan_setting" {
  description = "Basic scan setting with default values"
  value = {
    account_id = visionone_crm_account_scan_setting.basic.account_id
    enabled    = visionone_crm_account_scan_setting.basic.enabled
    interval   = visionone_crm_account_scan_setting.basic.interval
  }
}
