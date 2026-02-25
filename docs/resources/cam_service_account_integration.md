---
page_title: "visionone_cam_service_account_integration Resource - visionone"
subcategory: "GCP"
description: |-
  Creates a GCP service account with rotating keys, custom IAM role, and multi-project role bindings for Trend Micro Vision One Cloud Account Management.
---

# visionone_cam_service_account_integration (Resource)

Creates a GCP service account with rotating keys, custom IAM role, and multi-project role bindings for Trend Micro Vision One Cloud Account Management.

## Overview

This resource creates a service account in GCP and integrates it with Vision One Cloud Account Management (CAM). It supports three deployment modes:

- **Single Project**: Set up the service account and related resources for a single GCP project
- **Folder Level**: Set up the service account and related resources for all projects within a GCP folder
- **Organization Level**: Set up the service account and related resources for all projects across an entire GCP organization

The resource automatically:
- Creates a service account with specified roles
- Generates and manages service account keys and supports automatic key rotation based on a configurable schedule
- Registers the service account with Vision One
- Creates custom IAM roles which will use to centralize permissions for the service account, and assign them at the appropriate level (project, folder, or organization)

## Example Usage

### Single Project Integration

Set up the service account and related resources for a single GCP project with basic configuration.

```terraform
# Example: Single GCP Project Integration

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

resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id  = "my-gcp-project-id"
  title       = "Vision One CAM Service Account Role"
  description = "Custom role for Vision One CAM service account in central management project"
}

# Configure automatic key rotation every 90 days
resource "time_rotating" "key_rotation" {
  rotation_days = 90
}

# Create a service account in a single GCP project with comprehensive configuration
resource "visionone_cam_service_account_integration" "single_project" {
  depends_on = [visionone_cam_iam_custom_role.cam_role, time_rotating.key_rotation]
  # Project where the service account will be created
  project_id = "my-gcp-project-id"

  # Service account details
  account_id   = "visionone-cam-sa"
  display_name = "Vision One CAM Service Account"
  description  = "Service account for Trend Micro Vision One Cloud Account Management"

  # Use predefined viewer role for read-only access
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]

  # Configure automatic key rotation every 90 days
  rotation_time = time_rotating.key_rotation.rotation_rfc3339

  # Optional: Ignore if service account already exists (useful for re-runs)
  create_ignore_already_exists = true
}

# ===== Outputs =====
output "service_account_email" {
  value       = try(visionone_cam_service_account_integration.single_project.service_account_email, "")
  description = "Email address of the service account"
}

output "service_account_unique_id" {
  value       = try(visionone_cam_service_account_integration.single_project.service_account_unique_id, "")
  description = "Unique numeric ID of the service account"
}

output "key_name" {
  value       = try(visionone_cam_service_account_integration.single_project.key_name, "")
  description = "Resource name of the service account key"
}

output "key_valid_after" {
  value       = try(visionone_cam_service_account_integration.single_project.valid_after, "")
  description = "Timestamp when the key becomes valid"
}

output "key_valid_before" {
  value       = try(visionone_cam_service_account_integration.single_project.valid_before, "")
  description = "Timestamp when the key expires"
}

output "private_key" {
  value       = try(visionone_cam_service_account_integration.single_project.private_key, "")
  sensitive   = true
  description = "Private key in JSON format (base64 encoded) - SENSITIVE"
}

# Example: Save private key to a file (use with caution in production)
# resource "local_file" "service_account_key" {
#   content         = base64decode(visionone_cam_service_account_integration.single_project.private_key)
#   filename        = "${path.module}/service-account-key.json"
#   file_permission = "0600"
# }
```

### Folder Level Integration

Set up the service account and related resources for all projects within a GCP folder, with options to exclude specific projects.

```terraform
# Example: GCP Folder Level Integration

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

resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id  = "my-gcp-project-id"
  title       = "Vision One CAM Service Account Role"
  description = "Custom role for Vision One CAM service account in central management project"
}

# Configure automatic key rotation every 90 days
resource "time_rotating" "key_rotation" {
  rotation_days = 90
}

# Create a service account with folder-level access
resource "visionone_cam_service_account_integration" "folder_level" {
  depends_on = [visionone_cam_iam_custom_role.cam_role, time_rotating.key_rotation]

  # Central management project where the service account will be created
  central_management_project_id_in_folder = "my-management-project"

  # Service account details
  account_id   = "visionone-cam-folder-sa"
  display_name = "Vision One CAM Service Account - Folder Level"
  description  = "Service account for monitoring all projects in the folder"

  # Use both predefined viewer role and custom role
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]

  # Optional: Exclude specific projects from monitoring
  exclude_projects = [
    "project-to-exclude-1",
    "project-to-exclude-2",
  ]

  # Optional: Exclude free trial projects
  exclude_free_trial_projects = true

  rotation_time = time_rotating.key_rotation.rotation_rfc3339

  # Optional: Ignore if service account already exists
  create_ignore_already_exists = true
}

# ===== Outputs =====
output "service_account_email" {
  value       = try(visionone_cam_service_account_integration.folder_level.service_account_email, "")
  description = "Email address of the service account"
}

output "service_account_unique_id" {
  value       = try(visionone_cam_service_account_integration.folder_level.service_account_unique_id, "")
  description = "Unique numeric ID of the service account"
}

output "role_name" {
  value       = try(visionone_cam_iam_custom_role.cam_role.name, "")
  description = "Full resource name of the custom IAM role"
}

output "role_id" {
  value       = try(visionone_cam_iam_custom_role.cam_role.role_id, "")
  description = "Role ID of the custom IAM role"
}

output "bound_projects" {
  value       = visionone_cam_service_account_integration.folder_level.bound_projects != null ? visionone_cam_service_account_integration.folder_level.bound_projects : null
  description = "List of project IDs where IAM bindings were created (only applicable in multi-project mode)"
}

output "bound_project_numbers" {
  value       = visionone_cam_service_account_integration.folder_level.bound_project_numbers != null ? visionone_cam_service_account_integration.folder_level.bound_project_numbers : null
  description = "List of project numbers corresponding to bound_projects, in the same order"
}

output "bound_projects_count" {
  value       = visionone_cam_service_account_integration.folder_level.bound_projects != null ? length(visionone_cam_service_account_integration.folder_level.bound_projects) : null
  description = "Number of projects with IAM bindings (only applicable in multi-project mode)"
}

output "key_name" {
  value       = try(visionone_cam_service_account_integration.folder_level.key_name, "")
  description = "Resource name of the service account key"
}

output "key_valid_after" {
  value       = try(visionone_cam_service_account_integration.folder_level.valid_after, "")
  description = "Timestamp when the key becomes valid"
}

output "key_valid_before" {
  value       = try(visionone_cam_service_account_integration.folder_level.valid_before, "")
  description = "Timestamp when the key expires"
}

output "private_key" {
  value       = try(visionone_cam_service_account_integration.folder_level.private_key, "")
  sensitive   = true
  description = "Private key in JSON format (base64 encoded) - SENSITIVE"
}

# Example: Save private key to a file (use with caution in production)
# resource "local_file" "service_account_key" {
#   content         = base64decode(visionone_cam_service_account_integration.folder_level.private_key)
#   filename        = "${path.module}/service-account-key.json"
#   file_permission = "0600"
# }
```

### Organization Level Integration

Set up the service account and related resources for all projects across your entire GCP organization.

```terraform
# Example: GCP Organization Level Integration

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

# Create a custom IAM role at the organization level (optional but recommended for least privilege)
resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id      = "my-management-project"
  organization_id = "123456789012"
  title           = "Vision One CAM Custom Role - Org Level"
  description     = "Custom role for Vision One CAM across the entire organization"
}

# Optional: Configure automatic key rotation every 90 days
resource "time_rotating" "key_rotation" {
  rotation_days = 90
}

# Create a service account with organization-level access
resource "visionone_cam_service_account_integration" "organization_level" {
  depends_on = [visionone_cam_iam_custom_role.cam_role, time_rotating.key_rotation]

  # Central management project where the service account will be created
  central_management_project_id_in_org = "my-management-project"

  # Service account details
  account_id   = "visionone-cam-org-sa"
  display_name = "Vision One CAM Service Account - Organization Level"
  description  = "Service account for monitoring all projects in the organization"

  # Use both predefined viewer role and custom role
  # Remove custom role line if you only want to use roles/viewer
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]

  # Optional: Exclude specific projects from monitoring
  exclude_projects = [
    "test-project",
    "sandbox-project",
  ]

  # Optional: Exclude free trial projects
  exclude_free_trial_projects = true

  # Optional: Key rotation
  rotation_time = time_rotating.key_rotation.rotation_rfc3339

  # Optional: Ignore if service account already exists
  create_ignore_already_exists = true
}

# ===== Outputs =====
output "service_account_email" {
  value       = try(visionone_cam_service_account_integration.organization_level.service_account_email, "")
  description = "Email address of the service account"
}

output "service_account_unique_id" {
  value       = try(visionone_cam_service_account_integration.organization_level.service_account_unique_id, "")
  description = "Unique numeric ID of the service account"
}

output "role_name" {
  value       = try(visionone_cam_iam_custom_role.cam_role.name, "")
  description = "Full resource name of the custom IAM role"
}

output "role_id" {
  value       = try(visionone_cam_iam_custom_role.cam_role.role_id, "")
  description = "Role ID of the custom IAM role"
}

output "bound_projects" {
  value       = visionone_cam_service_account_integration.organization_level.bound_projects != null ? visionone_cam_service_account_integration.organization_level.bound_projects : null
  description = "List of project IDs where IAM bindings were created (only applicable in multi-project mode)"
}

output "bound_project_numbers" {
  value       = visionone_cam_service_account_integration.organization_level.bound_project_numbers != null ? visionone_cam_service_account_integration.organization_level.bound_project_numbers : null
  description = "List of project numbers corresponding to bound_projects, in the same order"
}

output "bound_projects_count" {
  value       = visionone_cam_service_account_integration.organization_level.bound_projects != null ? length(visionone_cam_service_account_integration.organization_level.bound_projects) : null
  description = "Number of projects with IAM bindings (only applicable in multi-project mode)"
}

output "key_name" {
  value       = try(visionone_cam_service_account_integration.organization_level.key_name, "")
  description = "Resource name of the service account key"
}

output "key_valid_after" {
  value       = try(visionone_cam_service_account_integration.organization_level.valid_after, "")
  description = "Timestamp when the key becomes valid"
}

output "key_valid_before" {
  value       = try(visionone_cam_service_account_integration.organization_level.valid_before, "")
  description = "Timestamp when the key expires"
}

output "private_key" {
  value       = try(visionone_cam_service_account_integration.organization_level.private_key, "")
  sensitive   = true
  description = "Private key in JSON format (base64 encoded) - SENSITIVE"
}

# Example: Save private key to a file (use with caution in production)
# resource "local_file" "service_account_key" {
#   content         = base64decode(visionone_cam_service_account_integration.organization_level.private_key)
#   filename        = "${path.module}/service-account-key.json"
#   file_permission = "0600"
# }
```

## Deployment Modes

### Single Project Mode

Use this mode to set up for a single GCP project.

**Required Parameters:**
- `project_id` - The GCP project ID where the service account will be created

**Use Case:** Simple deployments, testing, or when you only need to monitor one project.

### Folder Level Mode

Use this mode to set up for all projects within a specific GCP folder.

**Required Parameters:**
- `central_management_project_id_in_folder` - The project where the service account is created

**Use Case:** You have projects organized in folders and want automatic inclusion of new projects added to the folder.
- Centralized service account management
- Can exclude specific projects (testing, sandbox, etc.)

### Organization Level Mode

Use this mode to set up for all projects across your entire GCP organization.

**Required Parameters:**
- `central_management_project_id_in_org` - The project where the service account is created

**Use Case:** Enterprise deployments requiring complete visibility across all projects.
- Most comprehensive monitoring coverage
- Centralized management at the organization level
- Can exclude specific projects (testing, sandbox, etc.)

## IAM Roles Configuration

### Predefined Roles
The most common setup uses the GCP predefined `roles/viewer` role and the visionone_cam_iam_custom_role, which provides required permissions for access:
```hcl
roles = ["roles/viewer", "visionone_cam_iam_custom_role.cam_role.name"]
```

### Custom Roles
For additional permissions beyond viewer access, create a custom role using the `visionone_cam_iam_custom_role` resource:

```hcl
resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id = "my-project"
  title      = "Vision One CAM Custom Role"
}

resource "visionone_cam_service_account_integration" "main" {
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]
}
```

For organization-level custom roles, also specify the `organization_id` in the custom role resource.

```hcl
resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id = "my-project"
  organization_id = "123456789012"  # The custom role will create at this organization ID
  title      = "Vision One CAM Custom Role"
}

resource "visionone_cam_service_account_integration" "main" {
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]
}
```


## Key Rotation
Automatic key rotation is recommended for security best practices:

```hcl
resource "time_rotating" "key_rotation" {
  rotation_days = 90  # Rotate every 90 days
}

resource "visionone_cam_service_account_integration" "main" {
  rotation_time = time_rotating.key_rotation.rotation_rfc3339
}
```

When the rotation time is reached, Terraform will:
1. Generate a new service account key
2. Register it with Vision One
3. Delete the old key

## Project Filtering

For folder and organization level deployments, you can exclude specific projects:

```hcl
resource "visionone_cam_service_account_integration" "main" {
  # Exclude specific projects by ID
  exclude_projects = [
    "test-project",
    "sandbox-project",
    "development-env"
  ]

  # Automatically exclude free trial projects
  exclude_free_trial_projects = true
}
```

## Important Notes

### Permissions Required
To create this resource, you need the following GCP IAM permissions:
- `iam.serviceAccounts.create` - Create service accounts
- `iam.serviceAccountKeys.create` - Generate service account keys
- `resourcemanager.projects.setIamPolicy` - Grant roles to the service account
- `iam.roles.create` - (If using custom roles) Create custom IAM roles
- `resourcemanager.folders.setIamPolicy` - (For folder level) Grant folder-level permissions
- `resourcemanager.organizations.setIamPolicy` - (For org level) Grant org-level permissions

### Private Key Security
The `private_key` output is sensitive and base64-encoded. Best practices:
1. **Never commit to version control** - The key is automatically registered with Vision One
2. **Avoid saving to disk** unless absolutely necessary
3. **Use file permissions `0600`** if you must save locally
4. **Enable key rotation** to minimize exposure risk

### Mutual Exclusivity
Only ONE deployment mode can be used at a time. You must specify exactly one of:
- `project_id` (single project)
- `central_management_project_id_in_folder` (folder level)
- `central_management_project_id_in_org` (organization level)

### Understanding `bound_projects` and `bound_project_numbers` Outputs

The `bound_projects` output returns a list of **GCP project IDs** (string identifiers like "my-project-123"), NOT project numbers.

The `bound_project_numbers` output returns a list of **GCP project numbers** (numeric identifiers like "123456789012") in the same order as `bound_projects`. This is useful when you need the numeric project number, for example when creating `visionone_cam_connector_gcp` resources.

**Important:** When using `bound_projects` with `visionone_cam_connector_gcp`, note that the connector resource requires `project_number` (numeric, e.g., "123456789012"). Use `bound_project_numbers` instead for direct mapping.

```hcl
# bound_projects returns project IDs (strings)
output "discovered_projects" {
  value = visionone_cam_service_account_integration.main.bound_projects
  # Example output: ["project-a", "project-b", "project-c"]
}

# bound_project_numbers returns project numbers (numeric strings)
output "discovered_project_numbers" {
  value = visionone_cam_service_account_integration.main.bound_project_numbers
  # Example output: ["123456789012", "234567890123", "345678901234"]
}

# For cam_enable_api_services - project_id is accepted
resource "visionone_cam_enable_api_services" "apis" {
  for_each   = toset(visionone_cam_service_account_integration.main.bound_projects)
  project_id = each.value  # Uses project_id directly
}

# For cam_connector_gcp - project_number is required
# Use bound_project_numbers which contains the numeric project numbers
resource "visionone_cam_connector_gcp" "connectors" {
  for_each       = {
    for i, pid in visionone_cam_service_account_integration.main.bound_projects :
    pid => visionone_cam_service_account_integration.main.bound_project_numbers[i]
  }
  project_number = each.value  # Numeric project number
  # ... other attributes
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `account_id` (String) The account ID (email prefix) for the service account. Must be 6-30 characters, lowercase letters, digits, hyphens.
- `roles` (List of String) List of IAM role resource names to bind to the service account. Each role will be bound to the service account across all target projects. Supports both custom roles (e.g., projects/{project}/roles/{role_id}) and predefined roles (e.g., roles/viewer). At least one role is required.

### Optional

- `central_management_project_id_in_folder` (String) Project ID under a folder for centralized management. Service account will receive role bindings in all projects under the same folder. Mutually exclusive with central_management_project_id_in_org.
- `central_management_project_id_in_org` (String) Project ID under an organization for centralized management. Service account will receive role bindings in all projects under the same organization.
- `create_ignore_already_exists` (Boolean) If true, skip creation if a service account with the same email already exists (handles GCP 30-day soft deletion). The resource will adopt the existing service account. Defaults to true.
- `description` (String) Description of the service account. Maximum 256 UTF-8 bytes. If not specified, defaults to 'Service account for Trend Micro Vision One Cloud Account Management'.
- `display_name` (String) Display name for the service account. If not specified, defaults to 'Vision One CAM Service Account'.
- `exclude_free_trial_projects` (Boolean) If true, exclude free trial projects when applying IAM bindings across multiple projects. Only applies when using central_management_project_id_in_folder or central_management_project_id_in_org.
- `exclude_projects` (List of String) List of project IDs to exclude from IAM bindings. Only applies when using central_management_project_id_in_folder or central_management_project_id_in_org.
- `project_id` (String) The GCP project where the service account will be created. Defaults to provider project configuration.
- `rotation_time` (String) RFC3339 timestamp from time_rotating resource to trigger key rotation. When this value changes, the old key is deleted and a new key is created. Use with time_rotating resource's rotation_rfc3339 output.

### Read-Only

- `bound_project_numbers` (List of String) List of project numbers corresponding to bound_projects, in the same order.
- `bound_projects` (List of String) List of project IDs where IAM role bindings were created for this service account.
- `key_name` (String) The resource name of the service account key.
- `private_key` (String, Sensitive) The private key in JSON format, base64 encoded. This is sensitive and should be stored securely.
- `service_account_email` (String) The email address of the created service account.
- `service_account_name` (String) The fully-qualified name of the service account (projects/{project}/serviceAccounts/{email}).
- `service_account_unique_id` (String) The unique numeric ID of the service account.
- `valid_after` (String) RFC3339 timestamp indicating when the key becomes valid.
- `valid_before` (String) RFC3339 timestamp indicating when the key expires.

## Troubleshooting

### Error: "Service account already exists"

**Solution:** Set `create_ignore_already_exists = true` in your configuration to allow Terraform to manage existing service accounts.

### Error: "Permission denied"

**Causes:**
- Insufficient IAM permissions in GCP
- Invalid or expired Vision One API key
- Missing organization or folder-level permissions

**Solution:** Verify you have the required GCP IAM permissions and that your Vision One API key has CAM permissions.

### No projects in `bound_projects` output

**Causes:**
- Incorrect folder or organization ID
- Projects excluded by filters
- IAM propagation delay

**Solution:**
- Verify your folder/organization the targeted project ID using is under the correct GCP account and has projects
- Check `exclude_projects` and `exclude_free_trial_projects` settings
- Wait a few minutes for GCP IAM changes to propagate

### Key rotation not triggering

**Cause:** The `time_rotating` resource hasn't reached its rotation time.

**Solution:** Check the `rotation_rfc3339` timestamp or manually trigger by changing the `rotation_days` value.
