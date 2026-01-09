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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const (
	// DefaultNCTRepo is the default NVIDIA Container Toolkit repository.
	DefaultNCTRepo = "https://github.com/NVIDIA/nvidia-container-toolkit.git"
)

// Resolver resolves git references to commit SHAs.
type Resolver interface {
	// Resolve returns the full and short commit SHA for a reference.
	Resolve(ctx context.Context, repo, ref string) (fullSHA, shortSHA string, err error)
}

// GitHubResolver resolves refs using GitHub API.
type GitHubResolver struct {
	client *http.Client
}

// NewGitHubResolver creates a new GitHubResolver.
func NewGitHubResolver() *GitHubResolver {
	return &GitHubResolver{
		client: &http.Client{},
	}
}

// NewGitHubResolverWithClient creates a GitHubResolver with a custom HTTP
// client.
func NewGitHubResolverWithClient(client *http.Client) *GitHubResolver {
	return &GitHubResolver{
		client: client,
	}
}

// commitResponse represents the GitHub API response for commit info.
type commitResponse struct {
	SHA string `json:"sha"`
}

// Resolve handles various ref formats:
//   - Full SHA: abc123def456789...
//   - Short SHA: abc123
//   - Tag: v1.17.3 or refs/tags/v1.17.3
//   - Branch: main or refs/heads/main
//   - PR: refs/pull/123/head
func (r *GitHubResolver) Resolve(
	ctx context.Context, repo, ref string,
) (string, string, error) {
	owner, repoName, err := ParseRepoURL(repo)
	if err != nil {
		return "", "", err
	}

	normalizedRef := NormalizeRef(ref)

	// Use GitHub API to resolve ref
	// GET /repos/{owner}/{repo}/commits/{ref}
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/commits/%s",
		owner, repoName, normalizedRef,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve ref: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf(
			"ref not found: %s (status %d)", ref, resp.StatusCode,
		)
	}

	var commit commitResponse
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	if commit.SHA == "" {
		return "", "", fmt.Errorf("empty SHA in response for ref: %s", ref)
	}

	fullSHA := commit.SHA
	shortSHA := commit.SHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}

	return fullSHA, shortSHA, nil
}

// ParseRepoURL extracts owner and repo name from a GitHub URL.
// Handles:
//   - https://github.com/NVIDIA/nvidia-container-toolkit.git
//   - git@github.com:NVIDIA/nvidia-container-toolkit.git
//   - github.com/NVIDIA/nvidia-container-toolkit
func ParseRepoURL(repo string) (owner, name string, err error) {
	re := regexp.MustCompile(`github\.com[:/]([^/]+)/([^/.]+)`)
	matches := re.FindStringSubmatch(repo)
	if len(matches) != 3 {
		return "", "", fmt.Errorf("invalid GitHub repo URL: %s", repo)
	}
	return matches[1], matches[2], nil
}

// NormalizeRef strips refs/ prefix for API calls.
func NormalizeRef(ref string) string {
	ref = strings.TrimPrefix(ref, "refs/tags/")
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return ref
}
