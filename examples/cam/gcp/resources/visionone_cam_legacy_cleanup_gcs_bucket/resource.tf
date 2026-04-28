# Clean up legacy GCS bucket.
resource "visionone_cam_legacy_cleanup_gcs_bucket" "example" {
  project_id            = "my-gcp-project-id"
  service_account_key   = var.legacy_service_account_key
  preserve_state_bucket = true # Archive instead of delete.

  # Optional: copy state before cleanup.
  # destination_bucket = "my-centralized-state-bucket"

  # Optional: force-delete after copying state.
  # force_delete_bucket = true
}
