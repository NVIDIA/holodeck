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

package provisioner

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestDryrun_CustomTemplates_Valid(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			CustomTemplates: []v1alpha1.CustomTemplate{
				{
					Name:   "test-template",
					Phase:  v1alpha1.TemplatePhasePostInstall,
					Inline: "echo hello",
				},
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err != nil {
		t.Errorf("Dryrun failed for valid custom template: %v", err)
	}
}

func TestDryrun_CustomTemplates_InvalidPhase(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			CustomTemplates: []v1alpha1.CustomTemplate{
				{
					Name:   "bad-phase",
					Phase:  v1alpha1.TemplatePhase("invalid-phase"),
					Inline: "echo hello",
				},
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err == nil {
		t.Error("Dryrun should have failed for invalid phase")
	}
}

func TestDryrun_CustomTemplates_NoSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			CustomTemplates: []v1alpha1.CustomTemplate{
				{
					Name:  "no-source",
					Phase: v1alpha1.TemplatePhasePostInstall,
					// No Inline, File, or URL
				},
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err == nil {
		t.Error("Dryrun should have failed for template with no source")
	}
}

func TestDryrun_CustomTemplates_Multiple(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			CustomTemplates: []v1alpha1.CustomTemplate{
				{
					Name:   "pre-install-script",
					Phase:  v1alpha1.TemplatePhasePreInstall,
					Inline: "echo pre",
				},
				{
					Name:   "post-install-script",
					Phase:  v1alpha1.TemplatePhasePostInstall,
					Inline: "echo post",
				},
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err != nil {
		t.Errorf("Dryrun failed for multiple valid custom templates: %v", err)
	}
}

func TestDryrun_CustomTemplates_URLSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			CustomTemplates: []v1alpha1.CustomTemplate{
				{
					Name:     "url-template",
					Phase:    v1alpha1.TemplatePhasePostInstall,
					URL:      "https://example.com/script.sh",
					Checksum: "sha256:abc123",
				},
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err != nil {
		t.Errorf("Dryrun failed for URL custom template: %v", err)
	}
}

func TestDryrun_CustomTemplates_FileSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			CustomTemplates: []v1alpha1.CustomTemplate{
				{
					Name:  "file-template",
					Phase: v1alpha1.TemplatePhasePostInstall,
					File:  "./scripts/setup.sh",
				},
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err != nil {
		t.Errorf("Dryrun failed for file custom template: %v", err)
	}
}
