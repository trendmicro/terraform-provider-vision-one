# Delete legacy App Registration from CAM Ver1 deployment
resource "visionone_cam_legacy_cleanup_app_registration" "example" {
  subscription_id = "12345678-1234-1234-1234-123456789012"
}
