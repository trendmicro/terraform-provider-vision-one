package config

const (
	RESOURCE_TYPE_APP_REGISTRATION                = "cam_app_registration"
	RESOURCE_TYPE_SERVICE_PRINCIPAL               = "cam_service_principal"
	RESOURCE_TYPE_FEDERATED_IDENTITY              = "cam_federated_identity"
	RESOURCE_TYPE_CONNECTOR_AZURE                 = "cam_connector_azure"
	RESOURCE_TYPE_CONNECTOR_AZURE_DESCRIPTION     = "The `" + RESOURCE_TYPE_CONNECTOR_AZURE + "` resource allows you to manage Azure connectors for Trend AI Vision One Cloud Account Management (CAM). This resource is used to create and manage Azure connectors that are connected to Trend AI Vision One CAM for cloud account management purposes."
	RESOURCE_TYPE_RESOURCE_GROUPS                 = "cam_resource_groups"
	RESOURCE_TYPE_RESOURCE_GROUPS_DESCRIPTION     = "The `" + RESOURCE_TYPE_RESOURCE_GROUPS + "` resource allows you to manage Azure resource groups for Trend AI Vision One Cloud Account Management (CAM). This resource is used to create and manage Azure resource groups that are connected to Trend AI Vision One CAM for cloud account management purposes."
	RESOURCE_TYPE_ROLE_ASSIGNMENT                 = "cam_role_assignment"
	RESOURCE_TYPE_ROLE_DEFINITION                 = "cam_role_definition"
	RESOURCE_TYPE_LEGACY_CLEANUP_CUSTOM_ROLE      = "cam_legacy_cleanup_custom_role"
	RESOURCE_TYPE_LEGACY_CLEANUP_RESOURCE_GROUP   = "cam_legacy_cleanup_resource_group"
	RESOURCE_TYPE_LEGACY_CLEANUP_APP_REGISTRATION = "cam_legacy_cleanup_app_registration"

	// Resource Naming Prefixes (for resources we CREATE)
	AZURE_APP_REGISTRATION_NAME   = "v1-cam-"
	AZURE_CUSTOM_ROLE_DESCRIPTION = "This is a custom role created by Trend AI Vision One"
	AZURE_CUSTOM_ROLE_NAME        = "v1-cam-role-"
	AZURE_CUSTOM_ROLE_SCOPE       = "/subscriptions/"

	// LEGACY Resource Naming Prefixes (for Version1 resources we DETECT/CLEANUP)
	LEGACY_AZURE_CUSTOM_ROLE_PREFIX      = "v1-custom-role-"
	LEGACY_AZURE_RESOURCE_GROUP_PREFIX   = "trendmicro-v1-"
	LEGACY_AZURE_STORAGE_ACCOUNT_PREFIX  = "camtfstatestorage"
	LEGACY_AZURE_APP_REGISTRATION_PREFIX = "v1-app-registration-"
	LEGACY_AZURE_CONTAINER_PREFIX        = "camtfstate"
	LEGACY_AZURE_FEDERATED_IDENTITY_NAME = "v1-fed-cred"

	FAILED_TO_GET_CREDENTIALS_MSG     = "Failed to get Azure credentials"
	FAILED_TO_GET_GRAPH_CLIENT_MSG    = "Failed to get Microsoft Graph client"
	FEDERATED_CREDENTIALS_DESCRIPTION = "Federated Credentials created by Trend AI Vision One, used for Accessing Azure Resources"

	// Cleanup modes
	CLEANUP_MODE_DETECT_ONLY       = "detect_only"
	CLEANUP_MODE_DETECT_AND_REMOVE = "detect_and_remove"
)

var (
	AZURE_CUSTOM_ROLE_ACTIONS = []string{
		"*/read",
		"Microsoft.AppConfiguration/configurationStores/ListKeyValue/action",
		"Microsoft.Authorization/roleAssignments/read",
		"Microsoft.Authorization/roleDefinitions/read",
		"Microsoft.ContainerRegistry/registries/read",
		"Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
		"Microsoft.ContainerService/managedClusters/read",
		"Microsoft.EventGrid/eventSubscriptions/read",
		"Microsoft.EventGrid/systemTopics/read",
		"Microsoft.KeyVault/vaults/secrets/read",
		"Microsoft.Network/networkWatchers/queryFlowLogStatus/action",
		"Microsoft.ResourceGraph/resources/read",
		"Microsoft.Resources/deployments/operationstatuses/read",
		"Microsoft.Resources/deployments/operations/read",
		"Microsoft.Resources/deployments/read",
		"Microsoft.Resources/subscriptions/resourceGroups/read",
		"Microsoft.ServiceBus/namespaces/queues/read",
		"Microsoft.Storage/storageAccounts/listKeys/action",
		"Microsoft.Storage/storageAccounts/read",
		"Microsoft.Web/sites/config/list/Action",
		"Microsoft.Web/sites/config/read",
		"Microsoft.Web/sites/functions/listkeys/action",
		"microsoft.app/containerapps/read",
	}
	AZURE_CUSTOM_ROLE_DATA_ACTIONS = []string{
		"Microsoft.KeyVault/vaults/keys/read",
		"Microsoft.KeyVault/vaults/secrets/getSecret/action",
		"Microsoft.KeyVault/vaults/secrets/readMetadata/action",
	}
)
