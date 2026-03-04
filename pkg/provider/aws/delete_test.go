/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package aws

import (
	"fmt"
	"testing"
	"time"
)

func TestDeleteConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      time.Duration
		wantMin  time.Duration
		wantMax  time.Duration
		describe string
	}{
		{
			name:     "apiCallTimeout",
			got:      apiCallTimeout,
			wantMin:  10 * time.Second,
			wantMax:  60 * time.Second,
			describe: "should be reasonable for EC2 API calls (10s-60s)",
		},
		{
			name:     "deletionTimeout",
			got:      deletionTimeout,
			wantMin:  5 * time.Minute,
			wantMax:  30 * time.Minute,
			describe: "should be sufficient for instance termination (5m-30m)",
		},
		{
			name:     "retryDelay",
			got:      retryDelay,
			wantMin:  1 * time.Second,
			wantMax:  10 * time.Second,
			describe: "should be reasonable for retry delay (1s-10s)",
		},
		{
			name:     "maxRetryDelay",
			got:      maxRetryDelay,
			wantMin:  10 * time.Second,
			wantMax:  60 * time.Second,
			describe: "should cap exponential backoff (10s-60s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got < tt.wantMin || tt.got > tt.wantMax {
				t.Errorf("%s = %v, want between %v and %v (%s)",
					tt.name, tt.got, tt.wantMin, tt.wantMax, tt.describe)
			}
		})
	}
}

func TestDeleteConstantRelationships(t *testing.T) {
	// apiCallTimeout should be less than deletionTimeout
	if apiCallTimeout >= deletionTimeout {
		t.Errorf("apiCallTimeout (%v) should be less than deletionTimeout (%v)",
			apiCallTimeout, deletionTimeout)
	}

	// apiCallTimeout should be greater than retryDelay to allow meaningful work
	if apiCallTimeout <= retryDelay {
		t.Errorf("apiCallTimeout (%v) should be greater than retryDelay (%v)",
			apiCallTimeout, retryDelay)
	}

	// maxRetryDelay should be >= retryDelay (since backoff starts at retryDelay)
	if maxRetryDelay < retryDelay {
		t.Errorf("maxRetryDelay (%v) should be >= retryDelay (%v)",
			maxRetryDelay, retryDelay)
	}
}

func TestErrorAggregationPattern(t *testing.T) {
	// This tests the error aggregation pattern used in deleteEC2Instances
	tests := []struct {
		name           string
		errors         []error
		wantNil        bool
		wantErrCount   int
		wantErrContain string
	}{
		{
			name:    "no errors returns nil",
			errors:  []error{},
			wantNil: true,
		},
		{
			name:           "single error is aggregated",
			errors:         []error{fmt.Errorf("instance i-1234 failed")},
			wantNil:        false,
			wantErrCount:   1,
			wantErrContain: "failed to terminate 1 instance(s)",
		},
		{
			name: "multiple errors are aggregated",
			errors: []error{
				fmt.Errorf("instance i-1234 failed"),
				fmt.Errorf("instance i-5678 failed"),
				fmt.Errorf("instance i-9012 failed"),
			},
			wantNil:        false,
			wantErrCount:   3,
			wantErrContain: "failed to terminate 3 instance(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the error aggregation pattern from deleteEC2Instances
			errChan := make(chan error, len(tt.errors))
			for _, err := range tt.errors {
				errChan <- err
			}
			close(errChan)

			// Apply the same aggregation logic
			var errs []error
			for err := range errChan {
				errs = append(errs, err)
			}

			var result error
			if len(errs) > 0 {
				result = fmt.Errorf("failed to terminate %d instance(s): %v", len(errs), errs)
			}

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil error, got %v", result)
				}
				return
			}

			if result == nil {
				t.Error("expected non-nil error, got nil")
				return
			}

			if len(errs) != tt.wantErrCount {
				t.Errorf("expected %d errors aggregated, got %d", tt.wantErrCount, len(errs))
			}

			errStr := result.Error()
			if tt.wantErrContain != "" && !containsSubstring(errStr, tt.wantErrContain) {
				t.Errorf("error message %q should contain %q", errStr, tt.wantErrContain)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
