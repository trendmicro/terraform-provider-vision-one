# Example: Suppressing a check indefinitely
resource "visionone_crm_check_suppression" "check_suppression_indefinite" {
  account_id  = "12345678-4b0c-4200-86e0-1ff4acecac5b" # Vision One Cloud Risk Management account UUID
  service     = "AutoScaling"
  rule_id     = "ASG-003"
  region      = "ap-southeast-2"
  resource_id = "lcs-test-config-pd12345"
  note        = "Development environment - security exception approved"

  # Suppress indefinitely (no suppressed_until_date_time specified)
}

output "check_suppression_indefinite" {
  description = "Check suppression configuration without expiry date"
  value = {
    id   = visionone_crm_check_suppression.check_suppression_indefinite.id
    note = visionone_crm_check_suppression.check_suppression_indefinite.note
  }
}
