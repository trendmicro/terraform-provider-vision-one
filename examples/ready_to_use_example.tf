terraform {
  required_providers {
    visionone = {
      source  = "trendmicro/vision-one"
      version = "~> 1.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.22"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.10"
    }
  }
}

provider "kubernetes" {
  config_path = "~/.kube/config" # or specify other kubeconfig path
  # config_context = "my-context"    # if using context, you can specify it here
}

provider "helm" {
  kubernetes {
    config_path = "~/.kube/config" # or specify other kubeconfig path
  }
}

provider "visionone" {
  api_key       = "<your-api-key>" # get your api key
  regional_fqdn = "<your-regional-fqdn>" # get your fqdn
}


resource "visionone_container_cluster" "example_cluster" {
  name                       = "user_eks_fargat"
  description                = "terraform create cluster"
  group_id                   = "00000000-0000-0000-0000-000000000001"      # change to yours
  policy_id                  = "LogOnlyPolicy-2VkJTQrRM9cBtQlZcNZemHJgrxu" # change to yours
  runtime_security_enabled   = true
  vulnerability_scan_enabled = true
  malware_scan_enabled       = true
  secret_scan_enabled        = true
  namespaces                 = ["kube-system"]
  customizable_tags          = [{ id = "tag-id-1" }, { id = "tag-id-2" }] # change to yours
}

resource "helm_release" "trendmicro" {
  name             = "trendmicro"
  chart            = "https://github.com/trendmicro/visionone-container-security-helm/archive/main.tar.gz"
  namespace        = "trendmicro-system"
  create_namespace = true
  wait             = false

  set {
    name  = "visionOne.bootstrapToken"
    value = visionone_container_cluster.example_cluster.api_key
  }
  set {
    name  = "visionOne.endpoint"
    value = visionone_container_cluster.example_cluster.endpoint
  }
  set_list {
    name  = "visionOne.exclusion.namespaces"
    value = visionone_container_cluster.example_cluster.namespaces
  }
  set {
    name  = "visionOne.runtimeSecurity.enabled"
    value = visionone_container_cluster.example_cluster.runtime_security_enabled
  }
  set {
    name  = "visionOne.vulnerabilityScanning.enabled"
    value = visionone_container_cluster.example_cluster.vulnerability_scan_enabled
  }
  set {
    name  = "visionOne.malwareScanning.enabled"
    value = visionone_container_cluster.example_cluster.malware_scan_enabled
  }
  set {
    name  = "visionOne.secretScanning.enabled"
    value = visionone_container_cluster.example_cluster.secret_scan_enabled
  }
  set {
    name  = "visionOne.inventoryCollection.enabled"
    value = visionone_container_cluster.example_cluster.inventory_collection
  }
  set {
    name  = "logConfig.logLevel"
    value = "debug"
  }
  # set {
  #  name = "visionOne.fargateInjector.enabled"
  #  value = "true"
  #}
}

