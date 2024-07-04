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
}
