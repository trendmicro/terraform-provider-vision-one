# Example: Single GCP Project Integration

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

# Create a service account in a single GCP project with comprehensive configuration
resource "visionone_cam_service_account_integration" "single_project" {
  depends_on = [visionone_cam_iam_custom_role.cam_role, time_rotating.key_rotation]
  # Project where the service account will be created
  project_id = "my-gcp-project-id"

  # Service account details
  account_id   = "visionone-cam-sa"
  display_name = "Vision One CAM Service Account"
  description  = "Service account for Trend Micro Vision One Cloud Account Management"

  # Use predefined viewer role for read-only access
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]

  # Configure automatic key rotation every 90 days
  rotation_time = time_rotating.key_rotation.rotation_rfc3339

  # Optional: Ignore if service account already exists (useful for re-runs)
  create_ignore_already_exists = true
}

# ===== Outputs =====
output "service_account_email" {
  value       = try(visionone_cam_service_account_integration.single_project.service_account_email, "")
  description = "Email address of the service account"
}

output "service_account_unique_id" {
  value       = try(visionone_cam_service_account_integration.single_project.service_account_unique_id, "")
  description = "Unique numeric ID of the service account"
}

output "key_name" {
  value       = try(visionone_cam_service_account_integration.single_project.key_name, "")
  description = "Resource name of the service account key"
}

output "key_valid_after" {
  value       = try(visionone_cam_service_account_integration.single_project.valid_after, "")
  description = "Timestamp when the key becomes valid"
}

output "key_valid_before" {
  value       = try(visionone_cam_service_account_integration.single_project.valid_before, "")
  description = "Timestamp when the key expires"
}

output "private_key" {
  value       = try(visionone_cam_service_account_integration.single_project.private_key, "")
  sensitive   = true
  description = "Private key in JSON format (base64 encoded) - SENSITIVE"
}

# Example: Save private key to a file (use with caution in production)
# resource "local_file" "service_account_key" {
#   content         = base64decode(visionone_cam_service_account_integration.single_project.private_key)
#   filename        = "${path.module}/service-account-key.json"
#   file_permission = "0600"
# }
