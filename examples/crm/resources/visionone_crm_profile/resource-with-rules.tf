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

  # Type: choice-multiple-value-with-tags
  scan_rule {
    id         = "RG-001"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"

    extra_settings {
      name = "resourceTypes"
      type = "choice-multiple-value-with-tags"

      values {
        value           = "apigateway-restapi"
        enabled         = true
        customized_tags = ["production", "web-server"]
      }

      values {
        value           = "apigateway-stage"
        enabled         = true
        customized_tags = ["production", "apigw-stage"]
      }
    }
  }

  # Type: choice-multiple-value-with-risk-level
  scan_rule {
    id         = "IAM-054"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"

    extra_settings {
      name = "ConfigurationChanges"
      type = "choice-multiple-value-with-risk-level"

      values {
        value                 = "CreateLoginProfile"
        enabled               = true
        customized_risk_level = "MEDIUM"
      }

      values {
        value                 = "AddUserToGroup"
        enabled               = true
        customized_risk_level = "HIGH"
      }

      values {
        value                 = "AttachUserPolicy"
        enabled               = true
        customized_risk_level = "NOT_CUSTOMIZED"
      }
    }
  }
}
