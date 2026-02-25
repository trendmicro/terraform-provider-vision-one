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

# Multiple projects tag key creation example
# This example demonstrates how to create the same tag key across multiple projects using Terraform's `for_each` feature.
locals {
  projects = [
    "project-id-A",
    "project-id-B",
  ]
}
resource "visionone_cam_tag_key" "cam_version_key" {
  for_each    = toset(local.projects)
  short_name  = "vision-one-deployment-version"
  parent      = "projects/${each.value}"
  description = "Version tag key for CAM template identification"
}