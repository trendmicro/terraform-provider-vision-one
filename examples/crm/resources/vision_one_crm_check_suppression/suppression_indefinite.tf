# Example: Suppressing a check indefinitely
resource "visionone_crm_check_suppression" "check" {
  account_id  = "2b3c4d5e-6f7a-8b9c-0d1e-2f3a4b5c6d7e" # Vision One Cloud Risk Management account UUID
  service     = "KeyVault"
  rule_id     = "KeyVault-001"
  region      = "eastus"
  resource_id = "/subscriptions/f212b923-fc10-47fb-9940-6c844ec628d5/resourceGroups/myResources/providers/Microsoft.KeyVault/vaults/myKeyVault"
  note        = "Development environment - security exception approved"

  # Suppress indefinitely (no suppressed_until_date_time specified)
}

output "check_suppression_indefinite" {
  description = "Check suppression configuration without expiry date"
  value = {
    id   = visionone_crm_check_suppression.check.id
    note = visionone_crm_check_suppression.check.note
  }
}
