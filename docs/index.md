---
page_title: "Provider: Vision One"
description: |-
  Introduction of Vision One Provider.
---

# Vision One Provider

The Vision One provider is a plugin for Terraform that allows for the full lifecycle management of Vision One resources. This provider is maintained internally by the Vision One team.

To use the Vision One provider, you need to provide your API key and regional FQDN. You can do this by setting the `api_key` and `regional_fqdn` variables in your Terraform configuration file:

## Example Usage

```terraform
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
```

## Schema

### Required

- `api_key` (String) This is the API key for your Vision One account. The API key is a unique identifier for authenticating your account. Keep this key confidential to protect your account from unauthorized access, so tread this key as sensitive information. Generate the API key in your Vision One account settings or using the `VISIONONE_API_KEY` environment variable. For more information on the API key, see the [API Key Guide](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-__api-keys-2#GUID-E88BBD1F-EA82-4490-9C7F-E141E3BEE8F4-4).

- `regional_fqdn` (String) This is the regional Fully Qualified Domain Name (FQDN) to call the API in the backend. Get this FQDN using the `VISIONONE_REGIONAL_FQDN` environment variable. For a full list of FQDNs, see the [Regional Domains Guide](https://automation.trendmicro.com/xdr/Guides/Regional-domains/).

## Bugs and Issues

If you find an issue, open an issue in the [GitHub Repository](https://github.com/trendmicro/terraform-provider-vision-one/issues).

## Further Reading

For more information about the Vision One provider, see the [API Reference](https://automation.trendmicro.com/xdr/api-v3#).
