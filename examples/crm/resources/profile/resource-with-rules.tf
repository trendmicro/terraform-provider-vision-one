# This example demonstrates all extra_settings types supported by the CRM profile resource.
#
# NOTE: For simple list types, you can use either:
#   - `value_set` (recommended): A simple string array, e.g., value_set = ["value1", "value2"]
#   - `values` block: The traditional block syntax with nested value attributes
#
# Types that support `value_set`:
#   - multiple-string-values, multiple-ip-values, multiple-aws-account-values
#   - multiple-number-values (use strings, they are converted to numbers automatically)
#   - regions, ignored-regions, tags, countries
#
# Types that require `values` block (because they need `enabled` or other fields):
#   - choice-multiple-value, multiple-object-values

resource "visionone_crm_profile" "profile_with_rules" {
  name        = "crm-profile-with-rules"
  description = "Cloud Risk Management profile - rules included"

  # Rule without extra settings
  scan_rule {
    id         = "EC2-001"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"
  }

  # Rule with exceptions (filter_tags)
  scan_rule {
    id         = "EC2-034"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"
    exceptions {
      filter_tags = ["ignore_this_tag"]
      resource_ids = []
    }
    # Type: single-value-regex
    extra_settings {
      name  = "SecurityGroupsSafelistNamePattern"
      type  = "single-value-regex"
      value = "updated.*"
    }
    # Type: multiple-number-values (using value_set - values are strings, converted to numbers)
    extra_settings {
      name      = "commonlyUsedPorts"
      type      = "multiple-number-values"
      value_set = [80, 443, 20, 81, 22, 44]
    }
  }

  # Type: ttl
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

  # Type: multiple-aws-account-values and tags 
  scan_rule {
    id         = "SNS-002"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"
    exceptions {
      filter_tags = ["ignore_this_tag"]
      resource_ids = []
    }
    # Type: multiple-aws-account-values (using value_set)
    extra_settings {
      name      = "friendlyAccounts"
      type      = "multiple-aws-account-values"
      value_set = ["123456789012"]
    }

    # Type: tags (using value_set with underscore format)
    extra_settings {
      name      = "accountTags"
      type      = "tags"
      value_set = ["env_prod", "team_devops"]
    }
  }

  # Type: choice-single-value (Azure provider)
  scan_rule {
    id         = "Locks-001"
    provider   = "azure"
    enabled    = false
    risk_level = "LOW"
    extra_settings {
      name  = "level"
      type  = "choice-single-value"
      value = "CanNotDelete"
    }
  }

  # Type: multiple-ip-values (using value_set)
  scan_rule {
    id         = "RTM-007"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"
    extra_settings {
      name      = "authorisedIps"
      type      = "multiple-ip-values"
      value_set = ["10.0.0.0", "10.0.0.3"]
    }
    extra_settings {
      name  = "ttl"
      type  = "ttl"
      value = 23
    }
  }

  # Type: regions (using value_set)
  scan_rule {
    id         = "RTM-008"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"
    extra_settings {
      name      = "authorisedRegions"
      type      = "regions"
      value_set = ["us-east-1", "us-west-2"]
    }
    extra_settings {
      name  = "ttl"
      type  = "ttl"
      value = 24
    }
  }

  # Type: ignored-regions (using value_set)
  scan_rule {
    id         = "Config-001"
    provider   = "aws"
    enabled    = true
    risk_level = "HIGH"
    extra_settings {
      name      = "ignoredRegions"
      type      = "ignored-regions"
      value_set = ["eu-west-1"]
    }
  }

  # Type: countries (using value_set)
  scan_rule {
    id         = "RTM-005"
    provider   = "aws"
    enabled    = true
    risk_level = "LOW"
    extra_settings {
      name      = "authorisedCountries"
      type      = "countries"
      value_set = ["US", "AU", "NZ"]
    }
  }

  # Type: multiple-object-values with jsonencode (requires values block)
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
          eventName        = "updated-event"
          userIdentityType = "test"
        })
      }
    }
    extra_settings {
      name  = "ttl"
      type  = "ttl"
      value = 48
    }
  }

  # Type: single-number-value (AlibabaCloud provider)
  scan_rule {
    id         = "AlibabaCloud-ACK-002"
    provider   = "alibabaCloud"
    enabled    = false
    risk_level = "LOW"
    extra_settings {
      name  = "periodInDays"
      type  = "single-number-value"
      value = 60
    }
  }

  # Type: single-string-value
  scan_rule {
    id         = "ASG-013"
    provider   = "aws"
    enabled    = true
    risk_level = "MEDIUM"
    extra_settings {
      name  = "tierTag"
      type  = "single-string-value"
      value = "test1"
    }
    extra_settings {
      name  = "tierTagValue"
      type  = "single-string-value"
      value = "updated"
    }
  }

  # Type: multiple-vpc-gateway-mappings (using values block with vpc_id and gateway_ids)
  scan_rule {
    id         = "VPC-013"
    provider   = "aws"
    enabled    = true
    risk_level = "LOW"
    extra_settings {
      name = "SpecificVPCToSpecificGatewayMapping"
      type = "multiple-vpc-gateway-mappings"
      values {
        vpc_id      = "vpc-updated123"
        gateway_ids = ["igw-updated456", "vgw-updated789"]
      }
      values {
        vpc_id      = "vpc-updated456"
        gateway_ids = ["igw-updated789"]
      }
    }
  }

  # Type: multiple-string-values (using value_set) settings
  scan_rule {
    id         = "RG-001"
    provider   = "aws"
    enabled    = true
    risk_level = "LOW"
    # Type: multiple-string-values (using value_set)
    extra_settings {
      name      = "tags"
      type      = "multiple-string-values"
      value_set = ["Environment", "UpdatedRole"]
    }
    # Type: choice-multiple-value with settings hasn't been supported yet
  }

  # Type: choice-multiple-value (GCP provider) - requires values block
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

output "profile_with_rules" {
  value = visionone_crm_profile.profile_with_rules
}
