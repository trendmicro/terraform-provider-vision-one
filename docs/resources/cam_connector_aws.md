---
page_title: "visionone_cam_connector_aws Resource - visionone"
subcategory: "AWS"
description: |-
  Manages an AWS connector for Trend Micro Vision One CAM
---

# visionone_cam_connector_aws (Resource)

Manages an AWS connector for Trend Micro Vision One CAM

## Example Usage

```terraform
# Basic AWS connector example
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

resource "visionone_cam_connector_aws" "cam_connector_aws" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "Trend Micro Vision One CAM AWS Connector"
  description      = "This is a CAM connector created by Terraform Provider for Vision One"
  is_crem_enabled  = true
}
```

### Example with Features

```terraform
resource "visionone_cam_connector_aws" "cam_connector_aws" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Features"
  description      = "CAM connector with specific feature and region configuration"
  is_crem_enabled  = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["us-east-1", "us-west-2", "eu-west-1"]
    }
  ]
}

# Note: features_config_file_path requires features to also be set
resource "visionone_cam_connector_aws" "cam_connector_with_features_config" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Features Config File"
  description      = "CAM connector with features configuration file"
  is_crem_enabled  = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["us-east-1"]
    }
  ]
  features_config_file_path = "/path/to/features-config.json"
}
```

### Example with Connected Security Services

```terraform
# Note: server_workload_protection_regions is required when workload instance_ids are provided
resource "visionone_cam_connector_aws" "cam_connector_aws" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Workload Protection"
  description      = "CAM connector with Server and Workload Protection enabled"
  is_crem_enabled  = true

  connected_security_services = [
    {
      name         = "workload"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
      regions      = ["us-east-1", "us-west-2"]
    }
  ]

  server_workload_protection_regions = ["us-east-1", "us-west-2"]
}
```

### Example with AWS Organization

```terraform
# Note: is_aws_org_mgmt_account and organization_excluded_accounts require organization_id
resource "visionone_cam_connector_aws" "cam_connector_aws_org" {
  cloud_account_id        = "123456789012"
  role_arn                = "arn:aws:iam::123456789012:role/VisionOneRole"
  name                    = "CAM Connector - AWS Organization Management Account"
  description             = "CAM connector for AWS Organization management account"
  is_crem_enabled         = true
  is_aws_org_mgmt_account = true

  # organization_id: AWS OU ID (ou-<id>-<alphanum8-32>) or root ID (r-<alphanum4-32>)
  organization_id = "r-ab12"

  organization_excluded_accounts = ["999999999999"]
}
```

### Comprehensive Example - All Features Combined
- Multiple accounts, features, security services, organization, and custom tags.
<details>

```terraform
variable "aws_account_ids" {
  type    = set(string)
  default = ["123456789012", "210987654321"]
}

# Example: Multiple accounts using for_each
resource "visionone_cam_connector_aws" "cam_connector_multi" {
  for_each = var.aws_account_ids

  cloud_account_id = each.value
  role_arn         = "arn:aws:iam::${each.value}:role/VisionOneRole"
  name             = "Vision One CAM AWS Connector - ${each.value}"
  description      = "CAM connector for AWS account ${each.value}"
  is_crem_enabled  = true
}

# Example: Connector with features and config file path
resource "visionone_cam_connector_aws" "cam_connector_with_features_config" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Features Config File"
  description      = "CAM connector with features configuration file"
  is_crem_enabled  = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["us-east-1"]
    }
  ]
  features_config_file_path = "/path/to/features-config.json"
}

# Example: Connector with connected security services
resource "visionone_cam_connector_aws" "cam_connector_with_security_services" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Security Services"
  description      = "CAM connector with connected security services for workload protection"
  is_crem_enabled  = true

  connected_security_services = [
    {
      name         = "workload"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
      regions      = ["us-east-1", "us-west-2"]
    }
  ]

  server_workload_protection_regions = ["us-east-1", "us-west-2"]
}

# Example: Connector for AWS Organization management account
resource "visionone_cam_connector_aws" "cam_connector_org" {
  cloud_account_id        = "123456789012"
  role_arn                = "arn:aws:iam::123456789012:role/VisionOneRole"
  name                    = "CAM Connector for AWS Organization"
  description             = "CAM connector for AWS Organization management account"
  is_crem_enabled         = true
  is_aws_org_mgmt_account = true
  organization_id         = "r-ab12"

  organization_excluded_accounts = ["999999999999"]
}

# Example: Connector with custom tags
resource "visionone_cam_connector_aws" "cam_connector_with_tags" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector with Custom Tags"
  description      = "CAM connector with custom tags for resource identification"
  is_crem_enabled  = true

  custom_tags = {
    environment = "production"
    team        = "cam"
    cost-center = "12345"
  }
}

# Example: Connector with prevent_destroy disabled
# By default prevent_destroy=true (destroy will NOT call the CAM DELETE API).
# Set to false to allow the account to be removed from CAM on terraform destroy.
resource "visionone_cam_connector_aws" "cam_connector_allow_destroy" {
  cloud_account_id = "123456789012"
  role_arn         = "arn:aws:iam::123456789012:role/VisionOneRole"
  name             = "CAM Connector (destroyable)"
  description      = "CAM connector that will be removed from CAM on destroy"
  is_crem_enabled  = false
  prevent_destroy  = false
}
```

</details>

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `cloud_account_id` (String) AWS account ID (12-digit). Immutable — changing this forces a new resource.
- `role_arn` (String) AWS IAM Role ARN used by Trend Micro Vision One to access the AWS account

### Optional

- `cam_deployed_region` (String) AWS region where the CAM connector is deployed. Derived from `VisionOneBaseRegion` tag on the VisionOneRole; stored in state only — not sent to the API.
- `connected_security_services` (Attributes List) Connected security services (e.g. workload/SWP). Required when the Vision One tenant has an active security service instance. (see [below for nested schema](#nestedatt--connected_security_services))
- `custom_tags` (Map of String) Custom tags to apply to the connector (key-value pairs).
- `description` (String) Description of the connector
- `features` (Attributes List) List of features to enable for the connector (see [below for nested schema](#nestedatt--features))
- `features_config_file_path` (String) Path to the features configuration file
- `is_aws_org_mgmt_account` (Boolean) Marks this as the AWS Organization management account. Requires `organization_id`.
- `is_crem_enabled` (Boolean) Whether Trend Vision One Cloud CREM (isCAMCloudASRMEnabled) is enabled for the connector
- `is_tf_provider_deployed` (Boolean) Audit tag marking this account as onboarded via the Terraform provider. Defaults to `true`.
- `name` (String) Name of the connector
- `organization_excluded_accounts` (List of String) AWS account IDs (12-digit) excluded from organization onboarding. Requires `organization_id`.
- `organization_id` (String) AWS Organization/OU ID. Accepts `ou-` or `r-` prefix only (not bare `o-`). Sent as `tmv1-organizationID` header. Immutable — changing this forces a new resource.
- `prevent_destroy` (Boolean) When `true` (default), Terraform destroy will not call the CAM DELETE API, preserving the subscription in CAM. Set to `false` to allow the subscription to be removed from CAM on destroy.
- `server_workload_protection_regions` (List of String) Legacy/fallback list of AWS regions for Server & Workload Protection. Honored only when `connected_security_services` is absent.

### Read-Only

- `created_date_time` (String) Timestamp when the connector was created
- `id` (String) Unique identifier for the connector (equals cloud_account_id)
- `state` (String) Current state of the connector
- `updated_date_time` (String) Timestamp when the connector was last updated

<a id="nestedatt--connected_security_services"></a>
### Nested Schema for `connected_security_services`

Required:

- `name` (String) Name of the security service (e.g. `workload`)

Optional:

- `instance_ids` (List of String) Exactly one workload instance UUID
- `regions` (List of String) List of AWS regions for the security service


<a id="nestedatt--features"></a>
### Nested Schema for `features`

Required:

- `id` (String) Feature identifier

Optional:

- `regions` (List of String) List of regions to enable the feature in

## Import
Will be supported coming soon.
