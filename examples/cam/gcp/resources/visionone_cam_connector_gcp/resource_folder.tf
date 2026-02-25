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

# GCP connector with folder-level configuration
# This allows CAM to manage all projects under a specific GCP folder
# Use excluded_projects to skip specific project numbers from the folder scope
resource "visionone_cam_connector_gcp" "cam_connector_with_folder" {
  name                      = "CAM GCP Connector with Folder"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "CAM connector with folder-level configuration"

  folder = {
    id                = "123456789"
    display_name      = "My Folder"
    excluded_projects = ["987654321098", "876543210987"]
  }
}
