package cloud_account_management

// JitterConfig holds the randomized delay parameters for API call throttling.
type JitterConfig struct {
	// MinDelayMs is the minimum delay in milliseconds before each API call.
	MinDelayMs int
	// MaxDelayMs is the maximum random jitter in milliseconds added on top of MinDelayMs.
	MaxDelayMs int
}

var (
	// AzureJitterConfig is the default jitter configuration for Azure CAM API calls.
	AzureJitterConfig = JitterConfig{
		MinDelayMs: 100,
		MaxDelayMs: 1000,
	}

	// GCPJitterConfig is the default jitter configuration for GCP CAM API calls.
	GCPJitterConfig = JitterConfig{
		MinDelayMs: 100,
		MaxDelayMs: 1000,
	}
)
