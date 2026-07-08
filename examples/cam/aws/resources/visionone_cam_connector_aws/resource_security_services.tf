# AWS CAM connector with Server & Workload Protection integration
#
# ===== PREREQUISITES =====
# 1. An active Vision One Server & Workload Protection instance
# 2. The instance UUID (found in Vision One console)
# 3. AWS IAM role with the required permissions
#
# Note: server_workload_protection_regions is required when a workload
# entry in connected_security_services includes instance_ids.

terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

resource "visionone_cam_connector_aws" "cam_connector_aws" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Workload Protection"
  description      = "CAM connector with Server and Workload Protection enabled"
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
