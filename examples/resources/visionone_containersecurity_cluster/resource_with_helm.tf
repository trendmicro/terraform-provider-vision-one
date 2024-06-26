resource "helm_release" "trendmicro" {
  name             = "trendmicro"
  chart            = "https://github.com/trendmicro/cloudone-container-security-helm/archive/master.tar.gz"
  namespace        = "trendmicro-system"
  create_namespace = true

  set {
    name  = "cloudOne.apiKey"
    value = visionone_containersecurity_cluster.example_cluster.api_key
  }
  set {
    name  = "cloudOne.endpoint"
    value = visionone_containersecurity_cluster.example_cluster.endpoint
  }
  set_list {
    name  = "cloudOne.exclusion.namespaces"
    value = visionone_containersecurity_cluster.example_cluster.namespaces
  }
  set {
    name  = "cloudOne.runtimeSecurity.enabled"
    value = visionone_containersecurity_cluster.example_cluster.runtime_security_enabled
  }
  set {
    name  = "cloudOne.vulnerabilityScanning.enabled"
    value = visionone_containersecurity_cluster.example_cluster.vulnerability_scan_enabled
  }
  set {
    name  = "cloudOne.inventoryCollection.enabled"
    value = visionone_containersecurity_cluster.example_cluster.inventory_collection
  }
  set {
    name  = "proxy.httpsProxy"
    value = visionone_containersecurity_cluster.example_cluster.proxy.https_proxy
  }
  set {
    name  = "proxy.username"
    value = visionone_containersecurity_cluster.example_cluster.proxy.username
  }
  set {
    name  = "proxy.password"
    value = visionone_containersecurity_cluster.example_cluster.proxy.password
  }
}