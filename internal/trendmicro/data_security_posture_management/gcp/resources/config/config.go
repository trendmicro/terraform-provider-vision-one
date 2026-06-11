package config

const (
	RESOURCE_TYPE_LEGACY_CLEANUP_DSPM_REGION = "dspm_legacy_cleanup_region"

	// Legacy DSPM Package name prefix base — composes into dspm-{i|s|p}-{region_abbr}.
	LEGACY_GCP_DSPM_NAME_BASE = "dspm-"

	// Legacy GCS bucket holding the old Terraform Package Solution state.
	// Duplicated from cloud_account_management/gcp/resources/config so DSPM does
	// not depend on the CAM package. These two values are frozen by the legacy
	// Package's naming and will not change.
	LEGACY_GCP_GCS_BUCKET_PREFIX = "trendmicro-v1-"
	LEGACY_GCP_STATE_FILE_NAME   = "default.tfstate"
)
