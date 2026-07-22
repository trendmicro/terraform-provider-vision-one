# Example: GCP Organization Level Integration

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

# Create a custom IAM role at the organization level (optional but recommended for least privilege)
resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id      = "my-management-project"
  organization_id = "123456789012"
  title           = "Vision One CAM Custom Role - Org Level"
  description     = "Custom role for Vision One CAM across the entire organization"
}

# Org-level scan role for read-only discovery and scanning, granted once at the organization node.
# Bound at the org node via node_scan_roles below; all projects in the org inherit it.
resource "visionone_cam_gcp_scan_role" "scan_role" {
  project_id      = "my-management-project" # used for GCP authentication
  organization_id = "123456789012"
  role_id         = "trend_ai_auto_detect"
  title           = "Trend Vision One Auto-Detect Scan Role"
  description     = "Read-only discovery and scanning role bound at the organization node"
}

# Optional: Configure automatic key rotation every 90 days
resource "time_rotating" "key_rotation" {
  rotation_days = 90
}

# Create a service account with organization-level access
resource "visionone_cam_service_account_integration" "organization_level" {
  depends_on = [visionone_cam_iam_custom_role.cam_role, visionone_cam_gcp_scan_role.scan_role, time_rotating.key_rotation]

  # Central management project where the service account will be created
  central_management_project_id_in_org = "my-management-project"

  # Service account details
  account_id   = "visionone-cam-org-sa"
  display_name = "Vision One CAM Service Account - Organization Level"
  description  = "Service account for monitoring all projects in the organization"

  # roles/viewer is bound to all projects in the organization (sub-projects + primary project)
  roles = [
    "roles/viewer",
  ]

  # primary_project_roles are bound only to the primary project (where the service account lives)
  # This follows least-privilege: elevated permissions are not replicated to sub-projects
  primary_project_roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]

  # node_scan_roles are granted ONCE at the organization node for read-only discovery and scanning.
  # All projects in the organization, including projects created later, inherit these roles, so
  # new projects are covered without a per-project binding. roles/viewer is added here because
  # a basic role cannot be inlined into the scan custom role.
  node_scan_roles = [
    visionone_cam_gcp_scan_role.scan_role.name,
    "roles/viewer",
  ]

  # Optional: Exclude specific projects from monitoring
  exclude_projects = [
    "test-project",
    "sandbox-project",
  ]

  # Optional: Exclude free trial projects
  exclude_free_trial_projects = true

  # Optional: Key rotation
  rotation_time = time_rotating.key_rotation.rotation_rfc3339

  # Optional: Ignore if service account already exists
  create_ignore_already_exists = true
}

# ===== Outputs =====
output "service_account_email" {
  value       = try(visionone_cam_service_account_integration.organization_level.service_account_email, "")
  description = "Email address of the service account"
}

output "service_account_unique_id" {
  value       = try(visionone_cam_service_account_integration.organization_level.service_account_unique_id, "")
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
  value       = visionone_cam_service_account_integration.organization_level.bound_projects != null ? visionone_cam_service_account_integration.organization_level.bound_projects : null
  description = "List of project IDs where IAM bindings were created (only applicable in multi-project mode)"
}

output "bound_project_numbers" {
  value       = visionone_cam_service_account_integration.organization_level.bound_project_numbers != null ? visionone_cam_service_account_integration.organization_level.bound_project_numbers : null
  description = "List of project numbers corresponding to bound_projects, in the same order"
}

output "bound_projects_count" {
  value       = visionone_cam_service_account_integration.organization_level.bound_projects != null ? length(visionone_cam_service_account_integration.organization_level.bound_projects) : null
  description = "Number of projects with IAM bindings (only applicable in multi-project mode)"
}

output "key_name" {
  value       = try(visionone_cam_service_account_integration.organization_level.key_name, "")
  description = "Resource name of the service account key"
}

output "key_valid_after" {
  value       = try(visionone_cam_service_account_integration.organization_level.valid_after, "")
  description = "Timestamp when the key becomes valid"
}

output "key_valid_before" {
  value       = try(visionone_cam_service_account_integration.organization_level.valid_before, "")
  description = "Timestamp when the key expires"
}

output "private_key" {
  value       = try(visionone_cam_service_account_integration.organization_level.private_key, "")
  sensitive   = true
  description = "Private key in JSON format (base64 encoded) - SENSITIVE"
}

# Example: Save private key to a file (use with caution in production)
# resource "local_file" "service_account_key" {
#   content         = base64decode(visionone_cam_service_account_integration.organization_level.private_key)
#   filename        = "${path.module}/service-account-key.json"
#   file_permission = "0600"
# }
