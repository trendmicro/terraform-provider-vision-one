# Delete or archive legacy Resource Group from CAM Ver1 deployment
resource "visionone_cam_legacy_cleanup_resource_group" "example" {
  subscription_id        = "12345678-1234-1234-1234-123456789012"
  preserve_state_storage = true  # Archive instead of delete (preserves Terraform state)
  force_delete           = false # Don't delete if state files exist
}
