---
page_title: "visionone_cam_tag_value Resource - visionone"
subcategory: "GCP"
description: |-
  Manages a GCP resource tag value. Tag values are the allowed values for a specific tag key and can be attached to GCP resources.
---

# visionone_cam_tag_value (Resource)

Manages a GCP resource tag value. Tag values are the allowed values for a specific tag key and can be attached to GCP resources.

## Overview

This resource manages GCP resource tag values, which are the allowed values for a specific tag key. Tag values can be attached to GCP resources to organize, categorize, and manage them effectively.

The resource:
- Creates tag values under an existing tag key
- Supports custom descriptions for documentation
- Manages the complete lifecycle (create, read, update, delete)

**CAM Template Identification:**
Tag values are used by Cloud Account Management (CAM) to identify the template version that customers use for their environment. The `short_name` field can be set to a base64-encoded JSON string containing version information (e.g., `base64encode(jsonencode({"cloud-account-management" = "3.0.2047"}))`). This enables CAM to track and manage deployed infrastructure templates effectively. See the basic usage example below for the recommended CAM versioning pattern.

**Important:** Tag values inherit the scope from their parent tag key. Since tag keys are created at the **project level**, tag values are available only within that specific project. This ensures proper resource isolation and organization within your GCP project.

**Note:** Tag values are always scoped to the project level through their parent tag key.

## Example Usage
Create a tag value under a newly created tag key.

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

# Local variables for dynamic values
locals {
  date = formatdate("YYYY-MM-DD", timestamp())
}

# Create the version tag key
# This tag key is used by CAM to identify the template version that customers use for their environment
# The specific tag name is "vision-one-deployment-version" that the system will look for when CAM is deployed in customer's environment
resource "visionone_cam_tag_key" "cam_version_key" {
  short_name  = "vision-one-deployment-version"
  parent      = "projects/your-gcp-project-id"
  description = "Version tag key for CAM template identification"
}

# Create the version tag value
# NOTE: This tag value is used by CAM to identify the template version that customers use for their environment
# Template version can be retrieved from the GCP feature list API
# API endpoint: beta/cam/gcpProjects/features
# Portal: https://portal.xdr.trendmicro.com/index.html#/admin/automation_center
resource "visionone_cam_tag_value" "cam_version_value" {
  short_name  = base64encode(jsonencode({ "cloud-account-management" = "3.0.2047" }))
  parent      = visionone_cam_tag_key.cam_version_key.name
  description = "Created at ${local.date}"
}

output "tag_key_name" {
  description = "The resource name of the tag key (e.g., tagKeys/281477969039986)"
  value       = visionone_cam_tag_key.cam_version_key.name
}

output "tag_value_name" {
  description = "The resource name of the tag value (e.g., tagValues/987654321)"
  value       = visionone_cam_tag_value.cam_version_value.name
}
```

Create the same tag value across multiple projects with the tag key

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

# Local variables for dynamic values
locals {
  date = formatdate("YYYY-MM-DD", timestamp())
}

# Create the version tag key
# This tag key is used by CAM to identify the template version that customers use for their environment
# The specific tag name is "vision-one-deployment-version" that the system will look for when CAM is deployed in customer's environment
resource "visionone_cam_tag_key" "cam_version_key" {
  short_name  = "vision-one-deployment-version"
  parent      = "projects/your-gcp-project-id"
  description = "Version tag key for CAM template identification"
}

# Create the version tag value
# NOTE: This tag value is used by CAM to identify the template version that customers use for their environment
# Template version can be retrieved from the GCP feature list API
# API endpoint: beta/cam/gcpProjects/features
# Portal: https://portal.xdr.trendmicro.com/index.html#/admin/automation_center
resource "visionone_cam_tag_value" "cam_version_value" {
  short_name  = base64encode(jsonencode({ "cloud-account-management" = "3.0.2047" }))
  parent      = visionone_cam_tag_key.cam_version_key.name
  description = "Created at ${local.date}"
}

output "tag_key_name" {
  description = "The resource name of the tag key (e.g., tagKeys/281477969039986)"
  value       = visionone_cam_tag_key.cam_version_key.name
}

output "tag_value_name" {
  description = "The resource name of the tag value (e.g., tagValues/987654321)"
  value       = visionone_cam_tag_value.cam_version_value.name
}
```

Create the same tag value across multiple projects with other CAM resources

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

# Multiple projects tag key creation example
# This example demonstrates how to create the same tag key across multiple projects with other CAM resources using Terraform's `for_each` feature.

resource "visionone_cam_iam_custom_role" "cam_role" {
  project_id      = "org-level-project-id"
  organization_id = "organization-id"
  title           = "Vision One CAM Service Account Role Folder Scope"
  description     = "Custom role for Vision One CAM service account in central management project"
}

resource "time_rotating" "sa_key_rotation" {
  rotation_days = 90
}

resource "visionone_cam_service_account_integration" "comprehensive" {
  depends_on                           = [visionone_cam_iam_custom_role.cam_role, time_rotating.sa_key_rotation]
  central_management_project_id_in_org = "organization-id"
  account_id                           = "visionone-cam-sa"
  display_name                         = "Vision One CAM Service Account"
  description                          = "Production service account for Trend Micro Vision One Cloud Account Management with multi-project access"
  create_ignore_already_exists         = true
  roles = [
    "roles/viewer",
    visionone_cam_iam_custom_role.cam_role.name,
  ]
  exclude_free_trial_projects = true
  exclude_projects            = []
  rotation_time               = time_rotating.sa_key_rotation.rotation_rfc3339
}

resource "visionone_cam_enable_api_services" "comprehensive_api_enablement" {
  for_each   = toset(visionone_cam_service_account_integration.comprehensive.bound_projects)
  project_id = each.value
}

resource "visionone_cam_tag_key" "cam_version_key" {
  for_each    = toset(visionone_cam_service_account_integration.comprehensive.bound_projects)
  short_name  = "vision-one-deployment-version"
  parent      = "projects/${each.value}"
  description = "Version tag key for CAM template identification"
}

resource "visionone_cam_tag_value" "cam_version_value" {
  for_each    = toset(visionone_cam_service_account_integration.comprehensive.bound_projects)
  short_name  = base64encode(jsonencode({ "cloud-account-management" = "3.0.2047" }))
  parent      = visionone_cam_tag_key.cam_version_key[each.value].name
  description = "Created at ${local.date}"
}
```


## Naming Constraints
**Note:** For CAM version tracking, the system only identifies tag values with the specific structure: `base64encode(jsonencode({ "cloud-account-management" = "VERSION" }))` attached to a tag key named `vision-one-deployment-version`. The template version can be retrieved from:
- **API endpoint:** `beta/cam/gcpProjects/features`
- **Portal:** [Vision One Automation Center](https://portal.xdr.trendmicro.com/index.html#/admin/automation_center)

**Valid examples:**
- base64encode(jsonencode({ "cloud-account-management" = "VERSION" }))

## Use Cases
### CAM Version Tagging
Track CAM template versions for infrastructure management:
```hcl
# Tag Key: vision-one-deployment-version
# Tag Value: base64encode(jsonencode({"cloud-account-management" = "3.0.2047"}))
# This is used by CAM to identify the template version that customers use for their environment
# Template versions can be retrieved from: beta/cam/gcpProjects/features API endpoint
# The full API path is: https://api.xdr.trendmicro.com/beta/cam/gcpProjects/features
```

## Important Notes
### Permissions Required

To create and manage tag values, you need the following GCP IAM permissions:
- `resourcemanager.tagValues.create` - Create tag values
- `resourcemanager.tagValues.update` - Update tag values
- `resourcemanager.tagValues.delete` - Delete tag values
- `resourcemanager.tagValues.get` - Read tag value details
- `resourcemanager.tagKeys.get` - Read parent tag key details

These permissions are included in the following predefined roles:
- `roles/resourcemanager.tagAdmin` - Tag Administrator (recommended)
- `roles/resourcemanager.tagUser` - Tag User (read-only for listing/getting)
- `roles/owner` - Project/Folder/Organization Owner

### Deletion Behavior

**Tag values can only be deleted if they are not attached to any resources.** Before deleting a tag value:
1. Remove the tag value from all GCP resources using it
2. Wait for deletion propagation (may take a few minutes)
3. Then delete the tag value

If you attempt to delete a tag value that is still attached to resources, the operation will fail.

## Troubleshooting

### Error: "Permission denied" when creating tag value

**Causes:**
- Insufficient IAM permissions on the parent tag key
- Invalid or expired GCP credentials
- Organization policy restrictions

**Solution:**
- Verify you have the `resourcemanager.tagValues.create` permission
- Ensure you have access to the parent tag key
- Check that your GCP credentials are valid and have the necessary roles
- Contact your organization administrator if organization policies are blocking tag value creation

### Error: "Tag value already exists"

**Cause:** A tag value with the same `short_name` already exists within the parent tag key.

**Solution:**
- Choose a different `short_name`
- Import the existing tag value if you want Terraform to manage it
- Delete the existing tag value if it's no longer needed

### Error: "Parent tag key not found"

**Causes:**
- Invalid parent tag key format
- Parent tag key does not exist
- No access to the parent tag key

**Solution:**
- Verify the parent format is correct: `tagKeys/{tag_key_id}`
- Ensure the parent tag key exists by checking in GCP Console
- If referencing a Terraform resource, ensure the tag key resource is created first
- Confirm you have appropriate permissions on the parent tag key

### Error: "Tag value is in use" when deleting

**Cause:** The tag value is still attached to one or more GCP resources.

**Solution:**
1. Identify resources using this tag value:
```shell
# List all bindings for a tag value
gcloud resource-manager tags bindings list --tag-value=tagValues/987654321
```

2. Remove the tag value from all resources:
```shell
# Delete each binding
gcloud resource-manager tags bindings delete \
  --tag-value=tagValues/987654321 \
  --parent=//cloudresourcemanager.googleapis.com/projects/PROJECT_ID
```

3. Then delete the tag value

### Tag value not appearing in GCP Console

**Cause:** IAM propagation delay.

**Solution:**
- Wait 1-2 minutes for changes to propagate
- Refresh the GCP Console page
- Verify the tag value was created successfully by checking Terraform state

### Cannot attach tag value to resources

**Causes:**
- Tag key/value scope doesn't include the target resource
- Missing permissions to attach tags
- Resource type doesn't support tags

**Solution:**
- Verify the tag key was created at the project level and the target resource is in the same project
- Ensure you have `resourcemanager.tagBindings.create` permission
- Check that the resource type supports tag bindings (most GCP resources do)

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `parent` (String) The resource name of the parent tag key. Must be in the format 'tagKeys/{tag_key_id}'.
- `short_name` (String) The short name of the tag value. This must be unique within the parent tag key. Must be 1-63 characters, beginning and ending with an alphanumeric character, and containing only alphanumeric characters, underscores, and dashes.

### Optional

- `description` (String) Description of the tag value. Maximum of 256 characters.

### Read-Only

- `create_time` (String) The timestamp when the tag value was created.
- `etag` (String) Entity tag for concurrency control.
- `id` (String) Terraform resource identifier.
- `name` (String) The generated resource name of the tag value in the format 'tagValues/{tag_value_id}'.
- `namespaced_name` (String) The namespaced name of the tag value.
- `update_time` (String) The timestamp when the tag value was last updated.

## Import

Tag values can be imported using the format `tagValues/{tag_value_id}` or the namespaced name returned by the `name` attribute.

```shell
# Import using tag value ID
terraform import visionone_cam_tag_value.example tagValues/987654321

# Import using namespaced name (same format as the 'name' attribute)
terraform import visionone_cam_tag_value.example tagValues/987654321
```

To find your tag value ID:
1. Go to the GCP Console → Resource Manager → Tags
2. Find your tag key, then click to view its tag values
3. Copy the tag value ID
4. Use the format `tagValues/{tag_value_id}` for import
