# Basic AWS connector example
#
# ===== WHEN TO USE THIS EXAMPLE =====
# Use this when you have a single AWS account to connect to Trend Micro Vision One CAM.
#
# ===== PREREQUISITES =====
# 1. Create an AWS IAM role and trust relationship for Vision One
# 2. Note the Role ARN (e.g., arn:aws:iam::123456789012:role/VisionOneRole)
# 3. Your 12-digit AWS account ID

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
  name             = "Trend Micro Vision One CAM AWS Connector"
  description      = "This is a CAM connector created by Terraform Provider for Vision One"
  is_crem_enabled  = true
}
