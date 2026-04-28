# Delete the legacy Workload Identity Pool and OIDC provider (vision-one pool).
resource "visionone_cam_legacy_cleanup_workload_identity" "example" {
  project_id          = "my-gcp-project-id"
  service_account_key = var.legacy_service_account_key
}
