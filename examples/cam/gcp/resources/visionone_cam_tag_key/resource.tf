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

# Basic tag key at project level
# This tag key will be used by CAM to identify template versions
# The specific tag name is "vision-one-deployment-version" that the system will look for when CAM is deployed in customer's environment
resource "visionone_cam_tag_key" "cam_version_key" {
  short_name  = "vision-one-deployment-version"
  parent      = "projects/your-gcp-project-id"
  description = "Version tag key for CAM template identification"
}