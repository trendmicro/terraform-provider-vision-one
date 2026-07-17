---
page_title: "visionone_dspm_legacy_cleanup_region Resource - visionone"
subcategory: "Data Security Posture Management"
description: |-
  Deletes the per-region DSPM resources created by the legacy Terraform Package Solution in a single GCP project, so a Terraform Provider deployment can reuse the same name prefix. Each instance is keyed by (project_id, region). Deletion order matches the original local-exec bash: eventarc triggers → functions / run services → schedulers → disk (snapshot first if requested) + resource policy → VMs → VPC connector → firewall rules → NAT → router → subnet → VPC. Returns cleanup_status = "not_found" if no matching legacy resources exist in the region.
---

# visionone_dspm_legacy_cleanup_region (Resource)

Deletes the per-region DSPM resources created by the legacy Terraform Package Solution in a single GCP project, so a Terraform Provider deployment can reuse the same name prefix. Each instance is keyed by `(project_id, region)`. Deletion order matches the original local-exec bash: eventarc triggers → functions / run services → schedulers → disk (snapshot first if requested) + resource policy → VMs → VPC connector → firewall rules → NAT → router → subnet → VPC. Returns `cleanup_status = "not_found"` if no matching legacy resources exist in the region.

## Use Cases

- **Legacy to Provider Migration**: Delete the per-region DSPM Package resources (functions, schedulers, VMs, VPC, …) so the Terraform Provider Solution can reuse the same `dspm-{stage}-{region}` name prefix.
- **Multi-Region Cleanup**: Combine with `visionone_dspm_legacy_state_regions` to drive `for_each` cleanup across every region the legacy deployment touched.
- **Safe Re-Runs**: NotFound responses are treated as success, so the resource is idempotent across retries.

## Behavior

- **`terraform apply`**: Walks the legacy resource families in the same order as the original local-exec bash (eventarc triggers → functions / run services → schedulers → disk (snapshot first if requested) + resource policy → VMs → VPC connector → firewall rules → NAT → router → subnet → VPC).
- **`terraform destroy`**: Removes the resource from Terraform state only; legacy GCP objects already deleted by the apply remain absent.
- **`cleanup_status`**: One of `deleted`, `partial`, `not_found`, `failed`. `not_found` is returned when no legacy resources matched the prefix — the fresh-install path.
- **Disk Snapshot** (`snapshot_disk_before_delete = true`, default): the persistent scan-job disk is snapshotted to `{name_prefix}-disk-pre-upgrade` before deletion so the new stack can migrate scan data on first boot.

## Example Usage

```terraform
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
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `project_id` (String) The GCP project ID whose legacy DSPM resources should be cleaned up.
- `region` (String) The GCP region to clean up (e.g. `us-east1`).
- `stage` (String) DSPM stage the legacy Package deployment was rolled out under. One of `int`, `stg`, `prod`. The legacy resource name prefix becomes `dspm-{i|s|p}-{region_abbr}`, derived from this value.

### Optional

- `service_account_key` (String, Sensitive) Base64-encoded JSON service account key used to authenticate with GCP for cleanup operations. Optional — three common patterns:

- **CAM-integrated** (recommended): set to `visionone_cam_service_account_integration.comprehensive.private_key`. The CAM-minted SA (with IAM bindings granted in the same plan) is used without any customer-side key management.
- **BYO key**: set to a base64-encoded JSON key for any service account with delete permissions on the legacy DSPM resources. Use this when operator policy forbids using the CAM-minted SA or ADC for delete operations (e.g. enterprise-managed credentials, scope-limited audit trail).
- **ADC**: omit the attribute entirely. The provider falls back to Application Default Credentials (gcloud, workload identity, GCE metadata).
- `snapshot_disk_before_delete` (Boolean) When true (default), the persistent scan-job disk is snapshotted as `{name_prefix}-disk-pre-upgrade` before deletion. Keep enabled so main-app can migrate scan data on first boot of the new stack.
- `state_bucket` (String) GCS bucket holding the *current* Provider-mode Terraform state for this deployment (read from `gs://{state_bucket}/terraform.tfstate/default.tfstate`). When set, cleanup checks each candidate resource against this state before deleting it, and skips anything already tracked there — this is what prevents a forced replacement of this resource (e.g. a `bound_projects` change rotating the CAM service account key) from deleting infrastructure that a Provider-mode-to-Provider-mode migration is still actively using via a shared state file. Omit for a legacy Package-mode migration, where no Provider-mode state exists yet and the unconditional-delete behavior is safe (see `visionone_dspm_legacy_state_regions` for that path). If the state object can't be read for a reason other than "doesn't exist yet" (e.g. a permissions error), cleanup fails closed — it reports `cleanup_status = "failed"` rather than silently deleting as if nothing were tracked.

### Read-Only

- `cleanup_error` (String) Error message if cleanup encountered failures.
- `cleanup_status` (String) Status: `deleted`, `partial`, `not_found`, or `failed`.
- `deletion_timestamp` (String) RFC3339 timestamp when cleanup was performed.
- `id` (String) `{project_id}/{region}`.
- `name_prefix` (String) The computed legacy resource prefix (e.g. `dspm-i-use1`).
- `orphan_bucket_names` (List of String) GCS bucket names that pre-existed for this (project, region) tuple and were intentionally **not** deleted by cleanup. Audit-log buckets are data-preservation-sensitive, and deleting them races GCP's audit-log forwarding pipeline. Consume this list from the downstream new-module via `import { for_each = ... }` blocks to adopt the buckets into the new state. Empty on fresh installs.
- `resources_deleted` (Map of Number) Count of legacy resources deleted, keyed by resource family (functions, triggers, schedulers, run_services, vms, firewalls, router_nats, routers, subnets, vpcs, connectors, disks, snapshots, resource_policies, sinks, alert_policies, dashboards, orphan_buckets_preserved, orphan_bindings).
- `resources_preserved` (Map of Number) Count of candidate resources intentionally **not** deleted because `state_bucket` lookup found them already tracked in the current Provider-mode state, keyed by the same resource family names used in `resources_deleted` (only families that can be state-checked appear: firewalls, router_nats, routers, subnets, vpcs, connectors, disks, resource_policies, sinks, alert_policies, dashboards). Empty when `state_bucket` is unset.
- `snapshot_name` (String) The disk snapshot name created before disk deletion (empty if no disk existed or snapshot was disabled).

## Required Permissions

The authenticating principal must have GCP permissions to delete the legacy DSPM resources in the target project / region, including:

- `compute.disks.{get,delete,createSnapshot}`
- `compute.instances.{list,delete}`
- `compute.networks.delete`, `compute.subnetworks.delete`, `compute.routers.{get,patch,delete}`, `compute.firewalls.delete`
- `compute.resourcePolicies.delete`, `compute.snapshots.get`
- `vpcaccess.connectors.delete`
- `cloudfunctions.functions.delete`
- `run.services.delete`
- `cloudscheduler.jobs.delete`
- `eventarc.triggers.delete`
