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

# Basic example with minimal configuration
# Uses default title, description, and core permissions
# When permissions are not specified, the role will include default core permissions
# required for Vision One Cloud Account Management
resource "visionone_cam_iam_custom_role" "basic" {
  project_id = "your-gcp-project-id"
}
