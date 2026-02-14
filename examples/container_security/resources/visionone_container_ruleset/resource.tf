resource "visionone_container_ruleset" "example_ruleset" {
  name        = "LogOnlyRuleset"
  description = "A policy with several example logging rules. If linked to a cluster, it will generate events for enabled rule violations."
  labels = [{
    key   = "app"
    value = "nginx"
  }]

  rules = [
    {
      id         = "TM-00000006"
      mitigation = "log"
    },
    {
      id         = "TM-00000010"
      mitigation = "log"
    },
    {
      id         = "TM-00000023"
      mitigation = "log"
    },
    {
      id         = "TM-00000031"
      mitigation = "log"
    },
    {
      id         = "TM-00000032"
      mitigation = "log"
    }
  ]
}