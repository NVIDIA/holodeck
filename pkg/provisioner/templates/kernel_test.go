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
			name: "kernel modification disabled",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kernel: v1alpha1.Kernel{
						Modify: false,
					},
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// When Modify is false, the template should only contain whitespace
				if strings.TrimSpace(output) != "" {
					t.Errorf("expected empty output when kernel modification is disabled, got: %s", output)
				}
			},
		},
		{
			name: "kernel modification enabled with specific version",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kernel: v1alpha1.Kernel{
						Modify:  true,
						Version: "6.1.0",
					},
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// Check for required commands
				requiredCommands := []string{
					"sudo apt-get update",
					"install_packages_with_retry build-essential",
					"KERNEL_VERSION=\"6.1.0\"",
					"with_retry 3 5 wget",
					"make defconfig",
					"with_retry 3 5 make -j$(nproc)",
					"sudo make modules_install",
					"sudo make install",
					"sudo update-grub",
					"sudo reboot",
				}

				for _, cmd := range requiredCommands {
					if !strings.Contains(output, cmd) {
						t.Errorf("expected output to contain command: %s", cmd)
					}
				}

				// Check for kernel version
				if !strings.Contains(output, "KERNEL_VERSION=\"6.1.0\"") {
					t.Error("expected output to contain specified kernel version")
				}
			},
		},
		{
			name: "kernel modification enabled without version",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kernel: v1alpha1.Kernel{
						Modify: true,
					},
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// Check for required commands
				requiredCommands := []string{
					"sudo apt-get update",
					"install_packages_with_retry build-essential",
					"KERNEL_VERSION=$(curl -s https://www.kernel.org/releases.json",
					"with_retry 3 5 wget",
					"make defconfig",
					"with_retry 3 5 make -j$(nproc)",
					"sudo make modules_install",
					"sudo make install",
					"sudo update-grub",
					"sudo reboot",
				}

				for _, cmd := range requiredCommands {
					if !strings.Contains(output, cmd) {
						t.Errorf("expected output to contain command: %s", cmd)
					}
				}

				// Check for dynamic version detection
				if !strings.Contains(output, "KERNEL_VERSION=$(curl -s https://www.kernel.org/releases.json") {
					t.Error("expected output to contain dynamic kernel version detection")
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
				Modify:  true,
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
			name:     "package installation with retry",
			contains: "install_packages_with_retry build-essential",
		},
		{
			name:     "kernel version",
			contains: "KERNEL_VERSION=\"6.1.0\"",
		},
		{
			name:     "kernel download with retry",
			contains: "with_retry 3 5 wget",
		},
		{
			name:     "kernel compilation with retry",
			contains: "with_retry 3 5 make -j$(nproc)",
		},
		{
			name:     "kernel installation with sudo",
			contains: "sudo make install",
		},
		{
			name:     "grub update with sudo",
			contains: "sudo update-grub",
		},
		{
			name:     "reboot with sudo",
			contains: "sudo reboot",
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
			Kernel: v1alpha1.Kernel{
				Modify: true,
			},
		},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, env)
	if err == nil {
		t.Error("Expected error when executing invalid template, got nil")
	}
}
