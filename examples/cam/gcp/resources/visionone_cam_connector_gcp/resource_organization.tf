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

# GCP connector with organization-level configuration
# This allows CAM to manage all projects under the organization
# Use excluded_projects to skip specific project numbers from the organization scope
resource "visionone_cam_connector_gcp" "cam_connector_with_organization" {
  name                      = "CAM GCP Connector with Organization"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "CAM connector with organization-level configuration"

  organization = {
    id                = "123456789"
    display_name      = "My Organization"
    excluded_projects = ["987654321098", "876543210987"]
  }
}
