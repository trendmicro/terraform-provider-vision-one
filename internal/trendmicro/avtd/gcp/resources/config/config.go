package config

const (
	RESOURCE_TYPE_LEGACY_CLEANUP_AVTD_REGION = "avtd_legacy_cleanup_region"

	LEGACY_GCP_GCS_BUCKET_PREFIX = "trendmicro-v1-"
	LEGACY_GCP_STATE_FILE_NAME   = "default.tfstate"
)

var DEFAULT_RESOURCE_PREFIXES = []string{"v1avtd", "v1common", "v1phoenix"}

const RESOURCE_BUCKET_INFIX = "-resource-bucket-"

const ACCESS_LOGS_BUCKET_INFIX = "-access-logs-"
