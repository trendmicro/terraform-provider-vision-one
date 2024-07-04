# VisionOne Terraform Provider

Framework: https://developer.hashicorp.com/terraform/plugin/framework

## Local Development Setup

### For Mac User
create .terraformrc file under your home dire(~)

### 1. Setup $GOBIN
   Verify with

```shell
go env GOBIN
```

make sure you have the path ready.
If nothing setup GOBIN with default /Users/<Username>/go/bin

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

### 2. Compile Provider Code

```shell
go install .
```

The binary executive file will store at your $GOBIN

### 3. Verify with Terraform
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