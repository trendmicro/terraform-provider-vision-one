# Delete legacy DSPM Package resources for a (project, region) pair so a
# Terraform Provider deployment can reuse the same name prefix without
# collision. `stage` must match the DSPM stage the legacy Package was
# deployed under: "int", "stg", or "prod".

# Service-account key resolution:
# 1. If `legacy_service_account_key` is set, use it (BYO key override).
# 2. Otherwise fall back to the key minted by the paired CAM integration —
#    its IAM bindings are granted in the same plan, so the customer
#    doesn't need to manage a key for the typical install.
variable "legacy_service_account_key" {
  type        = string
  sensitive   = true
  default     = null
  description = "Optional override: base64-encoded JSON key for a service account with delete permissions on legacy DSPM Package resources. When null, the CAM-minted key from visionone_cam_service_account_integration is used."
}

resource "visionone_dspm_legacy_cleanup_region" "example" {
  project_id = "my-gcp-project-id"
  region     = "us-east1"
  stage      = "prod"
  service_account_key = coalesce(
    var.legacy_service_account_key,
    visionone_cam_service_account_integration.comprehensive.private_key,
  )
  snapshot_disk_before_delete = true

  # Skips deleting resources still tracked in the current Provider-mode state (safe on Package-mode too).
  state_bucket = "trendai-v1-terraform-state-${substr(sha256(var.primary_project_id), 0, 16)}"

  depends_on = [visionone_cam_service_account_integration.comprehensive]
}

# ADC-only pattern: no CAM integration in the same plan, and the operator
# environment already has GCP credentials (gcloud auth application-default
# login, workload identity, or GCE metadata server). Omit service_account_key
# entirely; the provider falls back to Application Default Credentials.
resource "visionone_dspm_legacy_cleanup_region" "adc_only" {
  project_id = "my-gcp-project-id"
  region     = "us-east1"
  stage      = "prod"
  # service_account_key omitted -> ADC
}

# Drive cleanup across every region the legacy deployment touched, by reading
# the legacy state bucket and the new TFP location list. setunion ensures
# coverage of both regions that have legacy resources and regions the new
# stack will deploy into.
data "visionone_dspm_legacy_state_regions" "legacy" {
  project_id = var.project_id
  service_account_key = coalesce(
    var.legacy_service_account_key,
    visionone_cam_service_account_integration.comprehensive.private_key,
  )
}

locals {
  cleanup_regions = setunion(data.visionone_dspm_legacy_state_regions.legacy.regions, var.tfp_locations)
}

resource "visionone_dspm_legacy_cleanup_region" "per_region" {
  for_each   = local.cleanup_regions
  project_id = var.project_id
  region     = each.value
  stage      = "prod"
  service_account_key = coalesce(
    var.legacy_service_account_key,
    visionone_cam_service_account_integration.comprehensive.private_key,
  )
}
