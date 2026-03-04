package aws

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"time"
)

const (
	defaultMaxRetries     = 3
	defaultInitialBackoff = 1 * time.Second
	defaultMaxBackoff     = 30 * time.Second
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     defaultMaxRetries,
		InitialBackoff: defaultInitialBackoff,
		MaxBackoff:     defaultMaxBackoff,
	}
}

// WithRetry executes fn with exponential backoff retry
func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var result T
	var err error

	backoff := cfg.InitialBackoff
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			return result, err
		}

		if attempt < cfg.MaxRetries {
			// Add jitter
			n, err := rand.Int(rand.Reader, big.NewInt(int64(backoff/2)))
			var jitter time.Duration
			if err == nil {
				jitter = time.Duration(n.Int64())
			}
			sleepDuration := backoff + jitter
			if sleepDuration > cfg.MaxBackoff {
				sleepDuration = cfg.MaxBackoff
			}

			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(sleepDuration):
			}

			backoff *= 2
		}
	}
	return result, err
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// AWS SDK v2 errors that are retryable
	errStr := err.Error()
	retryable := []string{
		"RequestLimitExceeded",
		"Throttling",
		"ServiceUnavailable",
		"InternalError",
		"connection reset",
		"timeout",
	}
	for _, r := range retryable {
		if strings.Contains(errStr, r) {
			return true
		}
	}
	return false
}
