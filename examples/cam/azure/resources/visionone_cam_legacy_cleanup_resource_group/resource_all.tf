# Multi-subscription legacy Resource Group cleanup with archive mode
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

# Cleanup Custom Roles first (dependencies)
resource "visionone_cam_legacy_cleanup_custom_role" "cleanup" {
  for_each = local.cleanup_subscription_ids

  subscription_id = each.value
}

# Deploy Ver2 CAM connector resources (not shown - see CAM connector examples)
# resource "visionone_cam_connector_azure" "subscriptions" {...}

# Cleanup Resource Groups (depends on custom role cleanup)
resource "visionone_cam_legacy_cleanup_resource_group" "cleanup" {
  depends_on = [
    visionone_cam_connector_azure.subscriptions,
    visionone_cam_legacy_cleanup_custom_role.cleanup
  ]

  for_each = local.cleanup_subscription_ids

  subscription_id        = each.key
  preserve_state_storage = true  # Archive instead of delete (preserves Terraform state)
  force_delete           = false # Don't delete if state files exist
}

# Output cleanup results
output "resource_group_cleanup_summary" {
  value = {
    for k, v in visionone_cam_legacy_cleanup_resource_group.cleanup :
    k => {
      deleted            = v.deleted
      archived           = v.archived
      cleanup_status     = v.cleanup_status
      deletion_timestamp = v.deletion_timestamp
    }
  }
}
