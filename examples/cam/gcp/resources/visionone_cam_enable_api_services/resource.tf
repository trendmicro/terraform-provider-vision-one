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

# Single project example
# Enable required API services for a single GCP project
resource "visionone_cam_enable_api_services" "single_project" {
  project_id = "your-gcp-project-id"
}
