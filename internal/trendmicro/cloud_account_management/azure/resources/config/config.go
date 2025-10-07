package config

const (
	RESOURCE_TYPE_APP_REGISTRATION            = "cam_app_registration"
	RESOURCE_TYPE_SERVICE_PRINCIPAL           = "cam_service_principal"
	RESOURCE_TYPE_FEDERATED_IDENTITY          = "cam_federated_identity"
	RESOURCE_TYPE_CONNECTOR_AZURE             = "cam_connector_azure"
	RESOURCE_TYPE_CONNECTOR_AZURE_DESCRIPTION = "The `" + RESOURCE_TYPE_CONNECTOR_AZURE + "` resource allows you to manage Azure connectors for Trend Micro Vision One Cloud Account Management (CAM). This resource is used to create and manage Azure connectors that are connected to Trend Micro Vision One CAM for cloud account management purposes."
	RESOURCE_TYPE_RESOURCE_GROUPS             = "cam_resource_groups"
	RESOURCE_TYPE_RESOURCE_GROUPS_DESCRIPTION = "The `" + RESOURCE_TYPE_RESOURCE_GROUPS + "` resource allows you to manage Azure resource groups for Trend Micro Vision One Cloud Account Management (CAM). This resource is used to create and manage Azure resource groups that are connected to Trend Micro Vision One CAM for cloud account management purposes."
	RESOURCE_TYPE_ROLE_ASSIGNMENT             = "cam_role_assignment"
	RESOURCE_TYPE_ROLE_DEFINITION             = "cam_role_definition"

	AZURE_APP_REGISTRATION_NAME   = "v1-cam-"
	AZURE_CUSTOM_ROLE_DESCRIPTION = "This is a custom role created by Trend Micro Vision One"
	AZURE_CUSTOM_ROLE_NAME        = "v1-cam-role-"
	AZURE_CUSTOM_ROLE_SCOPE       = "/subscriptions/"
	AZURE_RESOURCE_GROUP_NAME     = "trendmicro-v1-"

	FAILED_TO_GET_CREDENTIALS_MSG     = "Failed to get Azure credentials"
	FAILED_TO_GET_GRAPH_CLIENT_MSG    = "Failed to get Microsoft Graph client"
	FEDERATED_CREDENTIALS_DESCRIPTION = "Federated Credentials created by Trend Micro Vision One, used for Accessing Azure Resources"
)

var (
	AZURE_CUSTOM_ROLE_ACTIONS      = []string{"*/read", "Microsoft.AppConfiguration/configurationStores/ListKeyValue/action", "Microsoft.Authorization/roleAssignments/read", "Microsoft.Authorization/roleDefinitions/read", "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action", "Microsoft.ContainerService/managedClusters/read", "Microsoft.KeyVault/vaults/secrets/read", "Microsoft.Network/networkWatchers/queryFlowLogStatus/action", "Microsoft.Resources/deployments/operationstatuses/read", "Microsoft.Resources/deployments/operations/read", "Microsoft.Resources/deployments/read", "Microsoft.Resources/subscriptions/resourceGroups/read", "Microsoft.Web/sites/config/list/Action", "Microsoft.Web/sites/config/read", "Microsoft.Web/sites/functions/listkeys/action"}
	AZURE_CUSTOM_ROLE_DATA_ACTIONS = []string{"Microsoft.KeyVault/vaults/keys/read", "Microsoft.KeyVault/vaults/secrets/getSecret/action", "Microsoft.KeyVault/vaults/secrets/readMetadata/action"}
)
