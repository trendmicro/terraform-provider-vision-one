---
page_title: "visionone_crm_custom_rule Resource - visionone"
subcategory: "Cloud Risk Management"
description: |-
  Manages a Cloud Risk Management custom rule.
---

# visionone_crm_custom_rule (Resource)

Manages a Cloud Risk Management custom rule.

## Example Usage

### Basic Custom Rule

```terraform
# Basic custom rule example with single attribute and event rule
resource "visionone_crm_custom_rule" "s3_versioning_check" {
  name                      = "s3-bucket-versioning-enabled"
  description               = "Ensure S3 buckets have versioning enabled for data protection"
  risk_level                = "HIGH"
  cloud_provider            = "aws"
  service                   = "S3"
  resource_type             = "s3-bucket"
  enabled                   = true
  categories                = ["security", "reliability"]
  remediation_note          = "Enable versioning on the S3 bucket to protect against accidental deletion or modification. Use AWS Console or CLI: aws s3api put-bucket-versioning --bucket BUCKET_NAME --versioning-configuration Status=Enabled"
  resolution_reference_link = "https://docs.aws.amazon.com/AmazonS3/latest/userguide/Versioning.html"
  slug                      = "s3-bucket-versioning-check-001"

  # Define the attribute to evaluate
  attribute {
    name     = "bucketVersioning"
    path     = "data.BucketVersioning"
    required = true
  }

  # Define the event rule with conditions
  event_rule {
    description = "Check if bucket versioning status is enabled"

    conditions {
      operator = "all"

      condition {
        operator = "equal"
        fact     = "bucketVersioning"
        path     = "$.Status"
        value    = "Enabled"
      }
    }
  }
}

output "custom_rule_s3_versioning_id" {
  value       = visionone_crm_custom_rule.s3_versioning_check.id
  description = "The ID of the S3 versioning custom rule"
}

output "custom_rule_s3_versioning_slug" {
  value       = visionone_crm_custom_rule.s3_versioning_check.slug
  description = "The slug of the S3 versioning custom rule"
}
```

### Multiple Attributes and Event Rules

```terraform
# Example with multiple attributes and multiple event rules
resource "visionone_crm_custom_rule" "s3_comprehensive_security" {
  name             = "s3-bucket-comprehensive-security"
  description      = "Comprehensive security checks for S3 buckets including versioning and public access"
  risk_level       = "VERY_HIGH"
  cloud_provider   = "aws"
  service          = "S3"
  resource_type    = "s3-bucket"
  enabled          = true
  categories       = ["security", "operational-excellence"]
  remediation_note = "Enable versioning and block all public access on the S3 bucket"
  slug             = "s3-comprehensive-security-001"

  # Multiple attributes to evaluate
  attribute {
    name     = "bucketVersioning"
    path     = "data.BucketVersioning"
    required = true
  }

  attribute {
    name     = "bucketPublicAccess"
    path     = "data.PublicAccessBlockConfiguration"
    required = true
  }

  # Event rule checking versioning
  event_rule {
    description = "Verify versioning is enabled"

    conditions {
      operator = "all"

      condition {
        operator = "equal"
        fact     = "bucketVersioning"
        path     = "$.Status"
        value    = "Enabled"
      }
    }
  }

  # Event rule checking public access is blocked
  event_rule {
    description = "Ensure all public access is blocked"

    conditions {
      operator = "all"

      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.BlockPublicAcls"
        value    = "true"
      }

      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.BlockPublicPolicy"
        value    = "true"
      }

      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.IgnorePublicAcls"
        value    = "true"
      }

      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.RestrictPublicBuckets"
        value    = "true"
      }
    }
  }
}
```

### Using "Any" Operator

```terraform
# Example with "any" operator - at least one condition must match
resource "visionone_crm_custom_rule" "s3_encryption_any_method" {
  name             = "s3-bucket-encryption-any-method"
  description      = "Ensure S3 bucket uses any form of encryption (AES256 or KMS)"
  risk_level       = "HIGH"
  cloud_provider   = "aws"
  service          = "S3"
  resource_type    = "s3-bucket"
  enabled          = true
  categories       = ["security"]
  remediation_note = "Enable server-side encryption on the S3 bucket using either AES256 or AWS KMS"
  slug             = "s3-encryption-any-method-001"

  attribute {
    name     = "bucketEncryption"
    path     = "data.BucketEncryption"
    required = true
  }

  event_rule {
    description = "Check if any encryption method is enabled"

    conditions {
      operator = "any" # At least one condition must be true

      condition {
        operator = "equal"
        fact     = "bucketEncryption"
        path     = "$.ServerSideEncryptionConfiguration.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm"
        value    = "AES256"
      }

      condition {
        operator = "equal"
        fact     = "bucketEncryption"
        path     = "$.ServerSideEncryptionConfiguration.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm"
        value    = "aws:kms"
      }
    }
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `categories` (List of String) Categories of the custom rule. Allowed values: security, cost-optimisation, reliability, performance-efficiency, operational-excellence, sustainability.
- `cloud_provider` (String) The cloud provider. Allowed values: aws, azure, gcp, oci, alibabaCloud
- `description` (String) The custom rule description (max 255 characters).
- `enabled` (Boolean) Whether the rule is enabled or not.
- `name` (String) The custom rule name (max 255 characters).
- `resource_type` (String) The type of resource this custom rule applies to (max 100 characters).
- `risk_level` (String) The risk level. Allowed values: LOW, MEDIUM, HIGH, VERY_HIGH, EXTREME.
- `service` (String) The cloud service ID.
- `attribute` (Block List) The attributes of the resource data to be evaluated. (see [below for nested schema](#nestedblock--attribute))
- `event_rule` (Block List) The events to be evaluated by the custom rule. (see [below for nested schema](#nestedblock--event_rule))

### Optional

- `remediation_note` (String) The remediation notes for the custom rule (max 1000 characters).
- `resolution_reference_link` (String) A reference link for resolution guidance.
- `slug` (String) The slug of the custom rule. The system uses the slug to form the rule ID (max 200 characters).

### Read-Only

- `id` (String) The unique ID of the custom rule.

<a id="nestedblock--attribute"></a>
### Nested Schema for `attribute`

Required:

- `name` (String) The name of the attribute.
- `path` (String) The path to the attribute in the resource data.
- `required` (Boolean) Whether this attribute is required.


<a id="nestedblock--event_rule"></a>
### Nested Schema for `event_rule`

Required:

- `description` (String) The description of the event rule.
- `conditions` (Block, Optional) The conditions for event evaluation. (see [below for nested schema](#nestedblock--event_rule--conditions))

<a id="nestedblock--event_rule--conditions"></a>
### Nested Schema for `event_rule.conditions`

Optional:

- `condition` (Block List) List of conditions to evaluate. (see [below for nested schema](#nestedblock--event_rule--conditions--condition))
- `operator` (String) Logical operator. Allowed values: all, any.

<a id="nestedblock--event_rule--conditions--condition"></a>
### Nested Schema for `event_rule.conditions.condition`

Required:

- `operator` (String) Comparison operator.
- `fact` (String) The fact name for event rule conditions.
- `value` (String) The value to compare against (JSON encoded).

Optional:

- `path` (String) The path for evaluation.

## Import

Import is supported using the following syntax:

```shell
terraform import visionone_crm_custom_rule.example <custom_rule_id>
```
