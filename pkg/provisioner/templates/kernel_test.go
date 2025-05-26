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
package templates

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewKernelTemplate(t *testing.T) {
	tests := []struct {
		name        string
		env         v1alpha1.Environment
		wantErr     bool
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "kernel version not specified",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kernel: v1alpha1.Kernel{},
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// When Version is empty, the template should only contain whitespace
				if strings.TrimSpace(output) != "" {
					t.Errorf("expected empty output when kernel version is not specified, got: %s", output)
				}
			},
		},
		{
			name: "kernel version specified",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kernel: v1alpha1.Kernel{
						Version: "6.1.0",
					},
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// Check for required commands and configurations
				requiredElements := []string{
					"DEBIAN_FRONTEND=noninteractive",
					"EDITOR=/bin/true",
					"debconf debconf/frontend select Noninteractive",
					"CURRENT_KERNEL=$(uname -r)",
					"KERNEL_VERSION=\"6.1.0\"",
					"sudo apt-get update -y",
					"sudo rm -rf /boot/*${CURRENT_KERNEL}*",
					"sudo rm -rf /lib/modules/*${CURRENT_KERNEL}*",
					"sudo rm -rf /boot/*.old",
					"sudo apt-get install --allow-downgrades",
					"linux-image-${KERNEL_VERSION}",
					"linux-headers-${KERNEL_VERSION}",
					"linux-modules-${KERNEL_VERSION}",
					"sudo update-grub",
					"sudo update-initramfs -u -k ${KERNEL_VERSION}",
					"nohup sudo reboot",
				}

				for _, element := range requiredElements {
					if !strings.Contains(output, element) {
						t.Errorf("expected output to contain: %s", element)
					}
				}

				// Check for version comparison logic
				if !strings.Contains(output, "if [ \"${CURRENT_KERNEL}\" != \"${KERNEL_VERSION}\" ]") {
					t.Error("expected output to contain kernel version comparison")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewKernelTemplate(tt.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKernelTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, got.String())
			}
		})
	}
}

func TestKernelTemplateContent(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kernel: v1alpha1.Kernel{
				Version: "6.1.0",
			},
		},
	}

	template, err := NewKernelTemplate(env)
	if err != nil {
		t.Fatalf("Failed to create kernel template: %v", err)
	}

	output := template.String()

	// Test template structure
	tests := []struct {
		name     string
		contains string
	}{
		{
			name:     "non-interactive setup",
			contains: "DEBIAN_FRONTEND=noninteractive",
		},
		{
			name:     "kernel version",
			contains: "KERNEL_VERSION=\"6.1.0\"",
		},
		{
			name:     "apt update",
			contains: "sudo apt-get update -y",
		},
		{
			name:     "kernel package installation",
			contains: "sudo apt-get install --allow-downgrades",
		},
		{
			name:     "grub update",
			contains: "sudo update-grub",
		},
		{
			name:     "initramfs update",
			contains: "sudo update-initramfs -u -k ${KERNEL_VERSION}",
		},
		{
			name:     "reboot command",
			contains: "nohup sudo reboot",
		},
		{
			name:     "safe exit",
			contains: "# safely close the ssh connection\nexit 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(output, tt.contains) {
				t.Errorf("Template output does not contain expected content: %s", tt.contains)
			}
		})
	}
}

func TestKernelTemplateErrorHandling(t *testing.T) {
	// Test with invalid template
	invalidTemplate := "{{ .InvalidField }}"
	tmpl, err := template.New("invalid").Parse(invalidTemplate)
	if err != nil {
		t.Fatalf("Failed to create invalid template: %v", err)
	}

	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kernel: v1alpha1.Kernel{},
		},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, env)
	if err == nil {
		t.Error("Expected error when executing invalid template, got nil")
	}
}
