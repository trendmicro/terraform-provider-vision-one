---
page_title: "visionone_cam_tag_key Resource - visionone"
subcategory: "GCP"
description: |-
  Manages a GCP resource tag key. Tag keys are used to organize GCP resources with labels that can be used in IAM policies and for resource organization.
---

# visionone_cam_tag_key (Resource)

Manages a GCP resource tag key. Tag keys are used to organize GCP resources with labels that can be used in IAM policies and for resource organization.

## Overview

This resource manages GCP resource tag keys, which are used to organize GCP resources with labels that can be used in IAM policies and for resource organization. Tag keys define the categories or dimensions for organizing your GCP resources.

The resource:
- Creates tag keys at the project level
- Supports custom descriptions for documentation
- Manages the complete lifecycle (create, read, update, delete)

**CAM Template Identification:**
Tag keys and values are used by Cloud Account Management (CAM) to identify the template version that customers use for their environment. This enables CAM to track and manage deployed infrastructure templates effectively. See the examples below for the recommended CAM versioning pattern.

**Important:** Tag keys created at the **project level** are available only to resources within that specific project. This ensures proper resource isolation and organization within your GCP project.

**Note:** This resource creates tag keys at the project level only. Tag keys must be parented by projects in the format `projects/{project_id}`.

## Example Usage

Create a simple tag key at the project level.

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

# Basic tag key at project level
# This tag key will be used by CAM to identify template versions
# The specific tag name is "vision-one-deployment-version" that the system will look for when CAM is deployed in customer's environment
resource "visionone_cam_tag_key" "cam_version_key" {
  short_name  = "vision-one-deployment-version"
  parent      = "projects/your-gcp-project-id"
  description = "Version tag key for CAM template identification"
}
```

Create the same tag key across multiple projects

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
# This example demonstrates how to create the same tag key across multiple projects using Terraform's `for_each` feature.
locals {
  projects = [
    "project-id-A",
    "project-id-B",
  ]
}
resource "visionone_cam_tag_key" "cam_version_key" {
  for_each    = toset(local.projects)
  short_name  = "vision-one-deployment-version"
  parent      = "projects/${each.value}"
  description = "Version tag key for CAM template identification"
}
```

Create the same tag key across multiple projects with other CAM resources

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
```

## Naming Constraints
**Note:** The system only recognizes `vision-one-deployment-version` as the standard CAM versioning pattern for tag keys.

**Valid examples:**
- `vision-one-deployment-version`

## Important Notes
### Permissions Required
To create and manage tag keys, you need the following GCP IAM permissions:
- `resourcemanager.tagKeys.create` - Create tag keys
- `resourcemanager.tagKeys.update` - Update tag keys
- `resourcemanager.tagKeys.delete` - Delete tag keys
- `resourcemanager.tagKeys.get` - Read tag key details
- `resourcemanager.tagKeys.list` - List tag keys

These permissions are included in the following predefined roles:
- `roles/resourcemanager.tagAdmin` - Tag Administrator (recommended)
- `roles/resourcemanager.tagUser` - Tag User (read-only)
- `roles/owner` - Project/Folder/Organization Owner

### Deletion Behavior

**Tag keys can only be deleted if they have no tag values.** Before deleting a tag key:
1. Delete all tag values associated with the key
2. Ensure no resources are using tags from this key
3. Wait for deletion propagation (may take a few minutes)

If you attempt to delete a tag key with existing tag values, the operation will fail.

## Troubleshooting

### Error: "Permission denied" when creating tag key

**Causes:**
- Insufficient IAM permissions in the parent project
- Invalid or expired GCP credentials
- Organization policy restrictions

**Solution:**
- Verify you have the `resourcemanager.tagKeys.create` permission
- Check that your GCP credentials are valid and have the necessary roles
- Contact your organization administrator if organization policies are blocking tag key creation

### Error: "Tag key already exists"

**Cause:** A tag key with the same `short_name` already exists within the parent resource.

**Solution:**
- Choose a different `short_name`
- Import the existing tag key if you want Terraform to manage it
- Delete the existing tag key if it's no longer needed

### Error: "Parent resource not found"

**Causes:**
- Invalid parent project format
- Parent project does not exist
- No access to the parent project

**Solution:**
- Verify the parent format: `projects/{project_id}`
- Ensure the parent project exists and is active
- Confirm you have appropriate permissions on the parent project

### Error: "Tag key has tag values" when deleting

**Cause:** The tag key still has associated tag values.

**Solution:**
1. List all tag values for this key using the GCP Console or `gcloud` CLI
2. Delete all tag values first
3. Then delete the tag key

```shell
# List tag values for a tag key
gcloud resource-manager tags values list --parent=tagKeys/123456789

# Delete each tag value
gcloud resource-manager tags values delete tagValues/987654321
```

### Tag key not appearing in GCP Console

**Cause:** IAM propagation delay.

**Solution:**
- Wait 1-2 minutes for changes to propagate
- Refresh the GCP Console page
- Verify the tag key was created successfully by checking Terraform state

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `parent` (String) The resource name of the parent. Must be in the format 'projects/{project_id}.'
- `short_name` (String) The short name of the tag key. This must be unique within the parent resource. Must be 1-63 characters, beginning and ending with an alphanumeric character, and containing only alphanumeric characters, underscores, and dashes.

### Optional

- `description` (String) Description of the tag key. Maximum of 256 characters.

### Read-Only

- `create_time` (String) The timestamp when the tag key was created.
- `etag` (String) Entity tag for concurrency control.
- `id` (String) Terraform resource identifier.
- `name` (String) The generated resource name of the tag key in the format 'tagKeys/{tag_key_id}'.
- `namespaced_name` (String) The namespaced name of the tag key.
- `update_time` (String) The timestamp when the tag key was last updated.

## Import

Tag keys can be imported using the format `tagKeys/{tag_key_id}` or the namespaced name returned by the `name` attribute.

```shell
# Import using tag key ID
terraform import visionone_cam_tag_key.example tagKeys/281477969039986

# Import using namespaced name (same format as the 'name' attribute)
terraform import visionone_cam_tag_key.example tagKeys/281477969039986
```

To find your tag key ID:
1. Go to the GCP Console → Resource Manager → Tags
2. Find your tag key and copy its ID
3. Use the format `tagKeys/{tag_key_id}` for import
