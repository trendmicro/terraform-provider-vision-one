# Example: Custom scan interval configuration
# This example shows how to set a custom scan interval for more frequent scanning
# Note: interval must be between 1 and 12 hours

resource "visionone_crm_account_scan_setting" "frequent_scan" {
  account_id = "aws-123456789012" # The CRM account ID
  enabled    = true
  interval   = 6 # Scan every 6 hours (valid range: 1-12 hours)
}
