# AWS CAM connector with features configuration

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
  name             = "CAM Connector with Features"
  description      = "CAM connector with specific feature and region configuration"
  is_crem_enabled  = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["us-east-1", "us-west-2", "eu-west-1"]
    }
  ]
}
