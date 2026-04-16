# Example: Permanently disabled scanning
# This example shows how to permanently disable scanning for an account

resource "visionone_crm_account_scan_setting" "disabled" {
  account_id = "aws-123456789012" # The CRM account ID
  enabled    = false              # Disable scanning permanently

  # No disabled_until_datetime means it stays disabled indefinitely
}
