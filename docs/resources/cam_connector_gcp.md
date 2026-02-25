---
page_title: "visionone_cam_connector_gcp Resource - visionone"
subcategory: "GCP"
description: |-
  Manages a GCP connector for Trend Micro Vision One CAM
---

# visionone_cam_connector_gcp (Resource)

Manages a GCP connector for Trend Micro Vision One CAM

### When to Use This Resource

| Scenario | Recommended Approach |
|----------|---------------------|
| **Single GCP project** | Use the basic example with a pre-existing service account key |
| **Multiple projects (same folder)** | Use the folder example with `visionone_cam_service_account_integration` |
| **Multiple projects (same organization)** | Use the comprehensive example with `visionone_cam_service_account_integration` |
| **Folder-wide visibility** | Set `folder` block to enable cross-project management within a folder |
| **Organization-wide visibility** | Set `organization` block to enable cross-project management |

### Resource Dependencies

For a complete CAM setup, resources should be created in this order:

1. `visionone_cam_iam_custom_role` - Create custom IAM role (optional but recommended)
2. `visionone_cam_service_account_integration` - Create service account with keys
3. `visionone_cam_enable_api_services` - Enable required GCP APIs
4. `visionone_cam_tag_key` / `visionone_cam_tag_value` - Create version tracking tags
5. `visionone_cam_connector_gcp` - Register the connector (this resource)

**Note:** For single-project setups with an existing service account, you can use this resource standalone.

### Important Notes

- **`project_number` vs `project_id`**: This resource requires the numeric `project_number` (e.g., "123456789012"), NOT the string `project_id` (e.g., "my-project"). Find it in GCP Console > Home > Project number.
- **`name` is immutable**: Changing the connector name requires resource replacement (destroy + recreate).
- **`organization` block**: Set this when you want CAM to discover and manage ALL projects under your GCP organization, not just the single project specified in `project_number`.
- **`folder` block**: Set this when you want CAM to discover and manage ALL projects under a specific GCP folder, not just the single project specified in `project_number`.
- **`organization` and `folder` are mutually exclusive**: Only one of these blocks should be set per connector.

## Example Usage

```terraform
# Basic GCP connector example - Single Project
#
# ===== WHEN TO USE THIS EXAMPLE =====
# Use this when you have:
# - A SINGLE GCP project to connect
# - An existing service account with a JSON key file
#
# For MULTI-PROJECT setup with automatic service account creation,
# see resource_all.tf instead.
#
# ===== PREREQUISITES =====
# 1. Create a GCP service account in your project
# 2. Download the JSON key file and save as "service-account-key.json"
#    (GCP Console > IAM & Admin > Service Accounts > Create Key > JSON)
# 3. The service account needs at minimum: roles/viewer
# 4. Get your project_number from GCP Console > Home (not project_id)
# 5. Get service_account_id from: Service Account details > Unique ID

terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# Connect a single GCP project to Trend Micro Vision One CAM
# NOTE: service_account_key must be base64 encoded JSON credentials
resource "visionone_cam_connector_gcp" "cam_connector_gcp" {
  name                      = "Trend Micro Vision One CAM GCP Connector"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "This is a CAM connector created by Terraform Provider for Vision One"
}
```

### Example with Organization

```terraform
terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# GCP connector with organization-level configuration
# This allows CAM to manage all projects under the organization
# Use excluded_projects to skip specific project numbers from the organization scope
resource "visionone_cam_connector_gcp" "cam_connector_with_organization" {
  name                      = "CAM GCP Connector with Organization"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "CAM connector with organization-level configuration"

  organization = {
    id                = "123456789"
    display_name      = "My Organization"
    excluded_projects = ["987654321098", "876543210987"]
  }
}
```

### Example with Folder

```terraform
terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# GCP connector with folder-level configuration
# This allows CAM to manage all projects under a specific GCP folder
# Use excluded_projects to skip specific project numbers from the folder scope
resource "visionone_cam_connector_gcp" "cam_connector_with_folder" {
  name                      = "CAM GCP Connector with Folder"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "CAM connector with folder-level configuration"

  folder = {
    id                = "123456789"
    display_name      = "My Folder"
    excluded_projects = ["987654321098", "876543210987"]
  }
}
```

### Example with Connected Security Services

```terraform
terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# GCP connector with connected security services
# Link workload protection or other Vision One security services to this connector
resource "visionone_cam_connector_gcp" "cam_connector_with_security_services" {
  name                      = "CAM GCP Connector with Security Services"
  project_number            = "123456789012"
  service_account_id        = "123456789012345678901"
  service_account_key       = base64encode(file("service-account-key.json"))
  is_cam_cloud_asrm_enabled = true
  description               = "CAM connector with connected security services"

  connected_security_services = [
    {
      name         = "workload"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
    }
  ]
}
```

### Comprehensive Example - All Features Combined
- Complete GCP CAM setup with custom IAM role, service account integration, and connector registration.
<details>

```terraform
# Comprehensive example - All GCP CAM features combined
#
# ===== WHEN TO USE THIS EXAMPLE =====
# Use this example when you need to:
# - Connect MULTIPLE GCP projects under an organization to Vision One CAM
# - Automate service account creation and key rotation
# - Have Vision One manage all projects discovered in your GCP organization
#
# For SINGLE PROJECT setup, see the "Basic example" section above - it's simpler
# and requires only a pre-existing service account key file.
#
# ===== PREREQUISITES =====
# 1. GCP Organization Admin or Project Owner permissions
# 2. Vision One API key with CAM permissions
# 3. A "central management project" in GCP where the service account will be created
# 4. Organization ID (found in GCP Console > IAM & Admin > Settings)
#
# ===== WHAT THIS EXAMPLE CREATES =====
# - Custom IAM role at organization level
# - Service account with organization-wide access
# - Automatic 90-day key rotation
# - API services enablement for all discovered projects
# - Version tracking tags for CAM template identification
# - GCP connectors for each discovered project
#
# ===== SECURITY WARNING =====
# This example saves the service account key to a local file for demonstration.
# In production environments:
# - Use a secrets manager (HashiCorp Vault, GCP Secret Manager, AWS Secrets Manager)
# - Never commit service account keys to version control
# - Consider using Workload Identity Federation instead of keys

terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
    time = {
      source = "hashicorp/time"
    }
    local = {
      source = "hashicorp/local"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# ===== Step 1: Create a custom IAM role at organization level =====
# This role provides additional permissions beyond the predefined viewer role
resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id      = "your-central-management-project"
  organization_id = "123456789012"
  title           = "Vision One CAM Service Account Role"
  description     = "Custom role for Vision One CAM service account"
}

# ===== Step 2: Configure automatic key rotation every 90 days =====
resource "time_rotating" "sa_key_rotation" {
  rotation_days = 90
}

# ===== Step 3: Create service account with organization-level scope =====
resource "visionone_cam_service_account_integration" "comprehensive" {
  depends_on = [visionone_cam_iam_custom_role.cam_role, time_rotating.sa_key_rotation]

  # Service Account Configuration
  central_management_project_id_in_org = "your-central-management-project"
  account_id                           = "visionone-cam-sa-org"
  display_name                         = "Vision One CAM Service Account"
  description                          = "Service account for Trend Micro Vision One Cloud Account Management"
  create_ignore_already_exists         = true

  # Role Configuration - predefined Viewer role + custom role
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]

  # Project filtering options
  exclude_free_trial_projects = true
  exclude_projects = [
    "project-to-exclude-1",
    "project-to-exclude-2",
  ]

  # Key Rotation Configuration
  rotation_time = time_rotating.sa_key_rotation.rotation_rfc3339
}

# ===== Step 4: Save service account key to local file =====
# WARNING: This saves sensitive credentials to disk. For production use:
# - Use a secrets manager instead (Vault, GCP Secret Manager, etc.)
# - Never commit this file to version control
# - Add "*.json" to .gitignore
resource "local_file" "service_account_key" {
  content         = base64decode(visionone_cam_service_account_integration.comprehensive.private_key)
  filename        = "${path.module}/service-account-key.json"
  file_permission = "0600"
}

# ===== Local variables for safe iteration =====
# Handle case where bound_projects might be null or empty
locals {
  bound_projects        = coalesce(visionone_cam_service_account_integration.comprehensive.bound_projects, [])
  bound_project_numbers = coalesce(visionone_cam_service_account_integration.comprehensive.bound_project_numbers, [])
  # Map of project ID to project number for connector creation
  project_id_to_number = {
    for i, pid in local.bound_projects :
    pid => local.bound_project_numbers[i]
    if i < length(local.bound_project_numbers)
  }
}

# ===== Step 5: Enable required API services for all bound projects =====
resource "visionone_cam_enable_api_services" "api_services" {
  for_each   = toset(local.bound_projects)
  project_id = each.value
}

# ===== Step 6: Create tag key for version tracking =====
# The tag key "vision-one-deployment-version" is used by CAM to identify template versions
resource "visionone_cam_tag_key" "version" {
  for_each    = toset(local.bound_projects)
  short_name  = "vision-one-deployment-version"
  parent      = "projects/${each.value}"
  description = "Version tag key for CAM template identification"
}

# ===== Step 7: Create tag value with template version =====
# Template version can be retrieved from the GCP feature list API
# API endpoint: beta/cam/gcpProjects/features
resource "visionone_cam_tag_value" "version" {
  for_each    = toset(local.bound_projects)
  short_name  = base64encode(jsonencode({ "cloud-account-management" = "3.0.2047" }))
  parent      = visionone_cam_tag_key.version[each.value].name
  description = "CAM template version tag"
}

# ===== Step 8: Create GCP connectors for all bound projects =====
# Use for_each to loop through all projects discovered in Step 3
#
# NOTE on project_number:
# - bound_projects contains project IDs (strings like "my-project")
# - bound_project_numbers contains project numbers (numeric like "123456789012")
# - The connector requires project_number, so we use bound_project_numbers via local.project_id_to_number
#
# NOTE on service_account_key:
# - The private_key output from visionone_cam_service_account_integration is ALREADY base64 encoded
# - Do NOT use base64encode() again, pass it directly
# - This differs from simple examples where you manually encode a JSON file:
#   Simple example: base64encode(file("service-account-key.json"))
#   This example:   visionone_cam_service_account_integration.comprehensive.private_key (already encoded)
resource "visionone_cam_connector_gcp" "connector" {
  for_each   = local.project_id_to_number
  depends_on = [visionone_cam_service_account_integration.comprehensive, visionone_cam_tag_value.version]

  name                      = "Vision One CAM GCP Connector - ${each.key}"
  project_number            = each.value
  service_account_id        = visionone_cam_service_account_integration.comprehensive.service_account_unique_id
  service_account_key       = visionone_cam_service_account_integration.comprehensive.private_key
  is_cam_cloud_asrm_enabled = true
  description               = "GCP connector for project ${each.key} (${each.value})"
}

# ===== Outputs =====
output "service_account_email" {
  value       = visionone_cam_service_account_integration.comprehensive.service_account_email
  description = "Email address of the service account"
}

output "service_account_unique_id" {
  value       = visionone_cam_service_account_integration.comprehensive.service_account_unique_id
  description = "Unique numeric ID of the service account"
}

output "bound_projects" {
  value       = visionone_cam_service_account_integration.comprehensive.bound_projects
  description = "List of project IDs where IAM bindings were created"
}

output "bound_project_numbers" {
  value       = visionone_cam_service_account_integration.comprehensive.bound_project_numbers
  description = "List of project numbers corresponding to bound_projects, in the same order"
}

output "key_name" {
  value       = visionone_cam_service_account_integration.comprehensive.key_name
  description = "Resource name of the service account key"
}

output "key_valid_after" {
  value       = visionone_cam_service_account_integration.comprehensive.valid_after
  description = "Timestamp when the key becomes valid"
}

output "key_valid_before" {
  value       = visionone_cam_service_account_integration.comprehensive.valid_before
  description = "Timestamp when the key expires"
}

output "connector_ids" {
  value       = { for k, v in visionone_cam_connector_gcp.connector : k => v.id }
  description = "Map of project numbers to connector IDs"
}

output "connector_states" {
  value       = { for k, v in visionone_cam_connector_gcp.connector : k => v.state }
  description = "Map of project numbers to connector states"
}

output "tag_key_names" {
  value       = { for k, v in visionone_cam_tag_key.version : k => v.name }
  description = "Map of project IDs to their tag key names"
}

output "tag_value_names" {
  value       = { for k, v in visionone_cam_tag_value.version : k => v.name }
  description = "Map of project IDs to their tag value names"
}
```

</details>

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `is_cam_cloud_asrm_enabled` (Boolean) Whether Trend Vision One Cloud CREM is enabled for the connector
- `name` (String) Name of the connector
- `project_number` (String) GCP project number for the connector
- `service_account_id` (String) GCP service account unique ID used to connect to the GCP project
- `service_account_key` (String, Sensitive) GCP service account key (JSON credentials) used to authenticate with the GCP project. Must be provided as a base64-encoded string.

### Optional

- `cam_deployed_region` (String) Region where CAM is deployed for this connector
- `connected_security_services` (Attributes List) List of connected security services for the connector (see [below for nested schema](#nestedatt--connected_security_services))
- `description` (String) Description of the connector
- `folder` (Attributes) GCP folder details for the connector (see [below for nested schema](#nestedatt--folder))
- `organization` (Attributes) GCP organization details for the connector (see [below for nested schema](#nestedatt--organization))

### Read-Only

- `created_date_time` (String) Timestamp when the connector was created
- `id` (String) Unique identifier for the connector (same as project_number)
- `project_id` (String) GCP project ID
- `project_name` (String) GCP project name
- `service_account_email` (String) GCP service account email
- `state` (String) Current state of the connector
- `updated_date_time` (String) Timestamp when the connector was last updated

<a id="nestedatt--connected_security_services"></a>
### Nested Schema for `connected_security_services`

Required:

- `instance_ids` (List of String) List of instance IDs for the security service
- `name` (String) Name of the security service


<a id="nestedatt--folder"></a>
### Nested Schema for `folder`

Required:

- `display_name` (String) Display name of the folder
- `id` (String) GCP folder ID

Optional:

- `excluded_projects` (List of String) List of project numbers to exclude from the folder


<a id="nestedatt--organization"></a>
### Nested Schema for `organization`

Required:

- `display_name` (String) Display name of the organization
- `id` (String) GCP organization ID

Optional:

- `excluded_projects` (List of String) List of project numbers to exclude from the organization

## Import
Will be supported coming soon.
