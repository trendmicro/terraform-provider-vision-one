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

# Comprehensive example with all optional fields
# Demonstrates all available configuration options for the IAM custom role resource
resource "visionone_cam_iam_custom_role" "cam_iam_custom_role" {
  # Required field
  project_id = "your-gcp-project-id"

  # Optional: Custom role ID (auto-generated if not provided)
  role_id = "visionOneComprehensiveRole"

  # Optional: Human-readable title
  title = "Vision One Comprehensive Custom Role"

  # Optional: Detailed description
  description = "A comprehensive custom role for Trend Micro Vision One Cloud Account Management with all features and custom permissions"

  # Optional: Custom list of permissions
  # IMPORTANT: If provided, these OVERWRITE (not append to) the default core permissions
  # When combined with feature_permissions, this becomes the base and features are added on top
  # For detailed permissions, refer to: [API endpoint to be provided]
  permissions = [
    "iam.roles.get",
    "iam.roles.list",
    "iam.serviceAccountKeys.create",
    "iam.serviceAccountKeys.delete",
    "iam.serviceAccounts.getAccessToken",
    "resourcemanager.tagKeys.get",
    "resourcemanager.tagKeys.list",
    "resourcemanager.tagValues.get",
    "resourcemanager.tagValues.list",
  ]

  # Optional: Feature-specific permissions
  # The provider will automatically aggregate permissions for these features on top of
  # the base permissions (either default core permissions or your custom permissions above)
  # For available features: [API endpoint to be provided]
  feature_permissions = [
    "cloud-sentry",
    "real-time-posture-monitoring"
  ]

  # Optional: Launch stage (defaults to "GA" if not specified)
  # Valid values: ALPHA, BETA, GA, DEPRECATED, DISABLED, EAP
  stage = "GA"
}

# Output the role details
output "role_name" {
  description = "The full resource name of the created role"
  value       = visionone_cam_iam_custom_role.cam_iam_custom_role.name
}

output "role_id" {
  description = "The role ID"
  value       = visionone_cam_iam_custom_role.cam_iam_custom_role.role_id
}

output "role_deleted" {
  description = "Whether the role has been deleted"
  value       = visionone_cam_iam_custom_role.cam_iam_custom_role.deleted
}
