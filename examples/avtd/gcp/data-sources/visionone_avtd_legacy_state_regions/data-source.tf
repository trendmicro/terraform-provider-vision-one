data "visionone_avtd_legacy_state_regions" "example" {
  project_id          = "my-gcp-project-id"
  service_account_key = var.legacy_service_account_key
}

output "legacy_regions" {
  value = data.visionone_avtd_legacy_state_regions.example.regions
}
