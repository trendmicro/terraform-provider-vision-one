variable "subscription_id_list" {
  type    = set(string)
  default = ["11111111-1111-2222-aaaa-bbbbbbbbbbbb", "00000000-1ea8-4822-b823-abcdefghijkl"]
}

resource "visionone_cam_role_definition" "cam_role_definition" {
  for_each        = var.subscription_id_list
  subscription_id = each.value
}

resource "visionone_cam_role_assignment" "cam_role_assignment" {
  for_each           = var.subscription_id_list
  role_definition_id = visionone_cam_role_definition.cam_role_definition[each.key].id
  principal_id       = visionone_cam_app_registration.cam_app_registration.principal_id
  subscription_id    = each.value
}