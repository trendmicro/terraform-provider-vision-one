resource "visionone_crm_profile" "advanced" {
  name        = "crm-profile-advanced"
  description = "Profile with advanced rule settings"

  scan_rule {
    id         = "RG-001"
    provider   = "aws"
    enabled    = true
    risk_level = "LOW"

    # Use value_set for multiple-string-values
    extra_settings {
      name      = "tags"
      type      = "multiple-string-values"
      value_set = ["Environment", "UpdatedRole"]
    }
  }
}
