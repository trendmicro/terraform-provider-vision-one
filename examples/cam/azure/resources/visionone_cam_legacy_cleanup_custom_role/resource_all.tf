# Multi-subscription legacy Custom Role cleanup
variable "subscription_ids" {
  type = list(string)
  default = [
    "12345678-1234-1234-1234-123456789012",
    "23456789-2345-2345-2345-234567890123",
  ]
}

locals {
  cleanup_subscription_ids = toset(var.subscription_ids)
}

# Deploy Ver2 CAM connector resources (not shown - see CAM connector examples)
# resource "visionone_cam_connector_azure" "subscriptions" {...}

# Cleanup legacy Custom Roles
resource "visionone_cam_legacy_cleanup_custom_role" "cleanup" {
  depends_on = [visionone_cam_connector_azure.subscriptions]

  for_each = local.cleanup_subscription_ids

  subscription_id = each.key
}

# Output cleanup results
output "custom_role_cleanup_summary" {
  value = {
    for k, v in visionone_cam_legacy_cleanup_custom_role.cleanup :
    k => {
      deleted            = v.deleted
      role_assignments   = v.role_assignments_count
      cleanup_status     = v.cleanup_status
      deletion_timestamp = v.deletion_timestamp
    }
  }
}
