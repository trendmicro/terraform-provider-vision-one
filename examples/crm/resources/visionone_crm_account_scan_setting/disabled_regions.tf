# Example: Disable scanning for specific regions
# This example shows how to exclude certain regions from scanning

resource "visionone_crm_account_scan_setting" "exclude_regions" {
  account_id = "aws-123456789012" # The CRM account ID
  enabled    = true

  # Disable scanning for test/development regions
  disabled_regions = [
    "us-west-1",
    "eu-central-1",
    "ap-southeast-2"
  ]
}
