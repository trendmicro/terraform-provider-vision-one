terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your_vision_one_api_key>"
  regional_fqdn = "<regional_fqdn>" # e.g., "https://api.xdr.trendmicro.com"
}

data "visionone_cam_connect_aws_accounts" "cam_connect_aws_accounts" {
  top   = 50        # Optional: limit the number of results, e.g., 25, 50, 100, 500, 1000, 5000
  state = "managed" # Optional: filter by state, e.g., "managed", "outdated", "failed"
}


output "cam_connect_aws_accounts" {
  value = jsondecode(jsonencode(data.visionone_cam_connect_aws_accounts.cam_connect_aws_accounts.cloud_accounts))
}
