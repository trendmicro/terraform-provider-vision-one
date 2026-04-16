# Example: Comprehensive configuration with Azure CAM connector and CRM account data source
# This example demonstrates a complete workflow:
# 1. Create Azure CAM connector
# 2. Look up the CRM account ID using the Azure subscription ID
# 3. Configure scan settings for that account

# Create Azure CAM connector (simplified example)
resource "visionone_cam_connector_azure" "production" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "Trend Micro Vision One Production Connector"
  subscription_id           = "11111111-1111-1111-1111-111111111111"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "Production Azure subscription for cloud security monitoring"
  is_cam_cloud_asrm_enabled = true
}

# Look up the CRM account ID using Azure subscription ID
# The account must be created by the CAM connector first
data "visionone_crm_account" "production_azure" {
  depends_on = [visionone_cam_connector_azure.production]

  azure_subscription_id = "11111111-1111-1111-1111-111111111111"
}

# Configure comprehensive scan settings for the Azure account
resource "visionone_crm_account_scan_setting" "comprehensive" {
  depends_on = [data.visionone_crm_account.production_azure]

  # Use the CRM account ID from the data source
  account_id = data.visionone_crm_account.production_azure.id

  # Enable scanning
  enabled = true

  # Custom scan interval (in hours)
  # More frequent scans for production environments
  interval = 2

  # Exclude specific regions from scanning
  # Useful for regions not in use or dev/test regions
  disabled_regions = [
    "westus",       # Not used in production
    "northeurope",  # Development region
    "australiaeast" # Test region
  ]

  # Optional: Temporarily disable until a specific date/time
  # Leave empty for no automatic re-enable
  # Example: "2026-07-01T00:00:00Z" to disable until July 1st
  disabled_until_datetime = ""
}
