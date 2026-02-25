# Example: Custom Role Definition with Custom Permissions
# This example shows how to override the default Vision One CAM permissions
# with custom actions and data_actions

resource "visionone_cam_role_definition" "custom_permissions" {
  subscription_id = "11111111-1111-2222-aaaa-bbbbbbbbbbbb"

  # Override default actions with custom permissions
  # If not provided, defaults to standard Vision One CAM required permissions
  actions = [
    "Microsoft.Resources/subscriptions/read",
    "Microsoft.Storage/storageAccounts/read",
    "Microsoft.Storage/storageAccounts/listKeys/action",
    "Microsoft.ContainerRegistry/registries/read",
    "Microsoft.KeyVault/vaults/secrets/read"
  ]

  # Override default data actions with custom data permissions
  # If not provided, defaults to standard Vision One CAM required data permissions
  data_actions = [
    "Microsoft.KeyVault/vaults/secrets/getSecret/action",
    "Microsoft.KeyVault/vaults/keys/read",
    "Microsoft.KeyVault/vaults/secrets/readMetadata/action"
  ]
}

# Example: Partial Override - Only Custom Actions
# You can override just actions or just data_actions
resource "visionone_cam_role_definition" "custom_actions_only" {
  subscription_id = "11111111-1111-2222-aaaa-bbbbbbbbbbbb"

  # Custom actions, but data_actions will use defaults
  actions = [
    "*/read",
    "Microsoft.Storage/storageAccounts/listKeys/action"
  ]
}
