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
  application_id             = visionone_cam_app_registration.cam_app_registration.application_id
  app_registration_object_id = visionone_cam_app_registration.cam_app_registration.object_id
  cam_deployed_region        = "us"                    # Replace with your deployed region, e.g., "us"
  v1_business_id             = "<your_v1_business_id>" # Replace with your Vision One business ID
  federated_identity_name    = "v1-fed-credential"     # Name of the Federated Identity, default set up to v1-fed-credential
}

variable "subscription_id_list" {
  type    = set(string)
  default = ["11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"]
}

resource "visionone_cam_role_definition" "cam_role_definition" {
  for_each        = var.subscription_id_list
  subscription_id = each.value
}
resource "visionone_cam_role_assignment" "cam_role_assignment" {
  depends_on = [
    visionone_cam_app_registration.cam_app_registration,
    visionone_cam_service_principal.cam_service_principal,
    visionone_cam_federated_identity.cam_federated_identity,
    visionone_cam_role_definition.cam_role_definition
  ]
  for_each           = var.subscription_id_list
  role_definition_id = visionone_cam_role_definition.cam_role_definition[each.key].id
  principal_id       = visionone_cam_service_principal.cam_service_principal.principal_id
  subscription_id    = each.value

}

resource "visionone_cam_connector_azure" "cam_connector_azure" {
  depends_on = [
    visionone_cam_role_assignment.cam_role_assignment,
  ]
  for_each                    = toset(var.subscription_id_list)
  application_id              = visionone_cam_app_registration.cam_app_registration.application_id
  name                        = "Trend Micro Vision Azure CAM Connector-${each.value}"
  subscription_id             = each.value
  tenant_id                   = visionone_cam_app_registration.cam_app_registration.tenant_id
  description                 = "Production Azure CAM connector for cloud security monitoring and management via Vision One"
  is_cam_cloud_asrm_enabled   = true
  connected_security_services = [] # Specify only if there are connected security services. Example: [{"name":"workload","instance_ids":["63cd6a7e-2222-3333-7777-36e46466eac5"]}]; otherwise keep it as [].
}

output "cam_configuration_main_subscription" {
  value = {
    app_registration_id         = visionone_cam_app_registration.cam_app_registration.application_id
    app_registration_object_id  = visionone_cam_app_registration.cam_app_registration.object_id
    cam_deployed_region         = visionone_cam_federated_identity.cam_federated_identity.cam_deployed_region
    service_principal_object_id = visionone_cam_service_principal.cam_service_principal.principal_id
    subscription_id             = visionone_cam_app_registration.cam_app_registration.subscription_id
    tenant_id                   = visionone_cam_app_registration.cam_app_registration.tenant_id
    v1_account_id               = visionone_cam_federated_identity.cam_federated_identity.v1_business_id
  }
}
