# Example with Features
resource "visionone_cam_connector_gcp" "cam_connector_with_features" {
  name                      = "CAM Connector with Features"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "GCP connector with feature configuration"

  features = [
    {
      id        = "cloud-sentry"
      locations = ["us-central1"]
    }
  ]
}

# Example with Features and a config file path
# Note: features_config_file_path requires features to also be set
resource "visionone_cam_connector_gcp" "cam_connector_with_features_config" {
  name                      = "CAM Connector with Features Config File"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "GCP connector with features configuration file"

  features = [
    {
      id        = "cloud-sentry"
      locations = ["us-central1"]
    }
  ]
  features_config_file_path = "/path/to/features-config.json"
}
