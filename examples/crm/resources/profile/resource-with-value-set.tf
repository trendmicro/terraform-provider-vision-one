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
