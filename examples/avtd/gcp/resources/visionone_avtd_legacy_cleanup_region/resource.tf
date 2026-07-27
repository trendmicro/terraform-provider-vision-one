variable "legacy_service_account_key" {
  type        = string
  sensitive   = true
  default     = null
  description = "Optional override: base64-encoded JSON key for a service account with delete permissions on legacy AVTD resources. When null, the CAM-minted key from visionone_cam_service_account_integration is used."
}

resource "visionone_avtd_legacy_cleanup_region" "example" {
  project_id = "my-gcp-project-id"
  region     = "northamerica-northeast2"
  stage      = "prod"
  service_account_key = coalesce(
    var.legacy_service_account_key,
    visionone_cam_service_account_integration.comprehensive.private_key,
  )
  preserve_resource_bucket = true

  depends_on = [visionone_cam_service_account_integration.comprehensive]
}

resource "visionone_avtd_legacy_cleanup_region" "adc_only" {
  project_id = "my-gcp-project-id"
  region     = "northamerica-northeast2"
}

data "visionone_avtd_legacy_state_regions" "legacy" {
  project_id = var.project_id
  service_account_key = coalesce(
    var.legacy_service_account_key,
    visionone_cam_service_account_integration.comprehensive.private_key,
  )
}

locals {
  cleanup_regions = setunion(data.visionone_avtd_legacy_state_regions.legacy.regions, var.tfp_locations)
}

resource "visionone_avtd_legacy_cleanup_region" "per_region" {
  for_each   = local.cleanup_regions
  project_id = var.project_id
  region     = each.value
  service_account_key = coalesce(
    var.legacy_service_account_key,
    visionone_cam_service_account_integration.comprehensive.private_key,
  )
}

resource "visionone_avtd_legacy_cleanup_region" "custom_prefixes" {
  project_id        = "my-gcp-project-id"
  region            = "northamerica-northeast2"
  resource_prefixes = ["acme-avtd", "acme-common"]
}

resource "visionone_avtd_legacy_cleanup_region" "sidecar" {
  project_id         = "my-scanned-project-id"
  sidecar_project_id = "my-legacy-sidecar-project-id"
  region             = "northamerica-northeast2"
  service_account_key = coalesce(
    var.legacy_service_account_key,
    visionone_cam_service_account_integration.comprehensive.private_key,
  )
}
