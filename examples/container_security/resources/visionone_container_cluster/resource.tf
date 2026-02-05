resource "visionone_container_cluster" "example_cluster" {
  name                       = "example_cluster"
  description                = "This is a sample cluster"
  resource_id                = "arn:aws:eks:xxx:xxx:cluster/xxx"
  policy_id                  = "LogOnlyPolicy-xxx"
  group_id                   = "00000000-0000-0000-0000-000000000001"
  runtime_security_enabled   = true
  vulnerability_scan_enabled = true
  malware_scan_enabled       = true
  secret_scan_enabled        = true
  namespaces                 = ["kube-system"]
  customizable_tags          = [{ id = "tag-id-1" }, { id = "tag-id-2" }]
}