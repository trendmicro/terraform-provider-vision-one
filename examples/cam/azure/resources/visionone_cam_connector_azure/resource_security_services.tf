# Example with Connected Security Services
resource "visionone_cam_connector_azure" "cam_connector_with_security_services" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Security Services"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector with connected security services"
  is_cam_cloud_asrm_enabled = true

  connected_security_services = [
    {
      name         = "workload"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
    }
  ]
}
