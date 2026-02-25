# Example: Suppressing a check until a specific date/time
resource "visionone_crm_check_suppression" "check" {
  account_id  = "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d" # Vision One Cloud Risk Management account UUID
  service     = "EC2"
  rule_id     = "EC2-074"
  region      = "ap-south-1"
  resource_id = "sg-061c4319bdc0646a3"
  note        = "Suppressed for maintenance window - will be reviewed in Q2"

  # Optional: Suppress until a specific date/time (ISO 8601 format with UTC timezone)
  suppressed_until_date_time = "2026-06-30T23:59:59Z"
}

output "check_suppression_with_date" {
  description = "Check suppression configuration with expiry date"
  value = {
    id                         = visionone_crm_check_suppression.check.id
    note                       = visionone_crm_check_suppression.check.note
    suppressed_until_date_time = visionone_crm_check_suppression.check.suppressed_until_date_time
  }
}
