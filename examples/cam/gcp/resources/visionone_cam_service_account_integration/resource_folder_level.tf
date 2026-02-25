# Example: GCP Folder Level Integration

terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "your-vision-one-api-key"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id  = "my-gcp-project-id"
  title       = "Vision One CAM Service Account Role"
  description = "Custom role for Vision One CAM service account in central management project"
}

# Configure automatic key rotation every 90 days
resource "time_rotating" "key_rotation" {
  rotation_days = 90
}

# Create a service account with folder-level access
resource "visionone_cam_service_account_integration" "folder_level" {
  depends_on = [visionone_cam_iam_custom_role.cam_role, time_rotating.key_rotation]

  # Central management project where the service account will be created
  central_management_project_id_in_folder = "my-management-project"

  # Service account details
  account_id   = "visionone-cam-folder-sa"
  display_name = "Vision One CAM Service Account - Folder Level"
  description  = "Service account for monitoring all projects in the folder"

  # Use both predefined viewer role and custom role
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]

  # Optional: Exclude specific projects from monitoring
  exclude_projects = [
    "project-to-exclude-1",
    "project-to-exclude-2",
  ]

  # Optional: Exclude free trial projects
  exclude_free_trial_projects = true

  rotation_time = time_rotating.key_rotation.rotation_rfc3339

  # Optional: Ignore if service account already exists
  create_ignore_already_exists = true
}

# ===== Outputs =====
output "service_account_email" {
  value       = try(visionone_cam_service_account_integration.folder_level.service_account_email, "")
  description = "Email address of the service account"
}

output "service_account_unique_id" {
  value       = try(visionone_cam_service_account_integration.folder_level.service_account_unique_id, "")
  description = "Unique numeric ID of the service account"
}

output "role_name" {
  value       = try(visionone_cam_iam_custom_role.cam_role.name, "")
  description = "Full resource name of the custom IAM role"
}

output "role_id" {
  value       = try(visionone_cam_iam_custom_role.cam_role.role_id, "")
  description = "Role ID of the custom IAM role"
}

output "bound_projects" {
  value       = visionone_cam_service_account_integration.folder_level.bound_projects != null ? visionone_cam_service_account_integration.folder_level.bound_projects : null
  description = "List of project IDs where IAM bindings were created (only applicable in multi-project mode)"
}

output "bound_project_numbers" {
  value       = visionone_cam_service_account_integration.folder_level.bound_project_numbers != null ? visionone_cam_service_account_integration.folder_level.bound_project_numbers : null
  description = "List of project numbers corresponding to bound_projects, in the same order"
}

output "bound_projects_count" {
  value       = visionone_cam_service_account_integration.folder_level.bound_projects != null ? length(visionone_cam_service_account_integration.folder_level.bound_projects) : null
  description = "Number of projects with IAM bindings (only applicable in multi-project mode)"
}

output "key_name" {
  value       = try(visionone_cam_service_account_integration.folder_level.key_name, "")
  description = "Resource name of the service account key"
}

output "key_valid_after" {
  value       = try(visionone_cam_service_account_integration.folder_level.valid_after, "")
  description = "Timestamp when the key becomes valid"
}

output "key_valid_before" {
  value       = try(visionone_cam_service_account_integration.folder_level.valid_before, "")
  description = "Timestamp when the key expires"
}

output "private_key" {
  value       = try(visionone_cam_service_account_integration.folder_level.private_key, "")
  sensitive   = true
  description = "Private key in JSON format (base64 encoded) - SENSITIVE"
}

# Example: Save private key to a file (use with caution in production)
# resource "local_file" "service_account_key" {
#   content         = base64decode(visionone_cam_service_account_integration.folder_level.private_key)
#   filename        = "${path.module}/service-account-key.json"
#   file_permission = "0600"
# }
