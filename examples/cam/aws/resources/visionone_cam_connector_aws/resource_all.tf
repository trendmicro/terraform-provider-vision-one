# Comprehensive AWS CAM connector examples
#
# This file demonstrates all supported configurations for the
# visionone_cam_connector_aws resource.

# ===== Example 1: Multiple accounts using for_each =====
variable "aws_account_ids" {
  type    = set(string)
  default = ["123456789012", "210987654321"]
}

resource "visionone_cam_connector_aws" "cam_connector_multi" {
  for_each = var.aws_account_ids

  cloud_account_id = each.value
  role_arn         = "arn:aws:iam::${each.value}:role/VisionOneRole"
  name             = "Vision One CAM AWS Connector - ${each.value}"
  description      = "CAM connector for AWS account ${each.value}"
  is_crem_enabled  = true
}

# ===== Example 2: Connector with features =====
resource "visionone_cam_connector_aws" "cam_connector_with_features" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Features"
  description      = "CAM connector with feature configuration"
  is_crem_enabled  = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["us-east-1", "us-west-2"]
    }
  ]
}

# ===== Example 3: Connector with features and config file path =====
# Note: features_config_file_path requires features to also be set
resource "visionone_cam_connector_aws" "cam_connector_with_features_config" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Features Config File"
  description      = "CAM connector with features configuration file"
  is_crem_enabled  = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["us-east-1"]
    }
  ]
  features_config_file_path = "/path/to/features-config.json"
}

# ===== Example 4: Connector with Connected Security Services =====
# Note: server_workload_protection_regions is required when workload instance_ids are provided
resource "visionone_cam_connector_aws" "cam_connector_with_security_services" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Security Services"
  description      = "CAM connector with connected security services for workload protection"
  is_crem_enabled  = true

  connected_security_services = [
    {
      name         = "workload"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
      regions      = ["us-east-1", "us-west-2"]
    }
  ]

  server_workload_protection_regions = ["us-east-1", "us-west-2"]
}

# ===== Example 5: Connector with AWS Organization =====
# Note: is_aws_org_mgmt_account and organization_excluded_accounts require organization_id
resource "visionone_cam_connector_aws" "cam_connector_org" {
  cloud_account_id      = "123456789012"
  role_arn              = "arn:aws:iam::123456789012:role/VisionOneRole"
  name                  = "CAM Connector for AWS Organization"
  description           = "CAM connector for AWS Organization management account"
  is_crem_enabled       = true
  is_aws_org_mgmt_account = true

  # organization_id: bare AWS Organization ID (o-<alphanum10-32>), OU ID, or root ID
  organization_id = "o-aa111bb2cc"

  organization_excluded_accounts = ["999999999999"]
}

# ===== Example 6: Connector with custom tags =====
resource "visionone_cam_connector_aws" "cam_connector_with_tags" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Custom Tags"
  description      = "CAM connector with custom tags for resource identification"
  is_crem_enabled  = true

  custom_tags = {
    environment = "production"
    team        = "cam"
    cost-center = "12345"
  }
}

# ===== Example 7: Connector with prevent_destroy disabled =====
# By default prevent_destroy=true (destroy will NOT call the CAM DELETE API).
# Set to false to allow the account to be removed from CAM on terraform destroy.
resource "visionone_cam_connector_aws" "cam_connector_allow_destroy" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector (destroyable)"
  description      = "CAM connector that will be removed from CAM on destroy"
  is_crem_enabled  = false
  prevent_destroy  = false
}
