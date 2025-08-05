resource "visionone_container_policy" "example_policy" {
  name        = "LogOnlyPolicy"
  description = "A policy with several example logging rules. If linked to a cluster, it will generate events for enabled rule violations."
  default = {
    rules = [
      {
        action     = "log"
        mitigation = "log"
        type       = "podSecurityContext"
        enabled    = false
        statement = {
          properties = [
            {
              key   = "runAsNonRoot"
              value = "false"
            }
          ]
        }
      }
    ]
  }

  runtime = {
    rulesets = [
      {
        id = "LogOnlyRuleset-xxx"
      }
    ]
  }
  xdr_enabled = true

  malware_scan_mitigation = "log"
  malware_scan_enabled    = true
  malware_scan_schedule   = "0 0 * * *"

  secret_scan_mitigation              = "log"
  secret_scan_enabled                 = true
  secret_scan_schedule                = "0 0 * * *"
  secret_scan_skip_if_rule_not_change = true
  secret_scan_exclude_paths           = ["/safe_folder/*", "/folder?/*/config.json", "/folder/*/config.*"]
}
