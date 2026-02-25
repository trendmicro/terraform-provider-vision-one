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
# This example demonstrates how to create the same tag key across multiple projects with other CAM resources using Terraform's `for_each` feature.

resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id      = "org-level-project-id"
  organization_id = "organization-id"
  title           = "Vision One CAM Service Account Role Folder Scope"
  description     = "Custom role for Vision One CAM service account in central management project"
}

resource "time_rotating" "sa_key_rotation" {
  rotation_days = 90
}

resource "visionone_cam_service_account_integration" "comprehensive" {
  depends_on                           = [visionone_cam_iam_custom_role.cam_role, time_rotating.sa_key_rotation]
  central_management_project_id_in_org = "organization-id"
  account_id                           = "visionone-cam-sa"
  display_name                         = "Vision One CAM Service Account"
  description                          = "Production service account for Trend Micro Vision One Cloud Account Management with multi-project access"
  create_ignore_already_exists         = true
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]
  exclude_free_trial_projects = true
  exclude_projects            = []
  rotation_time               = time_rotating.sa_key_rotation.rotation_rfc3339
}

resource "visionone_cam_enable_api_services" "comprehensive_api_enablement" {
  for_each   = toset(visionone_cam_service_account_integration.comprehensive.bound_projects)
  project_id = each.value
}

resource "visionone_cam_tag_key" "cam_version_key" {
  for_each    = toset(visionone_cam_service_account_integration.comprehensive.bound_projects)
  short_name  = "vision-one-deployment-version"
  parent      = "projects/${each.value}"
  description = "Version tag key for CAM template identification"
}