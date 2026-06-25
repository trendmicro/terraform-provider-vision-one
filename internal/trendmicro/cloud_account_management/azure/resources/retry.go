package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Microsoft Graph error fragments meaning a freshly created app object has not
// yet replicated across directory replicas.
var graphPropagationErrorSubstrings = []string{
	"does not reference a valid application object",
	"does not exist or one of its queried reference-property objects are not present",
}

func isGraphPropagationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, s := range graphPropagationErrorSubstrings {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// retryOnGraphPropagation retries fn on Graph propagation errors with capped
// exponential backoff (5s,10s,20s,30s,30s,30s). Other errors return immediately.
func retryOnGraphPropagation(ctx context.Context, label string, fn func() error) error {
	const maxRetries = 6
	baseDelay := 5 * time.Second
	maxDelay := 30 * time.Second

	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err = fn()
		if err == nil || !isGraphPropagationError(err) {
			return err
		}
		if attempt == maxRetries {
			break
		}
		delay := min(baseDelay*time.Duration(1<<attempt), maxDelay)
		tflog.Info(ctx, fmt.Sprintf("[%s] app registration not yet propagated, retrying in %s (attempt %d/%d): %s",
			label, delay, attempt+1, maxRetries, err.Error()))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return err
}
