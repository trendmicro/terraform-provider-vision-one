package cloud_account_management

import "time"

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

const (
	// CAMAPITimeout is the HTTP client timeout for Azure and GCP CAM API operations.
	// These cloud provider APIs can be slow to respond, especially during initial setup.
	CAMAPITimeout = 60 * time.Second

	// GCPMaxServiceUsageConcurrency limits concurrent GCP Service Usage API calls
	// across all EnableAPIServices resource instances.
	GCPMaxServiceUsageConcurrency = 6

	// GCPMaxTagKeyConcurrency limits concurrent GCP Tag Key API calls
	// across all GCPTagKeyResource resource instances.
	GCPMaxTagKeyConcurrency = 4

	// GCPMaxServiceAccountConcurrency limits concurrent GCP IAM/CRM API calls
	// across all ServiceAccountIntegration resource instances.
	GCPMaxServiceAccountConcurrency = 6
)

var (
	// GCPServiceUsageSem is a global semaphore for EnableAPIServices.
	GCPServiceUsageSem = make(chan struct{}, GCPMaxServiceUsageConcurrency)

	// GCPTagKeySem is a global semaphore for GCPTagKeyResource.
	GCPTagKeySem = make(chan struct{}, GCPMaxTagKeyConcurrency)

	// GCPServiceAccountSem is a global semaphore for ServiceAccountIntegration.
	GCPServiceAccountSem = make(chan struct{}, GCPMaxServiceAccountConcurrency)
)
