# Example with Management Group
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
    excluded_subscriptions = [
      "11111111-1111-1111-1111-111111111111",
      "22222222-2222-2222-2222-222222222222"
    ]
  }
}
