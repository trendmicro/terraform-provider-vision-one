# Example: GCP read-only scan role for auto-detection
#
# Create this role only when you enable GCP auto-detection. It holds read-only permissions
# (resource hierarchy discovery + Cloud Asset Inventory read) and is granted once at the
# organization or folder node via node_scan_roles on visionone_cam_service_account_integration,
# so every project under the node — including projects created later — is covered through IAM
# inheritance. Unlike visionone_cam_iam_custom_role, it never includes deploy/write permissions.

terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "your-vision-one-api-key"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# Custom roles have no folder scope, so the scan role is DEFINED at the organization level
# (organization_id) and BOUND at the folder/organization node. Defining an org-level role
# requires organization-level permission.
resource "visionone_cam_gcp_scan_role" "scan_role" {
  project_id      = "my-management-project" # used for GCP authentication
  organization_id = "123456789012"
  role_id         = "trend_ai_auto_detect"
  title           = "Trend Vision One Auto-Detect Scan Role"
  description     = "Read-only discovery and scanning role bound at the organization or folder node"
}

output "scan_role_name" {
  value       = visionone_cam_gcp_scan_role.scan_role.name
  description = "Full resource name of the scan role; pass it to node_scan_roles on visionone_cam_service_account_integration"
}
