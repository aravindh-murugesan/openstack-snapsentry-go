package openstack

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud"
	"github.com/gophercloud/gophercloud/v2"
)

// isRetryable determines if an error is transient and warrants a retry.
// It specifically checks for standard HTTP 429/5xx codes from Gophercloud
// and assumes other unknown network errors are also retryable.
func isRetryable(err error) bool {
	var gopherErrors gophercloud.ErrUnexpectedResponseCode

	// Unwrap the error to see if it's a specific Gophercloud HTTP response error
	if errors.As(err, &gopherErrors) {
		switch gopherErrors.Actual {
		case http.StatusTooManyRequests, // 429 - Rate Limiting
			http.StatusRequestTimeout,      // 408 - Client Timeout
			http.StatusInternalServerError, // 500 - Server Error
			http.StatusServiceUnavailable,  // 503 - Maintenance/Overload
			http.StatusGatewayTimeout:      // 504 - Upstream Timeout
			return true
		default:
			// Client errors (400, 401, 404, etc.) are generally not retryable
			// as the request itself is invalid.
			return false
		}
	}
	// Fallback: If it's not a specific HTTP error code (e.g., DNS failure, connection reset),
	// we assume it's a transient network issue and safe to retry.
	return true
}

// ExecuteAction wraps a function with robust retry logic, including exponential backoff,
// jitter, and context timeouts.
//
// opName is used for logging and debugging purposes.
// operation is the function to execute; it must accept a context to support cancellation.
func ExecuteAction(ctx context.Context, cfg cloud.RetryConfig, opName string, operation func(ctx context.Context) error) error {
	// Enforce the global operation timeout defined in the config.
	// This ensures the retry loop doesn't run indefinitely.
	ctx, cancel := context.WithTimeout(ctx, cfg.OperationTimeout)
	defer cancel()

	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// 1. Pre-check: Stop immediately if the context is cancelled or timed out.
		if ctx.Err() != nil {
			return fmt.Errorf("%s timed out before attempt %d: %w", opName, attempt+1, ctx.Err())
		}

		// 2. Execute the operation
		lastErr = operation(ctx)
		if lastErr == nil {
			return nil // Success
		}

		// 3. Decision: Should we retry?
		if !isRetryable(lastErr) {
			return lastErr // Permanent error, fail fast.
		}

		// If this was the last attempt, don't wait/sleep, just return the error.
		if attempt == cfg.MaxRetries {
			break
		}

		slog.Warn("Transient error detected, scheduling retry",
			"operation", opName,
			"attempt", attempt+1,
			"max_retries", cfg.MaxRetries,
			"error", lastErr)

		// 4. Calculate Backoff (Exponential + Jitter)
		// Formula: BaseDelay * 2^attempt
		backoff := float64(cfg.BaseDelay) * math.Pow(2, float64(attempt))

		// Add Jitter: Randomize the wait time to prevent "thundering herd" problems.
		// We add a random duration between 0 and 50% of the calculated backoff.
		jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
		sleepDuration := time.Duration(backoff) + jitter

		// Cap the sleep duration at MaxDelay
		sleepDuration = min(sleepDuration, cfg.MaxDelay)

		// 5. Wait with Context awareness
		select {
		case <-time.After(sleepDuration):
			continue // Proceed to next attempt
		case <-ctx.Done():
			return fmt.Errorf("%s context cancelled during backoff: %w", opName, ctx.Err())
		}
	}

	return fmt.Errorf("%s failed after %d retries: %w", opName, cfg.MaxRetries, lastErr)
}
