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

# Comprehensive example showing all configuration options
resource "visionone_cam_enable_api_services" "all_options" {
  # Project ID where API services will be enabled
  # Optional - defaults to provider configuration or default GCP credentials
  project_id = "your-gcp-project-id"

  # List of API services to enable
  # Optional - defaults to required services for Vision One CAM
  # When not specified, automatically enables these default services:
  # You can override this list if you need additional services:
  services = [
    "iamcredentials.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "iam.googleapis.com",
    "cloudbuild.googleapis.com",
    "deploymentmanager.googleapis.com",
    "cloudfunctions.googleapis.com",
    "pubsub.googleapis.com",
    "secretmanager.googleapis.com",
    # Add additional services as needed for new features
    # "compute.googleapis.com",
  ]
}