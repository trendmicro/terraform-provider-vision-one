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

  # Event rule checking different `value` types
  event_rule {
    description = "Ensure all public access is blocked"

    conditions {
      operator = "all"

      # Event rule checking `string` type
      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.BlockPublicAcls"
        value    = jsonencode("string")
      }

      # Event rule checking `number` type
      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.BlockPublicPolicy"
        value    = jsonencode(123)
      }

      # Event rule checking `boolean` type
      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.IgnorePublicAcls"
        value    = jsonencode(true)
      }

      # Event rule checking `object` type
      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.RestrictPublicBuckets"
        value    = jsonencode({ days = 7, operator = "within" })
      }

      # Event rule checking `array of numbers` type
      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.RestrictPublicBuckets"
        value    = jsonencode([1,2,3])
      }

      # Event rule checking `array of strings` type
      condition {
        operator = "equal"
        fact     = "bucketPublicAccess"
        path     = "$.RestrictPublicBuckets"
        value    = jsonencode(["one", "two"])
      }
    }
  }
}
