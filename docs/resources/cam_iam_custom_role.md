---
page_title: "visionone_cam_iam_custom_role Resource - visionone"
subcategory: "GCP"
description: |-
  Trend Micro Vision One Cloud Account Management GCP Role Definition resource. Creates a custom GCP IAM role with the necessary permissions https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-gcp-required-granted-permissions for Trend Micro Vision One Cloud Account Management.
---

# visionone_cam_iam_custom_role (Resource)

Trend Micro Vision One Cloud Account Management GCP Role Definition resource. Creates a custom GCP IAM role with the [necessary permissions](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-gcp-required-granted-permissions) for Trend Micro Vision One Cloud Account Management.

## Important: Permission Behavior

When configuring the `permissions` field, please note:
- **If `permissions` is NOT provided**: The role will use default core permissions required for Vision One CAM 
- **If `permissions` IS provided**: Your custom permissions will **OVERWRITE** (not append to) the default permissions
- **When using `feature_permissions`**: These are aggregated on top of the base permissions (either defaults or your custom list)

For detailed permission requirements, refer to the [Permissions API](coming-soon).

## Example Usage

### Basic Usage

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

# Basic example with minimal configuration
# Uses default title, description, and core permissions
# When permissions are not specified, the role will include default core permissions
# required for Vision One Cloud Account Management
resource "visionone_cam_iam_custom_role" "basic" {
  project_id = "your-gcp-project-id"
}
```

### Custom Role ID and Metadata

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

# Example with custom role_id
# Demonstrates specifying a custom role ID instead of auto-generated one
resource "visionone_cam_iam_custom_role" "custom_role_id" {
  project_id  = "your-gcp-project-id"
  role_id     = "visionOneCustomRole"
  title       = "Vision One Custom Role with Specific ID"
  description = "Custom role for Vision One with a specific role ID"
}
```

### Custom Permissions

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

# Example with custom permissions
# IMPORTANT: When you provide the 'permissions' field, it will OVERWRITE the default
# core permissions, not append to them. Ensure you include all necessary permissions.
#
# For the complete list of required permissions, refer to:
# - Vision One GCP Required Permissions: https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-gcp-required-granted-permissions
# - Permissions API (coming soon): [API endpoint to be provided]
resource "visionone_cam_iam_custom_role" "custom_permissions" {
  project_id  = "your-gcp-project-id"
  title       = "Vision One Custom Permissions Role"
  description = "Custom role with specific permissions for Vision One"

  # These permissions will REPLACE the default core permissions
  permissions = [
    "iam.roles.get",
    "iam.roles.list",
    "resourcemanager.projects.get",
    "resourcemanager.projects.getIamPolicy",
    "compute.instances.list",
    "compute.instances.get"
  ]
}
```

### Feature-Based Permissions

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

# Example with feature permissions
# When only feature_permissions are specified (without custom permissions),
# the role will include:
# 1. Default core permissions (base)
# 2. Additional permissions required by the specified features (aggregated on top)
#
# For available features and their required permissions, refer to:
# - Features API (coming soon): [API endpoint to be provided]
resource "visionone_cam_iam_custom_role" "with_features" {
  project_id  = "your-gcp-project-id"
  title       = "Vision One Role with Features"
  description = "Custom role with feature-specific permissions for Vision One"

  # Feature permissions will be added to the default core permissions
  feature_permissions = [
    "cloud-sentry",
    "real-time-posture-monitoring"
  ]
}
```

### Combined Permissions and Features

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

# Example combining custom permissions and feature permissions
# IMPORTANT: Permission behavior when both are specified:
# 1. The 'permissions' list OVERWRITES the default core permissions (becomes the new base)
# 2. The 'feature_permissions' are then aggregated on top of your custom base permissions
# 3. Final role will have: your custom permissions + feature-specific permissions
#
# For detailed permission requirements, refer to:
# - Permissions API (coming soon): [API endpoint to be provided]
resource "visionone_cam_iam_custom_role" "combined" {
  project_id  = "your-gcp-project-id"
  title       = "Vision One Combined Permissions Role"
  description = "Custom role with both custom and feature permissions"

  # These custom permissions REPLACE the default core permissions (not append)
  permissions = [
    "iam.roles.get",
    "iam.roles.list",
    "resourcemanager.projects.get",
    "resourcemanager.projects.getIamPolicy"
  ]

  # Feature permissions are aggregated on top of the custom permissions above
  feature_permissions = [
    "cloud-sentry"
  ]
}
```

### Example Detailed Usage
- Create custom roles for multiple GCP projects with all optional parameters.
<details>

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

# Comprehensive example with all optional fields
# Demonstrates all available configuration options for the IAM custom role resource
resource "visionone_cam_iam_custom_role" "cam_iam_custom_role" {
  # Required field
  project_id = "your-gcp-project-id"

  # Optional: Custom role ID (auto-generated if not provided)
  role_id = "visionOneComprehensiveRole"

  # Optional: Human-readable title
  title = "Vision One Comprehensive Custom Role"

  # Optional: Detailed description
  description = "A comprehensive custom role for Trend Micro Vision One Cloud Account Management with all features and custom permissions"

  # Optional: Custom list of permissions
  # IMPORTANT: If provided, these OVERWRITE (not append to) the default core permissions
  # When combined with feature_permissions, this becomes the base and features are added on top
  # For detailed permissions, refer to: [API endpoint to be provided]
  permissions = [
    "iam.roles.get",
    "iam.roles.list",
    "iam.serviceAccountKeys.create",
    "iam.serviceAccountKeys.delete",
    "iam.serviceAccounts.getAccessToken",
    "resourcemanager.tagKeys.get",
    "resourcemanager.tagKeys.list",
    "resourcemanager.tagValues.get",
    "resourcemanager.tagValues.list",
  ]

  # Optional: Feature-specific permissions
  # The provider will automatically aggregate permissions for these features on top of
  # the base permissions (either default core permissions or your custom permissions above)
  # For available features: [API endpoint to be provided]
  feature_permissions = [
    "cloud-sentry",
    "real-time-posture-monitoring"
  ]

  # Optional: Launch stage (defaults to "GA" if not specified)
  # Valid values: ALPHA, BETA, GA, DEPRECATED, DISABLED, EAP
  stage = "GA"
}

# Output the role details
output "role_name" {
  description = "The full resource name of the created role"
  value       = visionone_cam_iam_custom_role.cam_iam_custom_role.name
}

output "role_id" {
  description = "The role ID"
  value       = visionone_cam_iam_custom_role.cam_iam_custom_role.role_id
}

output "role_deleted" {
  description = "Whether the role has been deleted"
  value       = visionone_cam_iam_custom_role.cam_iam_custom_role.deleted
}
```

</details>

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `description` (String) Description of the Trend Micro Vision One Cloud Account Management custom role definition.
- `feature_permissions` (Set of String) Set of features associated with the Trend Micro Vision One Cloud Account Management custom role definition. When specified, the role will include all permissions required by the specified features in addition to the base permissions (either default core permissions or your custom permissions list). The permissions are automatically retrieved and aggregated according to the Trend Micro Vision One GCP required permissions documentation. For available features, see the [Features API](coming-soon). Example: `["cloud-sentry", "real-time-posture-monitoring"]`.
- `organization_id` (String) The organization ID where the custom role will be created. When specified, creates an organization-level custom role that can be used across all projects in the organization. **Recommended for multi-project deployments** to allow the same custom role to be used across all projects in a folder. When this is set, project_id is still required for GCP authentication.
- `permissions` (List of String) List of permissions associated with the Trend Micro Vision One Cloud Account Management custom role definition. **IMPORTANT**: If specified, this list will OVERWRITE (not append to) the default core permissions. If not specified, the role will include the core permissions appropriate for the parent level (organization or project). Organization-level roles include organization, folder, and project permissions, while project-level roles include only project permissions. For detailed permission requirements, refer to the [Permissions API](coming-soon).
- `project_id` (String) The project ID used for GCP authentication and API calls. When creating a project-level custom role, this is where the role will be created. When creating an organization-level custom role (with organization_id), this project is used only for authentication. Required in all cases.
- `role_id` (String) Role ID to use for this custom role. If not specified, a Trend Micro Vision One Cloud Account Management Custom Role ID will be generated.
- `stage` (String) Current launch stage of the Trend Micro Vision One Cloud Account Management custom role (e.g., ALPHA, BETA, GA, DEPRECATED).
- `title` (String) Human-readable title for the Trend Micro Vision One Cloud Account Management custom role.

### Read-Only

- `deleted` (Boolean) Whether the Trend Micro Vision One Cloud Account Management custom role has been deleted.
- `name` (String) Full resource name of the Trend Micro Vision One Cloud Account Management custom role definition.

## Import
Will be supported coming soon.
