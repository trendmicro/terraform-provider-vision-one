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

resource "visionone_cam_service_principal" "cam_service_principal" {
  depends_on                 = [visionone_cam_app_registration.cam_app_registration]
  application_id             = visionone_cam_app_registration.cam_app_registration.application_id
  app_registration_object_id = visionone_cam_app_registration.cam_app_registration.object_id
  tags = [
    "Version:2.0.1896" # Indicate the current version of barebone, you can find the corresponding template version through the Feature API https://automation.trendmicro.com/xdr/api-beta/#tag/Azure-subscriptions. Check the cloud-account-management section and look for the lastTemplateVersion field.
  ]
}

resource "visionone_cam_federated_identity" "cam_federated_identity" {
  depends_on                 = [visionone_cam_app_registration.cam_app_registration]
  app_registration_object_id = visionone_cam_app_registration.cam_app_registration.object_id
  application_id             = visionone_cam_app_registration.cam_app_registration.application_id
  v1_business_id             = "<your_v1_business_id>" # Replace with your Vision One business ID
  federated_identity_name    = "v1-fed-credential"     # Name of the Federated Identity, default set up to v1-fed-credential
}

output "cam_configuration_main_subscription" {
  value = {
    app_registration_id        = visionone_cam_app_registration.cam_app_registration.application_id
    app_registration_object_id = visionone_cam_app_registration.cam_app_registration.object_id
    service_principal_id       = visionone_cam_service_principal.cam_service_principal.principal_id
  }
}