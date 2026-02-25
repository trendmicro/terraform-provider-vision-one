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

# Example with custom permissions
# IMPORTANT: When you provide the 'permissions' field, it will OVERWRITE the default
# core permissions, not append to them. Ensure you include all necessary permissions.
#
# For the complete list of required permissions, refer to:
# - Vision One GCP Required Permissions: https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-gcp-required-granted-permissions
# - Permissions API (coming soon): [API endpoint to be provided]
resource "visionone_cam_iam_custom_role" "custom_permissions" {
  project_id  = "your-gcp-project-id"
  title       = "Vision One Custom Permissions Role"
  description = "Custom role with specific permissions for Vision One"

  # These permissions will REPLACE the default core permissions
  permissions = [
    "iam.roles.get",
    "iam.roles.list",
    "resourcemanager.projects.get",
    "resourcemanager.projects.getIamPolicy",
    "compute.instances.list",
    "compute.instances.get"
  ]
}
