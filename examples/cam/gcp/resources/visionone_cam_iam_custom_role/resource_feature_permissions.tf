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

# Example with feature permissions
# When only feature_permissions are specified (without custom permissions),
# the role will include:
# 1. Default core permissions (base)
# 2. Additional permissions required by the specified features (aggregated on top)
#
# For available features and their required permissions, refer to:
# - Features API (coming soon): [API endpoint to be provided]
resource "visionone_cam_iam_custom_role" "with_features" {
  project_id  = "your-gcp-project-id"
  title       = "Vision One Role with Features"
  description = "Custom role with feature-specific permissions for Vision One"

  # Feature permissions will be added to the default core permissions
  feature_permissions = [
    "cloud-sentry",
    "real-time-posture-monitoring"
  ]
}
