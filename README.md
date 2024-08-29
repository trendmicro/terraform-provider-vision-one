# Trend Micro Vision One Terraform Provider

Framework: https://developer.hashicorp.com/terraform/plugin/framework

Our Terraform Provider: https://registry.terraform.io/providers/trendmicro/vision-one/latest

## Local Development Setup

### For Mac User
create .terraformrc file under your home directory(~)

### 1. Setup $GOBIN
   Verify with

```shell
go env GOBIN
```

make sure you have the path ready.
If nothing setup GOBIN with default /Users/YOUR_USERNAME/go/bin

### 2. Overrides local provider

check your provider installation setting in ~/.terraformrc

```shell
cat ~/.terraformrc
```

```shell
provider_installation {

  dev_overrides {
      "trendmicro/vision-one" = "<FILL_IN_WITH_$GOBIN_PATH>"

  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

### 3. Compile Provider Code

```shell
go install .
```

The binary executive file will store at your $GOBIN

### 4. Verify with Terraform
Either find sample code under example folder or make your own

```terraform
terraform {
  required_providers {
    visionone = {
      source = "trendmicro/vision-one"
    }
  }
}

provider "visionone" {
  api_key         = "xxx"
  regional_fqdn   = "xxx"
}

resource "visionone_container_cluster" "this" {
  name = "example"

}
```

## Use Example

### 1. Navigate to example folder. Use ruleset for example.

```
cd examples/resources/visionone_container_ruleset
```

### 2. Copy provider settings.

Copy the provider settings from `examples/provider/provider.tf`, fill in your API key and regional fully qualified domain name (FQDN), then paste in `examples/resources/visionone_container_ruleset/resource.tf`.

For the API key, add in the Vision One console.

For the regional FQDN, refer to the [Regional domains table](https://automation.trendmicro.com/xdr/Guides/Regional-domains).

### 3. Good to go!

Use the terraform command to fetch the latest terraform provider and build your resource.
```
terraform init
terraform apply
```

