resource "visionone_container_cluster" "example_cluster" {
  name                       = "example_cluster"
  description                = "This is a sample cluster"
  resource_id                = "arn:aws:eks:xxx:xxx:cluster/xxx"
  policy_id                  = "LogOnlyPolicy-xxx"
  group_id                   = "00000000-0000-0000-0000-000000000000"
  runtime_security_enabled   = true
  vulnerability_scan_enabled = true
  namespaces                 = ["kube-system"]
}