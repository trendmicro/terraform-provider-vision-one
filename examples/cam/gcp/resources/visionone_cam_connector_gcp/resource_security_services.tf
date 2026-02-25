terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# GCP connector with connected security services
# Link workload protection or other Vision One security services to this connector
resource "visionone_cam_connector_gcp" "cam_connector_with_security_services" {
  name                      = "CAM GCP Connector with Security Services"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "CAM connector with connected security services"

  connected_security_services = [
    {
      name         = "workload"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
    }
  ]
}
