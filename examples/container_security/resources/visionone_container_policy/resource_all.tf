resource "visionone_container_policy" "example_policy" {
  name        = "LogOnlyPolicy"
  description = "A policy with several example logging rules. If linked to a cluster, it will generate events for enabled rule violations."

  default = {
    rules = [
      // Pod properties
      {
        action     = "log"
        mitigation = "log"
        type       = "podSecurityContext"
        statement = {
          properties = [
            {
              key   = "hostNetwork"
              value = "true"
            }
          ]
        }
        enabled = true
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "podSecurityContext"
        statement = {
          properties = [
            {
              key   = "hostIPC"
              value = "true"
            }
          ]
        }
        enabled = true
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "podSecurityContext"
        statement = {
          properties = [
            {
              key   = "hostPID"
              value = "true"
            }
          ]
        }
        enabled = true
      },
      // Container properties
      {
        action     = "log"
        mitigation = "log"
        type       = "containerSecurityContext"
        statement = {
          properties = [
            {
              key   = "runAsNonRoot"
              value = "false"
            }
          ]
        }
        enabled = true
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "containerSecurityContext"
        statement = {
          properties = [
            {
              key   = "privileged"
              value = "true"
            }
          ]
        }
        enabled = true
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "containerSecurityContext"
        statement = {
          properties = [
            {
              key   = "allowPrivilegeEscalation"
              value = "true"
            }
          ]
        }
        enabled = true
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "containerSecurityContext"
        statement = {
          properties = [
            {
              key   = "readOnlyRootFilesystem"
              value = "false"
            }
          ]
        }
        enabled = true
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "containerSecurityContext"
        statement = {
          properties = [
            {
              key   = "capabilities-rule"
              value = "restrict-nondefaults"
            }
          ]
        }
        enabled = true
      },
      // Image properties
      {
        action     = "log"
        mitigation = "log"
        type       = "registry"
        statement = {
          properties = [
            {
              key   = "equals"
              value = "imageRegistryValue"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "image"
        statement = {
          properties = [
            {
              key   = "equals"
              value = "imageNameValue"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "tag"
        statement = {
          properties = [
            {
              key   = "equals"
              value = "imageTagValue"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "imagePath"
        statement = {
          properties = [
            {
              key   = "equals"
              value = "imagePathValue"
            }
          ]
        }
        enabled = false
      },
      // Artifact Scanner Scan results
      {
        action     = "log"
        mitigation = "log"
        type       = "unscannedImage"
        enabled    = true
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "malware"
        statement = {
          properties = [
            {
              key   = "count"
              value = "0"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "vulnerabilities"
        statement = {
          properties = [
            {
              key   = "max-severity"
              value = "high"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "cvssAttackVector"
        statement = {
          properties = [
            {
              key   = "cvss-attack-vector"
              value = "network"
            },
            {
              key   = "max-severity"
              value = "high"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "cvssAttackComplexity"
        statement = {
          properties = [
            {
              key   = "cvss-attack-complexity"
              value = "high"
            },
            {
              key   = "max-severity"
              value = "high"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "cvssAvailability"
        statement = {
          properties = [
            {
              key   = "cvss-availability"
              value = "high"
            },
            {
              key   = "max-severity"
              value = "high"
            }
          ]
        }
        enabled = false
      },
      // Kubectl Access
      {
        action     = "log"
        mitigation = "log"
        type       = "podexec"
        statement = {
          properties = [
            {
              key   = "podExec"
              value = "true"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "portforward"
        statement = {
          properties = [
            {
              key   = "podPortForward"
              value = "true"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "unscannedImageMalware"
        statement = {
          properties = [
            {
              key   = "days"
              value = "30"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "block"
        mitigation = "log"
        type       = "unscannedImageSecret"
        statement = {
          properties = [
            {
              key   = "days"
              value = "7"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "block"
        mitigation = "log"
        type       = "secrets"
        statement = {
          properties = [
            {
              key   = "count"
              value = "0"
            }
          ]
        }
        enabled = false
      }
    ]

    exceptions = [
      {
        action     = "log"
        mitigation = "log"
        type       = "registry"
        statement = {
          properties = [
            {
              key   = "equals"
              value = "registryValue"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "image"
        statement = {
          properties = [
            {
              key   = "equals"
              value = "nameValue"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "tag"
        statement = {
          properties = [
            {
              key   = "equals"
              value = "tagValue"
            }
          ]
        }
        enabled = false
      },
      {
        action     = "log"
        mitigation = "log"
        type       = "imagePath"
        statement = {
          properties = [
            {
              key   = "equals"
              value = "pathValue"
            }
          ]
        }
        enabled = false
      }
    ]
  }

  namespaced = [
    {
      name = "NamespacedPolicyDefinition"

      namespaces = ["example-namespaces-dev", "example-namespaces-prod"]

      rules = [
        // Pod properties
        {
          action     = "log"
          mitigation = "log"
          type       = "podSecurityContext"
          statement = {
            properties = [
              {
                key   = "hostNetwork"
                value = "true"
              }
            ]
          }
          enabled = true
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "podSecurityContext"
          statement = {
            properties = [
              {
                key   = "hostIPC"
                value = "true"
              }
            ]
          }
          enabled = true
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "podSecurityContext"
          statement = {
            properties = [
              {
                key   = "hostPID"
                value = "true"
              }
            ]
          }
          enabled = true
        },
        // Container properties
        {
          action     = "log"
          mitigation = "log"
          type       = "containerSecurityContext"
          statement = {
            properties = [
              {
                key   = "runAsNonRoot"
                value = "false"
              }
            ]
          }
          enabled = true
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "containerSecurityContext"
          statement = {
            properties = [
              {
                key   = "privileged"
                value = "true"
              }
            ]
          }
          enabled = true
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "containerSecurityContext"
          statement = {
            properties = [
              {
                key   = "allowPrivilegeEscalation"
                value = "true"
              }
            ]
          }
          enabled = true
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "containerSecurityContext"
          statement = {
            properties = [
              {
                key   = "readOnlyRootFilesystem"
                value = "false"
              }
            ]
          }
          enabled = true
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "containerSecurityContext"
          statement = {
            properties = [
              {
                key   = "capabilities-rule"
                value = "restrict-nondefaults"
              }
            ]
          }
          enabled = true
        },
        // Image properties
        {
          action     = "log"
          mitigation = "log"
          type       = "registry"
          statement = {
            properties = [
              {
                key   = "equals"
                value = "imageRegistryValue"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "image"
          statement = {
            properties = [
              {
                key   = "equals"
                value = "imageNameValue"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "tag"
          statement = {
            properties = [
              {
                key   = "equals"
                value = "imageTagValue"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "imagePath"
          statement = {
            properties = [
              {
                key   = "equals"
                value = "imagePathValue"
              }
            ]
          }
          enabled = false
        },
        // Artifact Scanner Scan results
        {
          action     = "log"
          mitigation = "log"
          type       = "unscannedImage"
          enabled    = true
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "malware"
          statement = {
            properties = [
              {
                key   = "count"
                value = "0"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "vulnerabilities"
          statement = {
            properties = [
              {
                key   = "max-severity"
                value = "high"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "cvssAttackVector"
          statement = {
            properties = [
              {
                key   = "cvss-attack-vector"
                value = "network"
              },
              {
                key   = "max-severity"
                value = "high"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "cvssAttackComplexity"
          statement = {
            properties = [
              {
                key   = "cvss-attack-complexity"
                value = "high"
              },
              {
                key   = "max-severity"
                value = "high"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "cvssAvailability"
          statement = {
            properties = [
              {
                key   = "cvss-availability"
                value = "high"
              },
              {
                key   = "max-severity"
                value = "high"
              }
            ]
          }
          enabled = false
        },
        // Kubectl Access
        {
          action     = "log"
          mitigation = "log"
          type       = "podexec"
          statement = {
            properties = [
              {
                key   = "podExec"
                value = "true"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "portforward"
          statement = {
            properties = [
              {
                key   = "podPortForward"
                value = "true"
              }
            ]
          }
          enabled = false
        }
      ]

      exceptions = [
        {
          action     = "log"
          mitigation = "log"
          type       = "registry"
          statement = {
            properties = [
              {
                key   = "equals"
                value = "registryValue"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "image"
          statement = {
            properties = [
              {
                key   = "equals"
                value = "nameValue"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "tag"
          statement = {
            properties = [
              {
                key   = "equals"
                value = "tagValue"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "imagePath"
          statement = {
            properties = [
              {
                key   = "equals"
                value = "pathValue"
              }
            ]
          }
          enabled = false
        },
        {
          action     = "log"
          mitigation = "log"
          type       = "unscannedImageMalware"
          statement = {
            properties = [
              {
                key   = "days"
                value = "30"
              }
            ]
          }
          enabled = false
        }
      ]
    }
  ]

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
