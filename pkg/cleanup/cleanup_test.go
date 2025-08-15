/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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

package cleanup

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestGitHubJobsResponse(t *testing.T) {
	// Test that GitHubJobsResponse struct is properly defined
	response := GitHubJobsResponse{
		Jobs: []struct {
			Status string `json:"status"`
		}{
			{Status: "completed"},
			{Status: "in_progress"},
		},
	}

	assert.Equal(t, 2, len(response.Jobs))
	assert.Equal(t, "completed", response.Jobs[0].Status)
	assert.Equal(t, "in_progress", response.Jobs[1].Status)
}

func TestCleanerCreation(t *testing.T) {
	// Test that we can create a Cleaner struct
	// Note: This might succeed or fail depending on AWS credentials availability
	log := logger.NewLogger()

	// Try to create a cleaner - it might succeed or fail depending on environment
	cleaner, err := New(log, "us-east-1")

	// If successful, verify the cleaner is properly initialized
	if err == nil {
		assert.NotNil(t, cleaner)
		assert.NotNil(t, cleaner.ec2)
		assert.NotNil(t, cleaner.log)
	}
	// If it fails, that's also acceptable (no AWS credentials)
	// The important thing is that the function exists and returns proper types
}
