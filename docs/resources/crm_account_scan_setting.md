---
page_title: "visionone_crm_account_scan_setting Resource - visionone"
subcategory: "Cloud Risk Management"
description: |-
  Manages scan settings for a Cloud Risk Management account.
  Account scan settings control how and when cloud posture scans are performed. These settings are automatically created when an account is added and can be updated to customize scan behavior.
---

# visionone_crm_account_scan_setting (Resource)

Manages scan settings for a Cloud Risk Management account.

Account scan settings control how and when cloud posture scans are performed. These settings are automatically created when an account is added and can be updated to customize scan behavior.

## Example Usage

### Basic Account Scan Setting

```terraform
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
```

### Custom Scan Interval

```terraform
# Example: Custom scan interval configuration
# This example shows how to set a custom scan interval for more frequent scanning
# Note: interval must be between 1 and 12 hours

resource "visionone_crm_account_scan_setting" "frequent_scan" {
  account_id = "aws-123456789012" # The CRM account ID
  enabled    = true
  interval   = 6 # Scan every 6 hours (valid range: 1-12 hours)
}
```

### Disabled Regions

```terraform
# Example: Disable scanning for specific regions
# Disabled Regions is only applicable of AWS account, please don't use it for the account of the other provider
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
```

### Permanently Disabled Scanning

```terraform
# Example: Permanently disabled scanning
# This example shows how to permanently disable scanning for an account

resource "visionone_crm_account_scan_setting" "disabled" {
  account_id = "aws-123456789012" # The CRM account ID
  enabled    = false              # Disable scanning permanently

  # No disabled_until_datetime means it stays disabled indefinitely
}
```

### Temporarily Disable Scanning

```terraform
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
```

### Working With CAM

```terraform
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

  # Optional: Temporarily disable until a specific date/time
  # Leave empty for no automatic re-enable
  # Example: "2026-07-01T00:00:00Z" to disable until July 1st
  disabled_until_datetime = ""
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `account_id` (String) The CRM account ID for which to manage scan settings.

### Optional

- `disabled_regions` (List of String) List of cloud regions where scanning is disabled. Only applicable for AWS accounts. For other providers, please do not use this attribute.
- `disabled_until_datetime` (String) ISO 8601 datetime string indicating when scanning should be disabled until. After this time, scanning will automatically resume. Leave empty to not use this feature.
- `enabled` (Boolean) Whether scanning is enabled for this account.
- `interval` (Number) Scan interval in hours. Determines how frequently the account is scanned.

## Import

Import is supported using the following syntax:

```shell
terraform import visionone_crm_account_scan_setting.example <account_id>
```
