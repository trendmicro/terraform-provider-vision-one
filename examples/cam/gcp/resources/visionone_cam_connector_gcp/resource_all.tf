# Comprehensive example - All GCP CAM features combined
#
# ===== WHEN TO USE THIS EXAMPLE =====
# Use this example when you need to:
# - Connect MULTIPLE GCP projects under an organization to Vision One CAM
# - Automate service account creation and key rotation
# - Have Vision One manage all projects discovered in your GCP organization
#
# For SINGLE PROJECT setup, see the "Basic example" section above - it's simpler
# and requires only a pre-existing service account key file.
#
# ===== PREREQUISITES =====
# 1. GCP Organization Admin or Project Owner permissions
# 2. Vision One API key with CAM permissions
# 3. A "central management project" in GCP where the service account will be created
# 4. Organization ID (found in GCP Console > IAM & Admin > Settings)
#
# ===== WHAT THIS EXAMPLE CREATES =====
# - Custom IAM role at organization level
# - Service account with organization-wide access
# - Automatic 90-day key rotation
# - API services enablement for all discovered projects
# - Version tracking tags for CAM template identification
# - GCP connectors for each discovered project
#
# ===== SECURITY WARNING =====
# This example saves the service account key to a local file for demonstration.
# In production environments:
# - Use a secrets manager (HashiCorp Vault, GCP Secret Manager, AWS Secrets Manager)
# - Never commit service account keys to version control
# - Consider using Workload Identity Federation instead of keys

terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
    time = {
      source = "hashicorp/time"
    }
    local = {
      source = "hashicorp/local"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# ===== Step 1: Create a custom IAM role at organization level =====
# This role provides additional permissions beyond the predefined viewer role
resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id      = "your-central-management-project"
  organization_id = "123456789012"
  title           = "Vision One CAM Service Account Role"
  description     = "Custom role for Vision One CAM service account"
}

# ===== Step 2: Configure automatic key rotation every 90 days =====
resource "time_rotating" "sa_key_rotation" {
  rotation_days = 90
}

# ===== Step 3: Create service account with organization-level scope =====
resource "visionone_cam_service_account_integration" "comprehensive" {
  depends_on = [visionone_cam_iam_custom_role.cam_role, time_rotating.sa_key_rotation]

  # Service Account Configuration
  central_management_project_id_in_org = "your-central-management-project"
  account_id                           = "visionone-cam-sa-org"
  display_name                         = "Vision One CAM Service Account"
  description                          = "Service account for Trend Micro Vision One Cloud Account Management"
  create_ignore_already_exists         = true

  # Role Configuration - predefined Viewer role + custom role
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]

  # Project filtering options
  exclude_free_trial_projects = true
  exclude_projects = [
    "project-to-exclude-1",
    "project-to-exclude-2",
  ]

  # Key Rotation Configuration
  rotation_time = time_rotating.sa_key_rotation.rotation_rfc3339
}

# ===== Step 4: Save service account key to local file =====
# WARNING: This saves sensitive credentials to disk. For production use:
# - Use a secrets manager instead (Vault, GCP Secret Manager, etc.)
# - Never commit this file to version control
# - Add "*.json" to .gitignore
resource "local_file" "service_account_key" {
  content         = base64decode(visionone_cam_service_account_integration.comprehensive.private_key)
  filename        = "${path.module}/service-account-key.json"
  file_permission = "0600"
}

# ===== Local variables for safe iteration =====
# Handle case where bound_projects might be null or empty
locals {
  bound_projects        = coalesce(visionone_cam_service_account_integration.comprehensive.bound_projects, [])
  bound_project_numbers = coalesce(visionone_cam_service_account_integration.comprehensive.bound_project_numbers, [])
  # Map of project ID to project number for connector creation
  project_id_to_number = {
    for i, pid in local.bound_projects :
    pid => local.bound_project_numbers[i]
    if i < length(local.bound_project_numbers)
  }
}

# ===== Step 5: Enable required API services for all bound projects =====
resource "visionone_cam_enable_api_services" "api_services" {
  for_each   = toset(local.bound_projects)
  project_id = each.value
}

# ===== Step 6: Create tag key for version tracking =====
# The tag key "vision-one-deployment-version" is used by CAM to identify template versions
resource "visionone_cam_tag_key" "version" {
  for_each    = toset(local.bound_projects)
  short_name  = "vision-one-deployment-version"
  parent      = "projects/${each.value}"
  description = "Version tag key for CAM template identification"
}

# ===== Step 7: Create tag value with template version =====
# Template version can be retrieved from the GCP feature list API
# API endpoint: beta/cam/gcpProjects/features
resource "visionone_cam_tag_value" "version" {
  for_each    = toset(local.bound_projects)
  short_name  = base64encode(jsonencode({ "cloud-account-management" = "3.0.2047" }))
  parent      = visionone_cam_tag_key.version[each.value].name
  description = "CAM template version tag"
}

# ===== Step 8: Create GCP connectors for all bound projects =====
# Use for_each to loop through all projects discovered in Step 3
#
# NOTE on project_number:
# - bound_projects contains project IDs (strings like "my-project")
# - bound_project_numbers contains project numbers (numeric like "123456789012")
# - The connector requires project_number, so we use bound_project_numbers via local.project_id_to_number
#
# NOTE on service_account_key:
# - The private_key output from visionone_cam_service_account_integration is ALREADY base64 encoded
# - Do NOT use base64encode() again, pass it directly
# - This differs from simple examples where you manually encode a JSON file:
#   Simple example: base64encode(file("service-account-key.json"))
#   This example:   visionone_cam_service_account_integration.comprehensive.private_key (already encoded)
resource "visionone_cam_connector_gcp" "connector" {
  for_each   = local.project_id_to_number
  depends_on = [visionone_cam_service_account_integration.comprehensive, visionone_cam_tag_value.version]

  name                      = "Vision One CAM GCP Connector - ${each.key}"
  project_number            = each.value
  service_account_id        = visionone_cam_service_account_integration.comprehensive.service_account_unique_id
  service_account_key       = visionone_cam_service_account_integration.comprehensive.private_key
  is_cam_cloud_asrm_enabled = true
  description               = "GCP connector for project ${each.key} (${each.value})"
}

# ===== Outputs =====
output "service_account_email" {
  value       = visionone_cam_service_account_integration.comprehensive.service_account_email
  description = "Email address of the service account"
}

output "service_account_unique_id" {
  value       = visionone_cam_service_account_integration.comprehensive.service_account_unique_id
  description = "Unique numeric ID of the service account"
}

output "bound_projects" {
  value       = visionone_cam_service_account_integration.comprehensive.bound_projects
  description = "List of project IDs where IAM bindings were created"
}

output "bound_project_numbers" {
  value       = visionone_cam_service_account_integration.comprehensive.bound_project_numbers
  description = "List of project numbers corresponding to bound_projects, in the same order"
}

output "key_name" {
  value       = visionone_cam_service_account_integration.comprehensive.key_name
  description = "Resource name of the service account key"
}

output "key_valid_after" {
  value       = visionone_cam_service_account_integration.comprehensive.valid_after
  description = "Timestamp when the key becomes valid"
}

output "key_valid_before" {
  value       = visionone_cam_service_account_integration.comprehensive.valid_before
  description = "Timestamp when the key expires"
}

output "connector_ids" {
  value       = { for k, v in visionone_cam_connector_gcp.connector : k => v.id }
  description = "Map of project numbers to connector IDs"
}

output "connector_states" {
  value       = { for k, v in visionone_cam_connector_gcp.connector : k => v.state }
  description = "Map of project numbers to connector states"
}

output "tag_key_names" {
  value       = { for k, v in visionone_cam_tag_key.version : k => v.name }
  description = "Map of project IDs to their tag key names"
}

output "tag_value_names" {
  value       = { for k, v in visionone_cam_tag_value.version : k => v.name }
  description = "Map of project IDs to their tag value names"
}
