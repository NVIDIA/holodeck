/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package gitref

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{
			name:      "HTTPS URL with .git",
			repo:      "https://github.com/NVIDIA/nvidia-container-toolkit.git",
			wantOwner: "NVIDIA",
			wantName:  "nvidia-container-toolkit",
			wantErr:   false,
		},
		{
			name:      "HTTPS URL without .git",
			repo:      "https://github.com/NVIDIA/nvidia-container-toolkit",
			wantOwner: "NVIDIA",
			wantName:  "nvidia-container-toolkit",
			wantErr:   false,
		},
		{
			name:      "SSH URL",
			repo:      "git@github.com:NVIDIA/nvidia-container-toolkit.git",
			wantOwner: "NVIDIA",
			wantName:  "nvidia-container-toolkit",
			wantErr:   false,
		},
		{
			name:      "Short form",
			repo:      "github.com/NVIDIA/holodeck",
			wantOwner: "NVIDIA",
			wantName:  "holodeck",
			wantErr:   false,
		},
		{
			name:    "Invalid URL - GitLab",
			repo:    "https://gitlab.com/NVIDIA/nvidia-container-toolkit",
			wantErr: true,
		},
		{
			name:    "Empty URL",
			repo:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, name, err := ParseRepoURL(tt.repo)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

func TestNormalizeRef(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "Tag with refs prefix",
			ref:  "refs/tags/v1.17.3",
			want: "v1.17.3",
		},
		{
			name: "Branch with refs prefix",
			ref:  "refs/heads/main",
			want: "main",
		},
		{
			name: "Plain tag",
			ref:  "v1.17.3",
			want: "v1.17.3",
		},
		{
			name: "Plain branch",
			ref:  "main",
			want: "main",
		},
		{
			name: "Commit SHA",
			ref:  "abc123def456",
			want: "abc123def456",
		},
		{
			name: "PR ref unchanged",
			ref:  "refs/pull/123/head",
			want: "refs/pull/123/head",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRef(tt.ref)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewGitHubResolver(t *testing.T) {
	resolver := NewGitHubResolver()
	assert.NotNil(t, resolver)
	assert.NotNil(t, resolver.client)
}
