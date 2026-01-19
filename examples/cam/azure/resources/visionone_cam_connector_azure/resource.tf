# Basic example
resource "visionone_cam_connector_azure" "cam_connector_azure" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "Trend Micro Vision One CAM Azure Connector"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "This is a test CAM connector created by Terraform Provider for Vision One"
  is_cam_cloud_asrm_enabled = true
}
