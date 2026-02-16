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
