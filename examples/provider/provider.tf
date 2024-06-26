terraform {
  required_providers {
    visionone = {
      source  = "trendmicro/vision-one"
      version = "~> 1.0"
    }
  }
}

provider "visionone" {
  api_key       = "<your-api-key>"
  regional_fqdn = "<your-regional-fqdn>"
}