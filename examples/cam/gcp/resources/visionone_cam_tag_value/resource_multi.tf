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

# Local variables for dynamic values
locals {
  date = formatdate("YYYY-MM-DD", timestamp())
}

# Create the version tag key
# This tag key is used by CAM to identify the template version that customers use for their environment
# The specific tag name is "vision-one-deployment-version" that the system will look for when CAM is deployed in customer's environment
resource "visionone_cam_tag_key" "cam_version_key" {
  short_name  = "vision-one-deployment-version"
  parent      = "projects/your-gcp-project-id"
  description = "Version tag key for CAM template identification"
}

# Create the version tag value
# NOTE: This tag value is used by CAM to identify the template version that customers use for their environment
# Template version can be retrieved from the GCP feature list API
# API endpoint: beta/cam/gcpProjects/features
# Portal: https://portal.xdr.trendmicro.com/index.html#/admin/automation_center
resource "visionone_cam_tag_value" "cam_version_value" {
  short_name  = base64encode(jsonencode({ "cloud-account-management" = "3.0.2047" }))
  parent      = visionone_cam_tag_key.cam_version_key.name
  description = "Created at ${local.date}"
}

output "tag_key_name" {
  description = "The resource name of the tag key (e.g., tagKeys/281477969039986)"
  value       = visionone_cam_tag_key.cam_version_key.name
}

output "tag_value_name" {
  description = "The resource name of the tag value (e.g., tagValues/987654321)"
  value       = visionone_cam_tag_value.cam_version_value.name
}
