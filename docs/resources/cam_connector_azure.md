---
page_title: "visionone_cam_connector_azure Resource - visionone"
subcategory: "Azure"
description: |-
  Manages an Azure connector for Trend Micro Vision One CAM
---

# visionone_cam_connector_azure (Resource)

Manages an Azure connector for Trend Micro Vision One CAM

## Example Usage

```terraform
# Basic example
resource "visionone_cam_connector_azure" "cam_connector_azure" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "Trend Micro Vision One CAM Azure Connector"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "This is a test CAM connector created by Terraform Provider for Vision One"
  is_cam_cloud_asrm_enabled = true
}
```

### Example with Management Group

```terraform
# Example with Management Group
resource "visionone_cam_connector_azure" "cam_connector_with_mgmt_group" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Management Group"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector for Azure Management Group"
  is_cam_cloud_asrm_enabled = true
  is_shared_application     = true
  cam_deployed_region       = "us-east-1"

  management_group_details = {
    id           = "mg-production"
    display_name = "Production Management Group"
    excluded_subscriptions = [
      "11111111-1111-1111-1111-111111111111",
      "22222222-2222-2222-2222-222222222222"
    ]
  }
}
```

### Example with Connected Security Services

```terraform
# Example with Connected Security Services
resource "visionone_cam_connector_azure" "cam_connector_with_security_services" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Security Services"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector with connected security services"
  is_cam_cloud_asrm_enabled = true

  connected_security_services = [
    {
      name         = "workload"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
    }
  ]
}
```

### Example with Features

```terraform
# Example with Features
resource "visionone_cam_connector_azure" "cam_connector_with_features" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Features"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector with feature configuration"
  is_cam_cloud_asrm_enabled = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["centralus"]
    }
  ]
}

# Example with Features and a config file path
# Note: features_config_file_path requires features to also be set
resource "visionone_cam_connector_azure" "cam_connector_with_features_config" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Features Config File"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector with features configuration file"
  is_cam_cloud_asrm_enabled = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["centralus"]
    }
  ]
  features_config_file_path = "/path/to/features-config.json"
}
```

### Example Detailed Usage
- Use the existing CAM App Registration to connect multiple Azure subscriptions.
<details>

```terraform
variable "subscription_id_list" {
  type    = set(string)
  default = ["11111111-1111-2222-aaaa-bbbbbbbbbbbb", "00000000-1ea8-4822-b823-abcdefghijkl"]
}

# Example: Multiple subscriptions using for_each
resource "visionone_cam_connector_azure" "cam_connector_azure" {
  for_each                  = toset(var.subscription_id_list)
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "Trend Micro Vision One CAM Azure Connector"
  subscription_id           = each.value
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "This is a test CAM connector created by Terraform Provider for Vision One"
  is_cam_cloud_asrm_enabled = true
}

# Example: Connector with Management Group configuration
resource "visionone_cam_connector_azure" "cam_connector_with_mgmt_group" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Management Group"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector for Azure Management Group"
  is_cam_cloud_asrm_enabled = true
  is_shared_application     = true
  cam_deployed_region       = "us-east-1"

  management_group_details = {
    id           = "mg-production"
    display_name = "Production Management Group"
  }
}

# Example: Connector with Connected Security Services
resource "visionone_cam_connector_azure" "cam_connector_with_security_services" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Security Services"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector with connected security services"
  is_cam_cloud_asrm_enabled = true

  connected_security_services = [
    {
      name         = "WorkloadSecurity"
      instance_ids = ["abcdef12-3456-7890-abcd-ef1234567890"]
    }
  ]
}

# Example: Connector with Features
resource "visionone_cam_connector_azure" "cam_connector_with_features" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Features"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector with feature configuration"
  is_cam_cloud_asrm_enabled = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["centralus"]
    }
  ]
}

# Example: Connector with Features and Config File Path
# Note: features_config_file_path requires features to also be set
resource "visionone_cam_connector_azure" "cam_connector_with_features_config" {
  application_id            = "aaaaaaaa-pppp-pppp-iiii-dddddddddddd"
  name                      = "CAM Connector with Features Config File"
  subscription_id           = "ssssssss-uuuu-bbbb-iiii-dddddddddddd"
  tenant_id                 = "tttttttt-eeee-nnnn-aaaa-nantid123456"
  description               = "CAM connector with features configuration file"
  is_cam_cloud_asrm_enabled = true

  features = [
    {
      id      = "cloud-sentry"
      regions = ["centralus"]
    }
  ]
  features_config_file_path = "/path/to/features-config.json"
}
```

</details>

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `application_id` (String) Azure application ID which is used to connect to the Azure subscription
- `is_cam_cloud_asrm_enabled` (Boolean) Whether Trend Vision One Cloud CREM is enabled for the connector
- `name` (String) Name of the connector
- `subscription_id` (String) Azure subscription ID for the connector
- `tenant_id` (String) Azure tenant ID for the connector

### Optional

- `cam_deployed_region` (String) Region where CAM is deployed for this connector
- `connected_security_services` (Attributes List) List of connected security services for the connector (see [below for nested schema](#nestedatt--connected_security_services))
- `description` (String) Description of the connector
- `features` (Attributes List) List of features to enable for the connector (see [below for nested schema](#nestedatt--features))
- `features_config_file_path` (String) Path to the features configuration file
- `is_shared_application` (Boolean) Whether the application is shared across multiple connectors
- `management_group_details` (Attributes) Azure management group details for the connector (see [below for nested schema](#nestedatt--management_group_details))
- `prevent_destroy` (Boolean) When `true` (default), Terraform destroy will not call the CAM DELETE API, preserving the subscription in CAM. Set to `false` to allow the subscription to be removed from CAM on destroy.

### Read-Only

- `created_date_time` (String) Timestamp when the connector was created
- `id` (String) Unique identifier for the connector
- `state` (String) Current state of the connector
- `updated_date_time` (String) Timestamp when the connector was last updated

<a id="nestedatt--connected_security_services"></a>
### Nested Schema for `connected_security_services`

Required:

- `instance_ids` (List of String) List of instance IDs for the security service
- `name` (String) Name of the security service


<a id="nestedatt--features"></a>
### Nested Schema for `features`

Required:

- `id` (String) Feature identifier

Optional:

- `regions` (List of String) List of regions to enable the feature in


<a id="nestedatt--management_group_details"></a>
### Nested Schema for `management_group_details`

Required:

- `display_name` (String) Display name of the management group
- `id` (String) Azure management group ID

Optional:

- `excluded_subscriptions` (List of String) List of subscription IDs to exclude from the management group

## Import
Will supported coming soon.