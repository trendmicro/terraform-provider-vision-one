# Delete the legacy service account (vision-one-service-account) and all its keys.
resource "visionone_cam_legacy_cleanup_service_account" "example" {
  project_id          = "my-gcp-project-id"
  service_account_key = var.legacy_service_account_key
}
