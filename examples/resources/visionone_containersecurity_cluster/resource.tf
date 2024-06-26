resource "visionone_containersecurity_cluster" "example_cluster" {
  name                       = "example_cluster"
  description                = "This is a sample cluster"
  resource_id                = "arn:aws:eks:xxx:xxx:cluster/xxx"
  policy_id                  = "LogOnlyPolicy-xxx"
  runtime_security_enabled   = true
  vulnerability_scan_enabled = true
  namespaces                 = ["kube-system"]
  proxy = {
    type          = "http"
    proxy_address = "192.168.0.1"
    port          = 8080
    username      = "user"
    password      = "password"
  }
}