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
        value    = jsonencode("AES256")
      }

      condition {
        operator = "equal"
        fact     = "bucketEncryption"
        path     = "$.ServerSideEncryptionConfiguration.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm"
        value    = jsonencode("aws:kms")
      }
    }
  }
}
