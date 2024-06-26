---
page_title: "visionone_containersecurity_policy Resource - policy"
subcategory: ""
description: |-
  The containersecurity_policy resource allows you to manage policies which define the rules that are used to control what is allowed to run in your Kubernetes cluster.
---

# visionone_containersecurity_policy (Resource)

The `containersecurity_policy` resource allows you to manage policies which define the rules that are used to control what is allowed to run in your Kubernetes cluster.

## Example Usage

```terraform
resource "visionone_containersecurity_policy" "example_policy" {
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
```

### Example Detailed Usage

<details>

```terraform
resource "visionone_containersecurity_policy" "example_policy" {
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
}
```

</details>

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `default` (Attributes) (see [below for nested schema](#nestedatt--default))
- `name` (String) A descriptive name for the policy.

### Optional

- `description` (String) A description of the policy.
- `namespaced` (Attributes List) The definition of all the policies. (see [below for nested schema](#nestedatt--namespaced))
- `runtime` (Attributes) The runtime properties of this policy. (see [below for nested schema](#nestedatt--runtime))
- `xdr_enabled` (Boolean) If true, enables XDR telemetry. Default is "true".Important: To use XDR telemetry, enable runtime security.

### Read-Only

- `created_date_time` (String)
- `id` (String) The unique ID assigned to this policy.
- `rulesets_updated_date_time` (String)
- `updated_date_time` (String)

<a id="nestedatt--default"></a>
### Nested Schema for `default`

Required:

- `rules` (Attributes List) The set of policy rules. The rules are OR together. (see [below for nested schema](#nestedatt--default--rules))

Optional:

- `exceptions` (Attributes List) The set of policy rules. The rules are OR together. (see [below for nested schema](#nestedatt--default--exceptions))

<a id="nestedatt--default--rules"></a>
### Nested Schema for `default.rules`

Required:

- `type` (String) The type of the policy rule.Enum: [podSecurityContext, containerSecurityContext, registry, image, tag, imagePath, vulnerabilities, cvssAttackVector, cvssAttackComplexity, cvssAvailability, checklists, checklistProfile, contents, malware, unscannedImage, podexec, portforward, capabilities].

Optional:

- `action` (String) Action to take when the rule fails during the admission control phase. Action is ignored in exceptions. It returns none if there is no record. Default is "none".Enum: [block, log, none].
- `enabled` (Boolean) Enable the rule. Default is "true".
- `mitigation` (String) Mitigation to take when the rule fails during runtime. Mitigation is ignored in exceptions. It returns none if there is no record.Default is "none".Enum: [log, isolate, terminate, none].
- `statement` (Attributes) (see [below for nested schema](#nestedatt--default--rules--statement))

<a id="nestedatt--default--rules--statement"></a>
### Nested Schema for `default.rules.statement`

Required:

- `properties` (Attributes List) (see [below for nested schema](#nestedatt--default--rules--statement--properties))

<a id="nestedatt--default--rules--statement--properties"></a>
### Nested Schema for `default.rules.statement.properties`

Required:

- `key` (String) See https://automation.trendmicro.com/xdr/api-v3#tag/Policies/paths/~1v3.0~1containerSecurity~1policies/post for more details.
- `value` (String)




<a id="nestedatt--default--exceptions"></a>
### Nested Schema for `default.exceptions`

Required:

- `type` (String) The type of the policy rule.Enum: [podSecurityContext, containerSecurityContext, registry, image, tag, imagePath, vulnerabilities, cvssAttackVector, cvssAttackComplexity, cvssAvailability, checklists, checklistProfile, contents, malware, unscannedImage, podexec, portforward, capabilities].

Optional:

- `action` (String) Action to take when the rule fails during the admission control phase. Action is ignored in exceptions. It returns none if there is no record. Default is "none".Enum: [block, log, none].
- `enabled` (Boolean) Enable the rule. Default is "true".
- `mitigation` (String) Mitigation to take when the rule fails during runtime. Mitigation is ignored in exceptions. It returns none if there is no record.Default is "none".Enum: [log, isolate, terminate, none].
- `statement` (Attributes) (see [below for nested schema](#nestedatt--default--exceptions--statement))

<a id="nestedatt--default--exceptions--statement"></a>
### Nested Schema for `default.exceptions.statement`

Required:

- `properties` (Attributes List) (see [below for nested schema](#nestedatt--default--exceptions--statement--properties))

<a id="nestedatt--default--exceptions--statement--properties"></a>
### Nested Schema for `default.exceptions.statement.properties`

Required:

- `key` (String) See https://automation.trendmicro.com/xdr/api-v3#tag/Policies/paths/~1v3.0~1containerSecurity~1policies/post for more details.
- `value` (String)





<a id="nestedatt--namespaced"></a>
### Nested Schema for `namespaced`

Required:

- `name` (String) Descriptive name for the namespaced policy definition.
- `namespaces` (List of String) The namespaces that are associated with this policy definition.
- `rules` (Attributes List) The set of policy rules. The rules are OR together. (see [below for nested schema](#nestedatt--namespaced--rules))

Optional:

- `exceptions` (Attributes List) The set of policy rules. The rules are OR together. (see [below for nested schema](#nestedatt--namespaced--exceptions))

<a id="nestedatt--namespaced--rules"></a>
### Nested Schema for `namespaced.rules`

Required:

- `type` (String) The type of the policy rule.Enum: [podSecurityContext, containerSecurityContext, registry, image, tag, imagePath, vulnerabilities, cvssAttackVector, cvssAttackComplexity, cvssAvailability, checklists, checklistProfile, contents, malware, unscannedImage, podexec, portforward, capabilities].

Optional:

- `action` (String) Action to take when the rule fails during the admission control phase. Action is ignored in exceptions. It returns none if there is no record. Default is "none".Enum: [block, log, none].
- `enabled` (Boolean) Enable the rule. Default is "true".
- `mitigation` (String) Mitigation to take when the rule fails during runtime. Mitigation is ignored in exceptions. It returns none if there is no record.Default is "none".Enum: [log, isolate, terminate, none].
- `statement` (Attributes) (see [below for nested schema](#nestedatt--namespaced--rules--statement))

<a id="nestedatt--namespaced--rules--statement"></a>
### Nested Schema for `namespaced.rules.statement`

Required:

- `properties` (Attributes List) (see [below for nested schema](#nestedatt--namespaced--rules--statement--properties))

<a id="nestedatt--namespaced--rules--statement--properties"></a>
### Nested Schema for `namespaced.rules.statement.properties`

Required:

- `key` (String) See https://automation.trendmicro.com/xdr/api-v3#tag/Policies/paths/~1v3.0~1containerSecurity~1policies/post for more details.
- `value` (String)




<a id="nestedatt--namespaced--exceptions"></a>
### Nested Schema for `namespaced.exceptions`

Required:

- `type` (String) The type of the policy rule.Enum: [podSecurityContext, containerSecurityContext, registry, image, tag, imagePath, vulnerabilities, cvssAttackVector, cvssAttackComplexity, cvssAvailability, checklists, checklistProfile, contents, malware, unscannedImage, podexec, portforward, capabilities].

Optional:

- `action` (String) Action to take when the rule fails during the admission control phase. Action is ignored in exceptions. It returns none if there is no record. Default is "none".Enum: [block, log, none].
- `enabled` (Boolean) Enable the rule. Default is "true".
- `mitigation` (String) Mitigation to take when the rule fails during runtime. Mitigation is ignored in exceptions. It returns none if there is no record.Default is "none".Enum: [log, isolate, terminate, none].
- `statement` (Attributes) (see [below for nested schema](#nestedatt--namespaced--exceptions--statement))

<a id="nestedatt--namespaced--exceptions--statement"></a>
### Nested Schema for `namespaced.exceptions.statement`

Required:

- `properties` (Attributes List) (see [below for nested schema](#nestedatt--namespaced--exceptions--statement--properties))

<a id="nestedatt--namespaced--exceptions--statement--properties"></a>
### Nested Schema for `namespaced.exceptions.statement.properties`

Required:

- `key` (String) See https://automation.trendmicro.com/xdr/api-v3#tag/Policies/paths/~1v3.0~1containerSecurity~1policies/post for more details.
- `value` (String)





<a id="nestedatt--runtime"></a>
### Nested Schema for `runtime`

Required:

- `rulesets` (Attributes List) The list of runtime rulesets associated to this policy. (see [below for nested schema](#nestedatt--runtime--rulesets))

<a id="nestedatt--runtime--rulesets"></a>
### Nested Schema for `runtime.rulesets`

Required:

- `id` (String) The ID of the ruleset

## Import

Import is supported using the following syntax:

```shell
terraform import visionone_containersecurity_policy.example_policy ${policy_id}
```