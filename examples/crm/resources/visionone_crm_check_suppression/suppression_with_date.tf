# Example: Suppressing a check until a specific date/time
resource "visionone_crm_check_suppression" "check_suppression_with_date" {
  account_id  = "12345678-4b0c-4200-86e0-1ff4acecac5b" # Vision One Cloud Risk Management account UUID
  service     = "AutoScaling"
  rule_id     = "ASG-003"
  region      = "ap-southeast-2"
  resource_id = "lcs-test-config-pd12345"
  note        = "Suppressed for maintenance window - will be reviewed in Q2"

  # Optional: Suppress until a specific date/time (ISO 8601 format with UTC timezone)
  suppressed_until_date_time = "2026-06-30T23:59:59Z"
}

output "check_suppression_with_date" {
  description = "Check suppression configuration with expiry date"
  value = {
    id                         = visionone_crm_check_suppression.check_suppression_with_date.id
    note                       = visionone_crm_check_suppression.check_suppression_with_date.note
    suppressed_until_date_time = visionone_crm_check_suppression.check_suppression_with_date.suppressed_until_date_time
  }
}
