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

# Example combining custom permissions and feature permissions
# IMPORTANT: Permission behavior when both are specified:
# 1. The 'permissions' list OVERWRITES the default core permissions (becomes the new base)
# 2. The 'feature_permissions' are then aggregated on top of your custom base permissions
# 3. Final role will have: your custom permissions + feature-specific permissions
#
# For detailed permission requirements, refer to:
# - Permissions API (coming soon): [API endpoint to be provided]
resource "visionone_cam_iam_custom_role" "combined" {
  project_id  = "your-gcp-project-id"
  title       = "Vision One Combined Permissions Role"
  description = "Custom role with both custom and feature permissions"

  # These custom permissions REPLACE the default core permissions (not append)
  permissions = [
    "iam.roles.get",
    "iam.roles.list",
    "resourcemanager.projects.get",
    "resourcemanager.projects.getIamPolicy"
  ]

  # Feature permissions are aggregated on top of the custom permissions above
  feature_permissions = [
    "cloud-sentry"
  ]
}
