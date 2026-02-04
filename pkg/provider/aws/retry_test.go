package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != defaultMaxRetries {
		t.Errorf("expected MaxRetries %d, got %d", defaultMaxRetries, cfg.MaxRetries)
	}
	if cfg.InitialBackoff != defaultInitialBackoff {
		t.Errorf("expected InitialBackoff %v, got %v", defaultInitialBackoff, cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != defaultMaxBackoff {
		t.Errorf("expected MaxBackoff %v, got %v", defaultMaxBackoff, cfg.MaxBackoff)
	}
}

func TestWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultRetryConfig()

	attempts := 0
	result, err := WithRetry(ctx, cfg, func() (string, error) {
		attempts++
		return "success", nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected result 'success', got %s", result)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	attempts := 0
	result, err := WithRetry(ctx, cfg, func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("Throttling: Rate exceeded")
		}
		return 42, nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("expected result 42, got %d", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	attempts := 0
	result, err := WithRetry(ctx, cfg, func() (string, error) {
		attempts++
		return "", errors.New("Throttling: Rate exceeded")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Throttling") {
		t.Errorf("expected throttling error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
	if attempts != 3 { // MaxRetries + 1 (initial attempt)
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultRetryConfig()

	attempts := 0
	result, err := WithRetry(ctx, cfg, func() (string, error) {
		attempts++
		return "", errors.New("InvalidParameter: Invalid value")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
}

func TestWithRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     200 * time.Millisecond,
	}

	attempts := 0
	cancel() // Cancel immediately

	result, err := WithRetry(ctx, cfg, func() (string, error) {
		attempts++
		return "", errors.New("Throttling: Rate exceeded")
	})

	if err == nil {
		t.Error("expected context error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if attempts > 1 {
		t.Errorf("expected at most 1 attempt after cancellation, got %d", attempts)
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "RequestLimitExceeded",
			err:      errors.New("RequestLimitExceeded: Too many requests"),
			expected: true,
		},
		{
			name:     "Throttling",
			err:      errors.New("Throttling: Rate exceeded"),
			expected: true,
		},
		{
			name:     "ServiceUnavailable",
			err:      errors.New("ServiceUnavailable: Service is down"),
			expected: true,
		},
		{
			name:     "InternalError",
			err:      errors.New("InternalError: Something went wrong"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "timeout",
			err:      errors.New("timeout waiting for response"),
			expected: true,
		},
		{
			name:     "InvalidParameter",
			err:      errors.New("InvalidParameter: Invalid value"),
			expected: false,
		},
		{
			name:     "AccessDenied",
			err:      errors.New("AccessDenied: Permission denied"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestWithRetry_ExponentialBackoff(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	attempts := 0
	start := time.Now()

	_, err := WithRetry(ctx, cfg, func() (string, error) {
		attempts++
		if attempts < 4 {
			return "", errors.New("Throttling: Rate exceeded")
		}
		return "success", nil
	})

	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	// Should have taken at least some time due to backoff
	// With 3 retries, we should have at least 10ms + 20ms + 40ms = 70ms (plus jitter)
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected elapsed time >= 50ms due to backoff, got %v", elapsed)
	}

	// Should not exceed reasonable bounds (allowing for jitter and execution time)
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected elapsed time < 500ms, got %v", elapsed)
	}
}

func TestWithRetry_BackoffCappedAtMax(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 20 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	}

	attempts := 0
	start := time.Now()

	_, err := WithRetry(ctx, cfg, func() (string, error) {
		attempts++
		if attempts < 6 {
			return "", errors.New("Throttling: Rate exceeded")
		}
		return "success", nil
	})

	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	// Backoff should be capped at MaxBackoff (50ms)
	// So max total time should be roughly: 20 + 40 + 50 + 50 + 50 = 210ms (plus jitter)
	if elapsed > 500*time.Millisecond {
		t.Errorf("backoff should be capped, expected elapsed < 500ms, got %v", elapsed)
	}
}

func TestWithRetry_GenericTypes(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultRetryConfig()

	// Test with int
	resultInt, err := WithRetry(ctx, cfg, func() (int, error) {
		return 123, nil
	})
	if err != nil || resultInt != 123 {
		t.Errorf("int test failed: result=%d, err=%v", resultInt, err)
	}

	// Test with struct
	type TestStruct struct {
		Value string
	}
	resultStruct, err := WithRetry(ctx, cfg, func() (TestStruct, error) {
		return TestStruct{Value: "test"}, nil
	})
	if err != nil || resultStruct.Value != "test" {
		t.Errorf("struct test failed: result=%+v, err=%v", resultStruct, err)
	}

	// Test with slice
	resultSlice, err := WithRetry(ctx, cfg, func() ([]string, error) {
		return []string{"a", "b", "c"}, nil
	})
	if err != nil || len(resultSlice) != 3 {
		t.Errorf("slice test failed: result=%v, err=%v", resultSlice, err)
	}
}
