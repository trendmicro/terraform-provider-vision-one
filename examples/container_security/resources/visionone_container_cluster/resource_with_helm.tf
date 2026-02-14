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
}