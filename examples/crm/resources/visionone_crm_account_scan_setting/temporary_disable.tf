# Example: Temporarily disable scanning until a specific date
# This example shows how to disable scanning temporarily for maintenance or migration
# Note: disabled_until_datetime must be between 1 and 72 hours from the current time

resource "visionone_crm_account_scan_setting" "temporary_disable" {
  account_id = "aws-123456789012" # The CRM account ID
  enabled    = false              # Disable scanning

  # Automatically re-enable scanning after this date/time
  # Must be between 1 hour and 72 hours from now (example: 48 hours from now)
  disabled_until_datetime = timeadd(timestamp(), "48h")
}
