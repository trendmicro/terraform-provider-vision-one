package config

const (
	GCP_CUSTOM_ROLE_NAME          = "vision_one_cam_role_"
	GCP_SCAN_ROLE_NAME            = "trend_ai_auto_detect_"
	RESOURCE_TYPE_IAM_CUSTOM_ROLE = "cam_iam_custom_role"
	RESOURCE_TYPE_GCP_SCAN_ROLE   = "cam_gcp_scan_role"

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

	// Legacy GCP cleanup resource types
	RESOURCE_TYPE_LEGACY_CLEANUP_GCS_BUCKET        = "cam_legacy_cleanup_gcs_bucket"
	RESOURCE_TYPE_LEGACY_CLEANUP_IAM_CUSTOM_ROLE   = "cam_legacy_cleanup_iam_custom_role"
	RESOURCE_TYPE_LEGACY_CLEANUP_WORKLOAD_IDENTITY = "cam_legacy_cleanup_workload_identity"
	RESOURCE_TYPE_LEGACY_CLEANUP_SERVICE_ACCOUNT   = "cam_legacy_cleanup_service_account"

	// Legacy GCP resource naming prefixes (matching old Terraform Package Solution)
	LEGACY_GCP_GCS_BUCKET_PREFIX                 = "trendmicro-v1-"
	LEGACY_GCP_STATE_FILE_NAME                   = "default.tfstate"
	LEGACY_GCP_CUSTOM_ROLE_PREFIX                = "vision_one_cam_role_"
	LEGACY_GCP_SERVICE_ACCOUNT_NAME              = "vision-one-service-account"
	LEGACY_GCP_SERVICE_ACCOUNT_DISPLAY_NAME      = "Vision One Service Account"
	LEGACY_GCP_WORKLOAD_IDENTITY_POOL_ID_PREFIX  = "v1-workload-identity-pool-"
	LEGACY_GCP_WORKLOAD_IDENTITY_POOL_ID_PREFIX2 = "vision-one-wif-pool-"
	LEGACY_GCP_OIDC_PROVIDER_PREFIX              = "vision-one-oidc-"

	// Migration resource types
	RESOURCE_TYPE_GCP_PROJECT_MIGRATION = "cam_gcp_project_migration"
)

var GCP_CUSTOM_ROLE_CORE_PERMISSIONS = []string{
	"iam.roles.get",
	"iam.roles.list",
	"iam.serviceAccountKeys.create",
	"iam.serviceAccountKeys.delete",
	"iam.serviceAccounts.get",
	"iam.serviceAccounts.getAccessToken",
	"resourcemanager.tagKeys.get",
	"resourcemanager.tagKeys.list",
	"resourcemanager.tagValues.get",
	"resourcemanager.tagValues.list",
}

// GCP_SCAN_ROLE_CORE_PERMISSIONS is the read-only base for the org-level scan role
// granted once at the Org/Folder node for project discovery and read-only scanning.
// New projects under the node inherit these permissions through IAM, so no per-project
// scan binding is needed. Mirrors roles/browser (resource hierarchy discovery) plus the
// cloudasset.* read permissions from roles/cloudasset.viewer (Cloud Asset Inventory).
// roles/viewer is intentionally not inlined here (a basic role cannot be included in a
// custom role); grant it as a predefined role at the same node when viewer-level read is needed.
var GCP_SCAN_ROLE_CORE_PERMISSIONS = []string{
	// roles/browser: resource hierarchy discovery
	"resourcemanager.folders.get",
	"resourcemanager.folders.list",
	"resourcemanager.organizations.get",
	"resourcemanager.projects.get",
	"resourcemanager.projects.getIamPolicy",
	"resourcemanager.projects.list",
	// roles/cloudasset.viewer: Cloud Asset Inventory read
	"cloudasset.assets.searchAllResources",
	"cloudasset.assets.searchAllIamPolicies",
	"cloudasset.assets.listResource",
	"cloudasset.assets.exportResource",
	"cloudasset.feeds.get",
	"cloudasset.feeds.list",
}

const (
	FEATURE_DATA_SECURITY_POSTURE_MANAGEMENT = "data-security-posture-management"
)

// Per-feature permissions unioned onto GCP_CUSTOM_ROLE_CORE_PERMISSIONS; placeholder until Features API ships.
var FEATURE_PERMISSIONS = map[string][]string{
	// Required by visionone_dspm_legacy_cleanup_region (runs under CAM SA).
	// Derived by `cases/05_gcp_lifecycle/scripts/derive_dspm_cleanup_perms.py`.
	FEATURE_DATA_SECURITY_POSTURE_MANAGEMENT: {
		"cloudfunctions.functions.delete",
		"cloudscheduler.jobs.delete",
		"compute.disks.createSnapshot",
		"compute.disks.delete",
		"compute.firewalls.delete",
		"compute.instances.delete",
		"compute.networks.delete",
		"compute.networks.updatePolicy", // implicit for firewall.delete + router NAT.delete
		"compute.resourcePolicies.delete",
		"compute.routers.delete",
		"compute.routers.update",
		"compute.snapshots.create",
		"compute.subnetworks.delete",
		"cloudbuild.builds.list",
		"cloudbuild.builds.update",
		"eventarc.triggers.delete",
		"iam.serviceAccountKeys.list",
		"iam.serviceAccounts.delete",
		"logging.sinks.delete",
		"run.services.delete",
		"storage.buckets.delete",
		"storage.objects.delete",
		"vpcaccess.connectors.delete",
	},
}

// Separate from FEATURE_PERMISSIONS so the read-only scan role can never gain deploy/write perms.
var SCAN_FEATURE_PERMISSIONS = map[string][]string{
	FEATURE_DATA_SECURITY_POSTURE_MANAGEMENT: {},
}

// GCP required API services to enable; extend when new features need additional services.
var GCP_REQUIRED_ENABLE_API_AND_SERVICE = []string{
	"iamcredentials.googleapis.com",
	"cloudresourcemanager.googleapis.com",
	"iam.googleapis.com",
	"cloudbuild.googleapis.com",
	"deploymentmanager.googleapis.com",
	"cloudfunctions.googleapis.com",
	"pubsub.googleapis.com",
	"secretmanager.googleapis.com",
	// data-security-posture-management API services
	"run.googleapis.com",
	"cloudscheduler.googleapis.com",
	"eventarc.googleapis.com",
}
