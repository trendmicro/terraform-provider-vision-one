terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
    google = {
      source = "hashicorp/google"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# ==============================================================================
# Folder-level integration - Enable API services for multiple projects
# ==============================================================================
# This example shows both automatic detection (using bound_projects) and
# manual project specification approaches. See the documentation for details
# on when to use each approach.
# ==============================================================================

# Approach 1: Automatic detection using bound_projects from service account integration
resource "visionone_cam_service_account_integration" "folder_level" {
  account_id                              = "vision-one-cam-sa"
  central_management_project_id_in_folder = "your-folder-id"
}

# Enable API services for each project discovered in the folder
resource "visionone_cam_enable_api_services" "folder_projects" {
  for_each = toset(visionone_cam_service_account_integration.folder_level.bound_projects)

  project_id = each.value
}

# ==============================================================================
# Approach 2: Manual project list (alternative)
# ==============================================================================

# locals {
#   folder_project_ids = [
#     "project-1-id",
#     "project-2-id",
#     "project-3-id",
#   ]
# }
#
# resource "visionone_cam_enable_api_services" "folder_projects_manual" {
#   for_each = toset(local.folder_project_ids)
#
#   project_id = each.value
# }
