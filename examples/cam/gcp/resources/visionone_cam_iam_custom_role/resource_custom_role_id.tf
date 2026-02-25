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

# Example with custom role_id
# Demonstrates specifying a custom role ID instead of auto-generated one
resource "visionone_cam_iam_custom_role" "custom_role_id" {
  project_id  = "your-gcp-project-id"
  role_id     = "visionOneCustomRole"
  title       = "Vision One Custom Role with Specific ID"
  description = "Custom role for Vision One with a specific role ID"
}
