/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v2"
)

func TestNewCommand(t *testing.T) {
	log := logger.NewLogger()
	cmd := NewCommand(log)

	assert.NotNil(t, cmd)
	assert.Equal(t, "validate", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotNil(t, cmd.Action)
}

func TestCommandFlags(t *testing.T) {
	log := logger.NewLogger()
	cmd := NewCommand(log)

	flagNames := make(map[string]bool)
	for _, flag := range cmd.Flags {
		for _, name := range flag.Names() {
			flagNames[name] = true
		}
	}

	expectedFlags := []string{
		"envFile",
		"f",
		"strict",
	}

	for _, expected := range expectedFlags {
		assert.True(t, flagNames[expected], "Expected flag '%s' to be present", expected)
	}
}

func TestCommandAction_MissingEnvFile(t *testing.T) {
	log := logger.NewLogger()
	cmd := NewCommand(log)
	app := &cli.App{
		Commands: []*cli.Command{cmd},
	}

	err := app.Run([]string{"holodeck", "validate"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "envFile")
}

func TestCommandAction_NonExistentFile(t *testing.T) {
	log := logger.NewLogger()
	cmd := NewCommand(log)
	app := &cli.App{
		Commands: []*cli.Command{cmd},
	}

	err := app.Run([]string{"holodeck", "validate", "-f", "/nonexistent/file.yaml"})
	require.Error(t, err)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateEnvFile(t *testing.T) {
	tests := []struct {
		name        string
		envFile     string
		createFile  bool
		fileContent string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty env file path",
			envFile:     "",
			expectError: true,
			errorMsg:    "environment file path is required",
		},
		{
			name:        "non-existent file",
			envFile:     "/tmp/nonexistent.yaml",
			expectError: true,
			errorMsg:    "file not found",
		},
		{
			name:        "invalid YAML",
			createFile:  true,
			fileContent: "invalid: yaml: content: [",
			expectError: true,
			errorMsg:    "invalid YAML",
		},
		{
			name:        "valid YAML",
			createFile:  true,
			fileContent: `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test
spec:
  provider: aws`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var envFilePath string
			if tt.createFile {
				tmpDir := t.TempDir()
				envFilePath = filepath.Join(tmpDir, "env.yaml")
				err := os.WriteFile(envFilePath, []byte(tt.fileContent), 0644)
				require.NoError(t, err)
			} else {
				envFilePath = tt.envFile
			}

			log := logger.NewLogger()
			cmd := &command{
				log:     log,
				envFile: envFilePath,
			}

			env, err := cmd.validateEnvFile()
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, env)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, env)
			}
		})
	}
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		env         *v1alpha1.Environment
		expectError bool
	}{
		{
			name: "missing provider",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Auth: v1alpha1.Auth{
						KeyName: "test-key",
					},
				},
			},
			expectError: true,
		},
		{
			name: "missing keyName",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
				},
			},
			expectError: true,
		},
		{
			name: "AWS missing region",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Auth: v1alpha1.Auth{
						KeyName: "test-key",
					},
				},
			},
			expectError: true,
		},
		{
			name: "AWS missing instance type",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Auth: v1alpha1.Auth{
						KeyName: "test-key",
					},
					Instance: v1alpha1.Instance{
						Region: "us-west-2",
					},
				},
			},
			expectError: true,
		},
		{
			name: "SSH missing hostUrl",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderSSH,
					Auth: v1alpha1.Auth{
						KeyName: "test-key",
					},
				},
			},
			expectError: true,
		},
		{
			name: "valid AWS config",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Auth: v1alpha1.Auth{
						KeyName: "test-key",
					},
					Instance: v1alpha1.Instance{
						Region: "us-west-2",
						Type:   "m5.large",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid SSH config",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderSSH,
					Auth: v1alpha1.Auth{
						KeyName: "test-key",
					},
					Instance: v1alpha1.Instance{
						HostUrl: "192.168.1.100",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid AWS cluster config",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Auth: v1alpha1.Auth{
						KeyName: "test-key",
					},
					Cluster: &v1alpha1.ClusterSpec{
						Region: "us-west-2",
						ControlPlane: v1alpha1.ControlPlaneSpec{
							Count:        1,
							InstanceType: "m5.large",
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewLogger()
			cmd := &command{log: log}

			results := cmd.validateRequiredFields(tt.env)
			hasError := false
			for _, r := range results {
				if !r.Passed {
					hasError = true
					break
				}
			}

			if tt.expectError {
				assert.True(t, hasError, "Expected validation errors")
			} else {
				assert.False(t, hasError, "Expected no validation errors")
			}
		})
	}
}

func TestValidateSSHKeys(t *testing.T) {
	tests := []struct {
		name        string
		env         *v1alpha1.Environment
		setupFiles  func(*testing.T) (string, string) // returns privateKeyPath, publicKeyPath
		expectError bool
	}{
		{
			name: "missing private key path",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Auth: v1alpha1.Auth{
						PublicKey: "/tmp/pub.key",
					},
				},
			},
			expectError: true,
		},
		{
			name: "missing public key path",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Auth: v1alpha1.Auth{
						PrivateKey: "/tmp/priv.key",
					},
				},
			},
			expectError: true,
		},
		{
			name: "non-existent private key",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Auth: v1alpha1.Auth{
						PrivateKey: "/tmp/nonexistent.key",
						PublicKey:  "/tmp/pub.key",
					},
				},
			},
			setupFiles: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				pubKey := filepath.Join(tmpDir, "pub.key")
				os.WriteFile(pubKey, []byte("ssh-rsa test"), 0644)
				return "/tmp/nonexistent.key", pubKey
			},
			expectError: true,
		},
		{
			name: "valid keys",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Auth: v1alpha1.Auth{
						PrivateKey: "",
						PublicKey:  "",
					},
				},
			},
			setupFiles: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				privKey := filepath.Join(tmpDir, "priv.key")
				pubKey := filepath.Join(tmpDir, "pub.key")
				os.WriteFile(privKey, []byte("-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----\n"), 0600)
				os.WriteFile(pubKey, []byte("ssh-rsa test"), 0644)
				return privKey, pubKey
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFiles != nil {
				privKey, pubKey := tt.setupFiles(t)
				tt.env.Spec.Auth.PrivateKey = privKey
				tt.env.Spec.Auth.PublicKey = pubKey
			}

			log := logger.NewLogger()
			cmd := &command{log: log}

			results := cmd.validateSSHKeys(tt.env)
			hasError := false
			for _, r := range results {
				if !r.Passed {
					hasError = true
					break
				}
			}

			if tt.expectError {
				assert.True(t, hasError, "Expected validation errors")
			} else {
				assert.False(t, hasError, "Expected no validation errors")
			}
		})
	}
}

func TestValidateComponents(t *testing.T) {
	tests := []struct {
		name        string
		env         *v1alpha1.Environment
		expectWarn  bool
	}{
		{
			name: "Container Toolkit without container runtime",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
						Install: true,
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Install: false,
					},
				},
			},
			expectWarn: true,
		},
		{
			name: "Kubernetes without container runtime",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						Install: true,
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Install: false,
					},
				},
			},
			expectWarn: true,
		},
		{
			name: "Invalid Kubernetes installer",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						Install:             true,
						KubernetesInstaller: "invalid-installer",
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Install: true,
					},
				},
			},
			expectWarn: true,
		},
		{
			name: "Valid configuration",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Install: true,
					},
					NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
						Install: true,
					},
					Kubernetes: v1alpha1.Kubernetes{
						Install:             true,
						KubernetesInstaller: "kubeadm",
					},
				},
			},
			expectWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewLogger()
			cmd := &command{log: log}

			results := cmd.validateComponents(tt.env)
			hasWarn := false
			for _, r := range results {
				if !r.Passed {
					hasWarn = true
					break
				}
			}

			if tt.expectWarn {
				assert.True(t, hasWarn, "Expected warnings")
			} else {
				assert.False(t, hasWarn, "Expected no warnings")
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with ~",
			input:    "~/.ssh/key",
			expected: "", // Will be different per system, just check it expands
		},
		{
			name:     "absolute path",
			input:    "/tmp/key",
			expected: "/tmp/key",
		},
		{
			name:     "relative path",
			input:    "./key",
			expected: "./key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.input == "~/.ssh/key" {
				// Check that ~ was expanded
				assert.NotContains(t, result, "~")
				if tt.expected != "" {
					homeDir, err := os.UserHomeDir()
					if err == nil {
						assert.Contains(t, result, homeDir)
					}
				}
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCommandStructure(t *testing.T) {
	log := logger.NewLogger()
	cmd := NewCommand(log)

	// Verify command metadata
	assert.Equal(t, "validate", cmd.Name)
	assert.Equal(t, "Validate a Holodeck environment file", cmd.Usage)
	assert.NotEmpty(t, cmd.Description)

	// Verify flags exist
	hasEnvFile := false
	hasStrict := false

	for _, flag := range cmd.Flags {
		names := flag.Names()
		for _, name := range names {
			switch name {
			case "envFile", "f":
				hasEnvFile = true
			case "strict":
				hasStrict = true
			}
		}
	}

	assert.True(t, hasEnvFile, "envFile flag should exist")
	assert.True(t, hasStrict, "strict flag should exist")
}

func TestCommandAction_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(envFile, []byte("invalid: yaml: ["), 0644)
	require.NoError(t, err)

	log := logger.NewLogger()
	cmd := NewCommand(log)
	app := &cli.App{
		Commands: []*cli.Command{cmd},
	}

	err = app.Run([]string{"holodeck", "validate", "-f", envFile})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}
