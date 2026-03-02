package api

import (
	"context"
	"crypto/rand"
	"math/big"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// RateLimitDelayMin is the minimum random delay in milliseconds before each API call.
	RateLimitDelayMin = 100
	// RateLimitDelayMax is the maximum random delay in milliseconds before each API call.
	RateLimitDelayMax = 1000
)

// RateLimitTransport wraps an http.RoundTripper and introduces a randomized delay
// before each request to avoid hitting API rate limits.
type RateLimitTransport struct {
	Base     http.RoundTripper
	MinDelay time.Duration
	MaxDelay time.Duration
}

// NewRateLimitTransport creates a new RateLimitTransport wrapping the given base transport.
// If base is nil, http.DefaultTransport is used.
func NewRateLimitTransport(base http.RoundTripper) *RateLimitTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RateLimitTransport{
		Base:     base,
		MinDelay: RateLimitDelayMin * time.Millisecond,
		MaxDelay: RateLimitDelayMax * time.Millisecond,
	}
}

// RoundTrip executes a single HTTP transaction with a randomized delay before sending.
func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	delay := RandomDelay(t.MinDelay, t.MaxDelay)
	tflog.Debug(req.Context(), "[RateLimit] Applying randomized delay before API call", map[string]interface{}{
		"delay_ms": delay.Milliseconds(),
		"method":   req.Method,
		"url":      req.URL.String(),
	})
	time.Sleep(delay)
	return t.Base.RoundTrip(req)
}

// RandomDelay returns a random duration between minDelay and maxDelay (inclusive).
func RandomDelay(minDelay, maxDelay time.Duration) time.Duration {
	if maxDelay <= minDelay {
		return minDelay
	}
	jitterRange := big.NewInt(int64(maxDelay - minDelay + 1))
	jitter, _ := rand.Int(rand.Reader, jitterRange)
	return minDelay + time.Duration(jitter.Int64())
}

// NewRateLimitedHTTPClient creates an *http.Client with a RateLimitTransport applied.
func NewRateLimitedHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		Transport:     NewRateLimitTransport(transport),
		CheckRedirect: base.CheckRedirect,
		Jar:           base.Jar,
		Timeout:       base.Timeout,
	}
}

// NewRateLimitedHTTPClientWithCredentials creates an *http.Client that applies
// OAuth2 authentication from the given credentials AND a randomized delay before
// each request. This is necessary because option.WithHTTPClient takes precedence
// over option.WithCredentials in the Google API SDK, so we must bake the OAuth2
// transport into the HTTP client ourselves.
func NewRateLimitedHTTPClientWithCredentials(ctx context.Context, cred *google.Credentials) *http.Client {
	// Create an OAuth2-authenticated HTTP client from the credential's token source
	authenticatedClient := oauth2.NewClient(ctx, cred.TokenSource)
	// Wrap the authenticated transport with the rate limit transport
	return &http.Client{
		Transport: NewRateLimitTransport(authenticatedClient.Transport),
	}
}
