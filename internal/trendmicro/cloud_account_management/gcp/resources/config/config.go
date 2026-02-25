package config

const (
	GCP_CUSTOM_ROLE_NAME          = "vision_one_cam_role_"
	RESOURCE_TYPE_IAM_CUSTOM_ROLE = "cam_iam_custom_role"

	// Connector constants
	RESOURCE_TYPE_CONNECTOR_GCP = "cam_connector_gcp"

	// Service Account Integration constants
	GCP_SERVICE_ACCOUNT_ROLE_NAME_PREFIX      = "vision_one_cam_sa_role_"
	RESOURCE_TYPE_SERVICE_ACCOUNT_INTEGRATION = "cam_service_account_integration"
	RESOURCE_TYPE_ENABLE_API_SERVICES         = "cam_enable_api_services"
	RESOURCE_TYPE_TAG_KEY                     = "cam_tag_key"
	RESOURCE_TYPE_TAG_VALUE                   = "cam_tag_value"

	// Service Account defaults
	SERVICE_ACCOUNT_DEFAULT_DISPLAY_NAME = "Vision One CAM Service Account"
	SERVICE_ACCOUNT_DEFAULT_DESCRIPTION  = "Service account for Trend Micro Vision One Cloud Account Management"

	// Custom role defaults for service account
	SA_CUSTOM_ROLE_DEFAULT_TITLE       = "Vision One CAM Service Account Role"
	SA_CUSTOM_ROLE_DEFAULT_DESCRIPTION = "Custom role for Vision One CAM service account"

	// Key configuration
	PRIVATE_KEY_TYPE_GOOGLE_CREDENTIALS = "TYPE_GOOGLE_CREDENTIALS_FILE" //nolint:gosec // Not a credential, just a constant for key type enum
	KEY_ALGORITHM_RSA_2048              = "KEY_ALG_RSA_2048"

	// Retry configuration for IAM policy updates
	IAM_POLICY_MAX_RETRIES        = 5
	IAM_POLICY_RETRY_INITIAL_WAIT = 1  // seconds
	IAM_POLICY_RETRY_MAX_WAIT     = 30 // seconds

	// GCP resource type constants
	PARENT_TYPE_ORGANIZATION = "organization"
	PARENT_TYPE_FOLDER       = "folder"

	// GCP lifecycle state constants
	LIFECYCLE_STATE_ACTIVE = "ACTIVE"
)

var GCP_CUSTOM_ROLE_CORE_PERMISSIONS = []string{
	"iam.roles.get",
	"iam.roles.list",
	"iam.serviceAccountKeys.create",
	"iam.serviceAccountKeys.delete",
	"iam.serviceAccounts.getAccessToken",
	"resourcemanager.tagKeys.get",
	"resourcemanager.tagKeys.list",
	"resourcemanager.tagValues.get",
	"resourcemanager.tagValues.list",
}

// GCP required API services to enable
// Note: This list can be extended when new features are added that require additional API services
var GCP_REQUIRED_ENABLE_API_AND_SERVICE = []string{
	"iamcredentials.googleapis.com",
	"cloudresourcemanager.googleapis.com",
	"iam.googleapis.com",
	"cloudbuild.googleapis.com",
	"deploymentmanager.googleapis.com",
	"cloudfunctions.googleapis.com",
	"pubsub.googleapis.com",
	"secretmanager.googleapis.com",
}
