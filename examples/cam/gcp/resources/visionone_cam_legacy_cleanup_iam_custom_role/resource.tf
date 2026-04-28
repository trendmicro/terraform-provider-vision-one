# Delete legacy IAM custom role.
resource "visionone_cam_legacy_cleanup_iam_custom_role" "example" {
  project_id          = "my-gcp-project-id"
  service_account_key = var.legacy_service_account_key

  # Optional: skip new provider-managed role.
  # custom_role_id = visionone_cam_iam_custom_role.new_role.name
}
