terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your_vision_one_api_key>"
  regional_fqdn = "<regional_fqdn>" # e.g., "https://api.xdr.trendmicro.com"
}

resource "visionone_cam_app_registration" "cam_app_registration" {}

output "cam_configuration_main_subscription" {
  value = {
    app_registration_id        = visionone_cam_app_registration.cam_app_registration.application_id
    app_registration_object_id = visionone_cam_app_registration.cam_app_registration.object_id
    tenant_id                  = visionone_cam_app_registration.cam_app_registration.tenant_id
    subscription_id            = visionone_cam_app_registration.cam_app_registration.subscription_id
    display_name               = visionone_cam_app_registration.cam_app_registration.display_name
  }
}