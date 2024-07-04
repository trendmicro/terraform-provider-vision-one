resource "visionone_container_cluster" "example_cluster" {
  #...
  proxy = {
    type          = "http"
    proxy_address = "192.168.0.1"
    port          = 8080
    username      = "user"
    password      = "password"
  }
}


resource "helm_release" "trendmicro" {
  #...
  set {
    name  = "proxy.httpsProxy"
    value = visionone_container_cluster.example_cluster.proxy.https_proxy
  }
  set {
    name  = "proxy.username"
    value = visionone_container_cluster.example_cluster.proxy.username
  }
  set {
    name  = "proxy.password"
    value = visionone_container_cluster.example_cluster.proxy.password
  }
}