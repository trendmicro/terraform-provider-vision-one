# Complete Ver1 to Ver2 migration workflow with legacy cleanup
variable "primary_subscription_id" {
  type    = string
  default = "12345678-1234-1234-1234-123456789012"
}

variable "use_existing_app_registration" {
  type        = bool
  description = "Set to true if reusing existing App Registration for Ver2"
  default     = false
}

# Deploy Ver2 CAM connector resources (not shown - see CAM connector examples)
# resource "visionone_cam_connector_azure" "subscriptions" {...}

# Cleanup legacy App Registration (only if not reusing)
resource "visionone_cam_legacy_cleanup_app_registration" "primary" {
  count = var.use_existing_app_registration ? 0 : 1

  depends_on = [visionone_cam_connector_azure.subscriptions]

  subscription_id = var.primary_subscription_id
}

# Output cleanup results
output "app_registration_cleanup_summary" {
  value = var.use_existing_app_registration ? null : {
    deleted = try(
      visionone_cam_legacy_cleanup_app_registration.primary[0].deleted,
      false
    )
    service_principal_deleted = try(
      visionone_cam_legacy_cleanup_app_registration.primary[0].service_principal_deleted,
      false
    )
    federated_identity_deleted = try(
      visionone_cam_legacy_cleanup_app_registration.primary[0].federated_identity_deleted,
      false
    )
    cleanup_status = try(
      visionone_cam_legacy_cleanup_app_registration.primary[0].cleanup_status,
      "skipped"
    )
  }
}
