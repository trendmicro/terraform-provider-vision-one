variable "subscription_id_list" {
  type    = set(string)
  default = ["11111111-1111-2222-aaaa-bbbbbbbbbbbb", "00000000-1ea8-4822-b823-abcdefghijkl"]
}

# Example: Multiple subscriptions using for_each
resource "visionone_cam_connector_azure" "cam_connector_azure" {
  for_each                  = toset(var.subscription_id_list)
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "Trend Micro Vision One CAM Azure Connector"
  subscription_id           = each.value
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "This is a test CAM connector created by Terraform Provider for Vision One"
  is_cam_cloud_asrm_enabled = true
}

# Example: Connector with Management Group configuration
resource "visionone_cam_connector_azure" "cam_connector_with_mgmt_group" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Management Group"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector for Azure Management Group"
  is_cam_cloud_asrm_enabled = true
  is_shared_application     = true
  cam_deployed_region       = "us-east-1"

  management_group_details = {
    id           = "mg-production"
    display_name = "Production Management Group"
  }
}

# Example: Connector with Connected Security Services
resource "visionone_cam_connector_azure" "cam_connector_with_security_services" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Security Services"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector with connected security services"
  is_cam_cloud_asrm_enabled = true

  connected_security_services = [
    {
      name         = "WorkloadSecurity"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
    }
  ]
}