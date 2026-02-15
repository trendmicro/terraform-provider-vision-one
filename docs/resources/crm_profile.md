---
page_title: "visionone_crm_profile Resource - visionone"
subcategory: "Cloud Risk Management"
description: |-
  Manages a Cloud Risk Management profile with rule settings.
---

# visionone_crm_profile (Resource)

Manages a Cloud Risk Management profile with rule settings.

## Example Usage

### Basic Profile Without Rules

```terraform
resource "visionone_crm_profile" "basic" {
  name        = "my-crm-profile"
  description = "Basic Cloud Risk Management profile"
}
```

### Profile With Scan Rules

```terraform
resource "visionone_crm_profile" "with_rules" {
  name        = "crm-profile-with-rules"
  description = "Profile with multiple scan rules"

  scan_rule {
    id         = "EC2-001"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"
  }

  scan_rule {
    id         = "RTM-002"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"

    extra_settings {
      name  = "ttl"
      type  = "ttl"
      value = 72
    }
  }

  # Type: multiple-object-values with jsonencode
  scan_rule {
    id         = "RTM-011"
    provider   = "aws"
    enabled    = true
    risk_level = "LOW"

    extra_settings {
      name = "patterns"
      type = "multiple-object-values"

      values {
        value = jsonencode({
          eventSource      = "test"
          eventName        = "my-event"
          userIdentityType = "test"
        })
      }
    }
  }

  # Type: choice-multiple-value (GCP provider)
  scan_rule {
    id         = "CloudSQL-031"
    provider   = "gcp"
    enabled    = false
    risk_level = "LOW"

    extra_settings {
      name = "LogErrorVerbosity"
      type = "choice-multiple-value"

      values {
        value   = "default"
        enabled = false
      }

      values {
        value   = "terse"
        enabled = true
      }

      values {
        value   = "verbose"
        enabled = false
      }
    }
  }
}
```

### Profile With Simple Value Lists (using value_set)

For simple list types, use `value_set` for a cleaner syntax. Supported types include:
`multiple-string-values`, `multiple-ip-values`, `multiple-aws-account-values`, `multiple-number-values`,
`regions`, `ignored-regions`, `tags`, and `countries`.

```terraform
resource "visionone_crm_profile" "with_value_set" {
  name        = "crm-profile-value-set"
  description = "Profile using value_set for simple lists"

  # Type: multiple-string-values
  scan_rule {
    id         = "RG-001"
    provider   = "aws"
    enabled    = true
    risk_level = "LOW"

    extra_settings {
      name      = "tags"
      type      = "multiple-string-values"
      value_set = ["Environment", "CostCenter", "Owner"]
    }
  }

  # Type: multiple-ip-values
  scan_rule {
    id         = "RTM-007"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"

    extra_settings {
      name      = "authorisedIps"
      type      = "multiple-ip-values"
      value_set = ["10.0.0.0", "10.0.0.3", "192.168.1.1"]
    }

    extra_settings {
      name  = "ttl"
      type  = "ttl"
      value = 23
    }
  }

  # Type: multiple-aws-account-values
  scan_rule {
    id         = "SNS-002"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"

    extra_settings {
      name      = "friendlyAccounts"
      type      = "multiple-aws-account-values"
      value_set = ["123456789012", "210987654321"]
    }
  }

  # Type: multiple-number-values use numbers directly
  scan_rule {
    id         = "EC2-034"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"

    extra_settings {
      name      = "commonlyUsedPorts"
      type      = "multiple-number-values"
      value_set = [80, 443, 22, 3306, 5432]

      # or you can use strings, they are converted to numbers
      # value_set = ["80", "443", "22", "3306", "5432"]
    }

  }

  # Type: regions
  scan_rule {
    id         = "RTM-008"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"

    extra_settings {
      name      = "authorisedRegions"
      type      = "regions"
      value_set = ["us-east-1", "us-west-2", "eu-west-1"]
    }

    extra_settings {
      name  = "ttl"
      type  = "ttl"
      value = 24
    }
  }

  # Type: ignored-regions
  scan_rule {
    id         = "Config-001"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"

    extra_settings {
      name      = "ignoredRegions"
      type      = "ignored-regions"
      value_set = ["eu-west-1", "ap-southeast-1"]
    }
  }

  # Type: countries
  scan_rule {
    id         = "RTM-005"
    provider   = "aws"
    enabled    = true
    risk_level = "LOW"

    extra_settings {
      name      = "authorisedCountries"
      type      = "countries"
      value_set = ["US", "AU", "NZ", "GB", "CA"]
    }
  }
}
```

### Profile With Multiple Setting Types

Example showing a rule with exceptions and multiple extra_settings using value_set:

```terraform
resource "visionone_crm_profile" "with_multiple_settings" {
  name        = "crm-profile-multiple-settings"
  description = "Profile with multiple setting types"

  scan_rule {
    id         = "SNS-002"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"

    exceptions {
      filter_tags  = ["ignore_this_tag"]
      resource_ids = []
    }

    extra_settings {
      name      = "friendlyAccounts"
      type      = "multiple-aws-account-values"
      value_set = ["123456789012"]
    }

    extra_settings {
      name      = "accountTags"
      type      = "tags"
      value_set = ["env_prod", "team_devops"]
    }
  }
}
```

### Profile With Advanced Extra Settings

```terraform
resource "visionone_crm_profile" "advanced" {
  name        = "crm-profile-advanced"
  description = "Profile with advanced rule settings"

  scan_rule {
    id         = "RG-001"
    provider   = "aws"
    enabled    = true
    risk_level = "LOW"

    # Use value_set for multiple-string-values
    extra_settings {
      name      = "tags"
      type      = "multiple-string-values"
      value_set = ["Environment", "UpdatedRole"]
    }
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the profile.

### Optional

- `description` (String) The description of the profile.
- `scan_rule` (Block Set) List of scan rule configurations. (see [below for nested schema](#nestedblock--scan_rule))

### Read-Only

- `id` (String) The unique ID of the profile. If provided, the resource will update the existing profile instead of creating a new one.

<a id="nestedblock--scan_rule"></a>
### Nested Schema for `scan_rule`

Required:

- `enabled` (Boolean) Whether the rule is enabled.
- `id` (String) The rule ID.
- `provider` (String) The cloud provider. Allowed values: aws, azure, gcp, oci, alibabaCloud.
- `risk_level` (String) The risk level of the rule. Allowed values: LOW, MEDIUM, HIGH, VERY_HIGH, EXTREME.

Optional:

- `exceptions` (Block, Optional) Rule exceptions configuration. (see [below for nested schema](#nestedblock--scan_rule--exceptions))
- `extra_settings` (Block List) Additional rule settings. (see [below for nested schema](#nestedblock--scan_rule--extra_settings))

<a id="nestedblock--scan_rule--exceptions"></a>
### Nested Schema for `scan_rule.exceptions`

Optional:

- `filter_tags` (Set of String) List of filter tags for exceptions.
- `resource_ids` (Set of String) List of resource IDs for exceptions.


<a id="nestedblock--scan_rule--extra_settings"></a>
### Nested Schema for `scan_rule.extra_settings`

Required:

- `name` (String) The name of the setting.
- `type` (String) The type of the setting. Allowed values: `multiple-string-values`, `multiple-object-values`, `choice-multiple-value`, `choice-single-value`, `countries`, `multiple-aws-account-values`, `multiple-ip-values`, `multiple-number-values`, `regions`, `ignored-regions`, `single-number-value`, `single-string-value`, `single-value-regex`, `ttl`, `multiple-vpc-gateway-mappings`, `tags`.

Optional:

- `value` (String) Single value for the setting. For numeric types (`ttl`, `single-number-value`, `multiple-number-values`), the value is automatically converted to a number.
- `value_set` (Set of String) Set of string values for simple types like multiple-string-values, multiple-ip-values, multiple-aws-account-values, multiple-number-values, regions, ignored-regions, tags, countries. For `multiple-number-values`, values are automatically converted to numbers.
- `values` (Block List) Multiple values for the setting. (see [below for nested schema](#nestedblock--scan_rule--extra_settings--values))

<a id="nestedblock--scan_rule--extra_settings--values"></a>
### Nested Schema for `scan_rule.extra_settings.values`

Optional:

- `enabled` (Boolean) Enabled value for the setting.
- `gateway_ids` (Set of String) List of gateway IDs (only for multiple-vpc-gateway-mappings type).
- `value` (String) Value for the setting. For `multiple-object-values` type, use JSON string (or `jsonencode` function). For numeric types, values are automatically converted to numbers.
- `vpc_id` (String) The VPC ID (only for multiple-vpc-gateway-mappings type).

## Import

Import is supported using the following syntax:

```shell
terraform import visionone_crm_profile.example <profile_id>
```
