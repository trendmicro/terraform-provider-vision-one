variable "subscription_id_list" {
  type    = set(string)
  default = ["11111111-1111-2222-aaaa-bbbbbbbbbbbb", "00000000-1ea8-4822-b823-abcdefghijkl"]
}

resource "visionone_cam_role_definition" "cam_role_definition" {
  for_each        = var.subscription_id_list
  subscription_id = each.value
}

