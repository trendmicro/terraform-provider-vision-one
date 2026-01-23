resource "visionone_crm_profile" "profile_without_rules" {
  name        = "crm-profile-without-rules"
  description = "Cloud Risk Management profile - without rules"
}

output "profile_without_rules" {
  value = visionone_crm_profile.profile_without_rules
}
