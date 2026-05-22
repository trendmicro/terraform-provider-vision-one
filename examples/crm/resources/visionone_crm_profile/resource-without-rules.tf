resource "visionone_crm_profile" "basic" {
  name        = "my-crm-profile"
  description = "Basic Cloud Risk Management profile"
  # for removing the description, set it to an empty string; if not set, it will keep the previous value
  # description = ""
}
