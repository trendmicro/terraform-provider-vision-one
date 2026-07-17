package config

const (
	RESOURCE_TYPE_LEGACY_CLEANUP_DSPM_REGION = "dspm_legacy_cleanup_region"

	// Legacy DSPM Package name prefix base — composes into dspm-{i|s|p}-{region_abbr}.
	LEGACY_GCP_DSPM_NAME_BASE = "dspm-"

	// Legacy GCS bucket prefix; intentional copy from CAM config — frozen by legacy Package naming, will not change.
	LEGACY_GCP_GCS_BUCKET_PREFIX = "trendmicro-v1-"
	LEGACY_GCP_STATE_FILE_NAME   = "default.tfstate"

	// GCS object path for the Provider-mode state within a customer's state_bucket.
	PROVIDER_STATE_OBJECT_NAME = "terraform.tfstate/default.tfstate"
)
