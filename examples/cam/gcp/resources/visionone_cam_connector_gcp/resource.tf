# Basic GCP connector example - Single Project
#
# ===== WHEN TO USE THIS EXAMPLE =====
# Use this when you have:
# - A SINGLE GCP project to connect
# - An existing service account with a JSON key file
#
# For MULTI-PROJECT setup with automatic service account creation,
# see resource_all.tf instead.
#
# ===== PREREQUISITES =====
# 1. Create a GCP service account in your project
# 2. Download the JSON key file and save as "service-account-key.json"
#    (GCP Console > IAM & Admin > Service Accounts > Create Key > JSON)
# 3. The service account needs at minimum: roles/viewer
# 4. Get your project_number from GCP Console > Home (not project_id)
# 5. Get service_account_id from: Service Account details > Unique ID

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

# Connect a single GCP project to Trend Micro Vision One CAM
# NOTE: service_account_key must be base64 encoded JSON credentials
resource "visionone_cam_connector_gcp" "cam_connector_gcp" {
  name                      = "Trend Micro Vision One CAM GCP Connector"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "This is a CAM connector created by Terraform Provider for Vision One"
}
