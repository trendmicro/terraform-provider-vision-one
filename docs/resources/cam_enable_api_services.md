---
page_title: "visionone_cam_enable_api_services Resource - visionone"
subcategory: "GCP"
description: |-
  Enables required GCP API services for Trend Micro Vision One Cloud Account Management. This resource ensures that all necessary APIs are enabled in the specified GCP project. Please note that API services are not disabled when this resource is destroyed to prevent disruption to other resources.
---

# visionone_cam_enable_api_services (Resource)

Enables required GCP API services for Trend Micro Vision One Cloud Account Management. This resource ensures that all necessary APIs are enabled in the specified GCP project. Please note that API services are not disabled when this resource is destroyed to prevent disruption to other resources.

## Overview

This resource enables required GCP API services for Trend Micro Vision One Cloud Account Management (CAM). It ensures that all necessary Google Cloud APIs are activated in your GCP project(s).

The resource automatically:
- Enables essential GCP API services required for Vision One CAM functionality by default:
  - `iamcredentials.googleapis.com` - IAM Service Account Credentials API
  - `cloudresourcemanager.googleapis.com` - Cloud Resource Manager API
  - `iam.googleapis.com` - Identity and Access Management API
  - `cloudbuild.googleapis.com` - Cloud Build API
  - `deploymentmanager.googleapis.com` - Cloud Deployment Manager API
  - `cloudfunctions.googleapis.com` - Cloud Functions API
  - `pubsub.googleapis.com` - Cloud Pub/Sub API
  - `secretmanager.googleapis.com` - Secret Manager API
- Supports custom service lists for specific use cases or additional features
- Validates that services are enabled and available
- Preserves enabled services when the resource is destroyed (to prevent disruption)

## Example Usage

### Single Project

Enable required API services for a single GCP project with default services.

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

# Single project example
# Enable required API services for a single GCP project
resource "visionone_cam_enable_api_services" "single_project" {
  project_id = "your-gcp-project-id"
}
```

### Folder and Organization Level Integration

Enable API services for multiple projects within a GCP folder or across your entire organization. These examples demonstrate two approaches: automatic detection using `bound_projects` from the service account integration (recommended), and manual project specification.

**Note:** The configuration is identical for both folder and organization level integrations. The only difference is the parameter used to reference the central management project:
- **Folder level**: Use `central_management_project_id_in_folder` from the folder-level service account integration
- **Organization level**: Use `central_management_project_id_in_org` from the organization-level service account integration

#### Folder Level Example

```terraform
terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
    google = {
      source = "hashicorp/google"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# ==============================================================================
# Folder-level integration - Enable API services for multiple projects
# ==============================================================================
# This example shows both automatic detection (using bound_projects) and
# manual project specification approaches. See the documentation for details
# on when to use each approach.
# ==============================================================================

# Approach 1: Automatic detection using bound_projects from service account integration
resource "visionone_cam_service_account_integration" "folder_level" {
  account_id                              = "vision-one-cam-sa"
  central_management_project_id_in_folder = "your-folder-id"
}

# Enable API services for each project discovered in the folder
resource "visionone_cam_enable_api_services" "folder_projects" {
  for_each = toset(visionone_cam_service_account_integration.folder_level.bound_projects)

  project_id = each.value
}

# ==============================================================================
# Approach 2: Manual project list (alternative)
# ==============================================================================

# locals {
#   folder_project_ids = [
#     "project-1-id",
#     "project-2-id",
#     "project-3-id",
#   ]
# }
#
# resource "visionone_cam_enable_api_services" "folder_projects_manual" {
#   for_each = toset(local.folder_project_ids)
#
#   project_id = each.value
# }
```

#### Organization Level Example

```terraform
terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
    google = {
      source = "hashicorp/google"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "https://api.xdr.trendmicro.com"
}

# ==============================================================================
# Organization-level integration - Enable API services for multiple projects
# ==============================================================================
# This example shows both automatic detection (using bound_projects) and
# manual project specification approaches. See the documentation for details
# on when to use each approach.
# ==============================================================================

# Approach 1: Automatic detection using bound_projects from service account integration
resource "visionone_cam_service_account_integration" "organization_level" {
  account_id                           = "vision-one-cam-sa"
  central_management_project_id_in_org = "your-organization-id"
}

# Enable API services for each project discovered in the organization
resource "visionone_cam_enable_api_services" "org_projects" {
  for_each = toset(visionone_cam_service_account_integration.organization_level.bound_projects)

  project_id = each.value
}

# ==============================================================================
# Approach 2: Manual project list (alternative)
# ==============================================================================

# locals {
#   org_project_ids = [
#     "project-1-id",
#     "project-2-id",
#     "project-3-id",
#   ]
# }
#
# resource "visionone_cam_enable_api_services" "org_projects_manual" {
#   for_each = toset(local.org_project_ids)
#
#   project_id = each.value
# }
```

### Custom Service List

Specify a custom list of API services to enable, including additional services beyond the defaults.

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

# Comprehensive example showing all configuration options
resource "visionone_cam_enable_api_services" "all_options" {
  # Project ID where API services will be enabled
  # Optional - defaults to provider configuration or default GCP credentials
  project_id = "your-gcp-project-id"

  # List of API services to enable
  # Optional - defaults to required services for Vision One CAM
  # When not specified, automatically enables these default services:
  # You can override this list if you need additional services:
  services = [
    "iamcredentials.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "iam.googleapis.com",
    "cloudbuild.googleapis.com",
    "deploymentmanager.googleapis.com",
    "cloudfunctions.googleapis.com",
    "pubsub.googleapis.com",
    "secretmanager.googleapis.com",
    # Add additional services as needed for new features
    # "compute.googleapis.com",
  ]
}
```

## Important Notes

### Permissions Required

To enable API services, you need the following GCP IAM permission:
- `serviceusage.services.enable` - Enable API services in the project

This permission is included in the following predefined roles:
- `roles/owner` - Project Owner
- `roles/editor` - Project Editor
- `roles/serviceusage.serviceUsageAdmin` - Service Usage Admin

### Behavior on Destroy

**API services are NOT disabled when this resource is destroyed.** This is intentional to prevent disruption to other resources that may depend on these services, including:
- Existing service accounts
- Other Vision One CAM resources
- Other applications using these APIs

If you need to disable services, you must do so manually through the GCP Console or `gcloud` CLI.

### Service Enablement Time

Enabling API services typically takes a few seconds, but in some cases may take up to 1-2 minutes. The resource will wait for each service to be fully enabled before completing.

## Troubleshooting

### Error: "Permission denied" when enabling services

**Causes:**
- Insufficient IAM permissions in the GCP project
- Invalid or expired GCP credentials
- Organization policy restrictions

**Solution:**
- Verify you have the `serviceusage.services.enable` permission
- Check that your GCP credentials are valid and have the necessary roles
- Contact your organization administrator if organization policies are blocking API enablement

### Error: "Service xxx.googleapis.com not found"

**Cause:** The specified service name is invalid or not available in your GCP project.

**Solution:**
- Verify the service name format is correct (e.g., `iam.googleapis.com`)
- Check that the service is available in your GCP region
- Refer to the [GCP API Library](https://console.cloud.google.com/apis/library) for valid service names
