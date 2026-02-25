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
        value    = jsonencode("Enabled")
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
