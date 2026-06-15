# Discover which GCP regions the legacy Terraform Package Solution was running
# in for a given project. Returns an empty set when no legacy state exists,
# letting downstream cleanup resources be a no-op for fresh installs.
data "visionone_dspm_legacy_state_regions" "example" {
  project_id          = "my-gcp-project-id"
  service_account_key = var.legacy_service_account_key
}

output "legacy_regions" {
  value = data.visionone_dspm_legacy_state_regions.example.regions
}
