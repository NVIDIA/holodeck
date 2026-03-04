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

package create

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShowSuccessMessage(t *testing.T) {
	tests := []struct {
		name           string
		instanceID     string
		opts           *options
		expectedOutput []string
		notExpected    []string
	}{
		{
			name:       "AWS instance with SSH and kubeconfig",
			instanceID: "i-12345",
			opts: &options{
				provision:  true,
				kubeconfig: "test.kubeconfig",
				cfg: v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Auth: v1alpha1.Auth{
							PrivateKey: "~/.ssh/test.pem",
							Username:   "ubuntu",
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install: true,
						},
					},
				},
				cache: v1alpha1.Environment{
					Status: v1alpha1.EnvironmentStatus{
						Properties: []v1alpha1.Properties{
							{
								Name:  aws.PublicDnsName,
								Value: "ec2-1-2-3-4.compute.amazonaws.com",
							},
						},
					},
				},
			},
			expectedOutput: []string{
				"✅ Successfully created instance: i-12345",
				"📋 SSH Connection:",
				"ssh -i ~/.ssh/test.pem ubuntu@ec2-1-2-3-4.compute.amazonaws.com",
				"📋 Kubernetes Access:",
				"Kubeconfig saved to:",
				"Option 1 - Copy to default location:",
				"cp",
				"~/.kube/config",
				"Option 2 - Set KUBECONFIG environment variable:",
				"export KUBECONFIG=",
				"Option 3 - Use with kubectl directly:",
				"kubectl --kubeconfig=",
				"📋 Next Steps:",
				"holodeck list",
				"holodeck delete i-12345",
				"holodeck status i-12345",
			},
			notExpected: []string{
				"microk8s",
				"kind",
			},
		},
		{
			name:       "SSH provider instance",
			instanceID: "ssh-instance",
			opts: &options{
				provision: true,
				cfg: v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderSSH,
						Instance: v1alpha1.Instance{
							HostUrl: "192.168.1.100",
						},
						Auth: v1alpha1.Auth{
							PrivateKey: "~/.ssh/id_rsa",
							Username:   "root",
						},
					},
				},
			},
			expectedOutput: []string{
				"✅ Successfully created instance: ssh-instance",
				"📋 SSH Connection:",
				"ssh -i ~/.ssh/id_rsa root@192.168.1.100",
				"📋 Next Steps:",
			},
			notExpected: []string{
				"📋 Kubernetes Access:",
			},
		},
		{
			name:       "Instance with microk8s",
			instanceID: "i-67890",
			opts: &options{
				provision: true,
				cfg: v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "microk8s",
						},
					},
				},
			},
			expectedOutput: []string{
				"✅ Successfully created instance: i-67890",
				"📋 Kubernetes Access:",
				"Note: For microk8s, access kubeconfig on the instance after SSH",
			},
			notExpected: []string{
				"Kubeconfig saved to:",
				"Option 1",
			},
		},
		{
			name:       "Instance without provisioning",
			instanceID: "i-noprov",
			opts: &options{
				provision: false,
				cfg: v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Kubernetes: v1alpha1.Kubernetes{
							Install: true,
						},
					},
				},
			},
			expectedOutput: []string{
				"✅ Successfully created instance: i-noprov",
				"📋 Kubernetes Access:",
				"Note: Run with --provision flag to install Kubernetes and download kubeconfig",
			},
			notExpected: []string{
				"Kubeconfig saved to:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture log output
			var buf bytes.Buffer
			log := logger.NewLogger()
			log.Out = &buf

			// Create command with test logger
			cmd := &command{log: log}

			// Create kubeconfig file if needed
			if tt.opts.kubeconfig != "" {
				tmpDir := t.TempDir()
				tt.opts.kubeconfig = filepath.Join(tmpDir, tt.opts.kubeconfig)
				err := os.WriteFile(tt.opts.kubeconfig, []byte("test-kubeconfig"), 0600)
				require.NoError(t, err)
			}

			// Call the function
			cmd.showSuccessMessage(tt.instanceID, tt.opts)

			// Check output
			output := buf.String()
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}

			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, output, notExpected, "Expected output NOT to contain: %s", notExpected)
			}
		})
	}
}

func TestHandleProvisionFailure(t *testing.T) {
	tests := []struct {
		name            string
		instanceID      string
		cachePath       string
		provisionErr    error
		envVars         map[string]string
		expectedError   string
		expectedOutput  []string
		expectUserInput bool
	}{
		{
			name:         "CI environment",
			instanceID:   "i-12345",
			cachePath:    "/tmp/test.yaml",
			provisionErr: assert.AnError,
			envVars: map[string]string{
				"CI": "true",
			},
			expectedError: "provisioning failed",
			expectedOutput: []string{
				"❌ Provisioning failed:",
				"💡 To clean up the failed instance, run:",
				"holodeck delete i-12345",
				"💡 To list all instances:",
				"holodeck list",
			},
			expectUserInput: false,
		},
		{
			name:         "Non-interactive environment",
			instanceID:   "i-67890",
			cachePath:    "/tmp/test2.yaml",
			provisionErr: assert.AnError,
			envVars: map[string]string{
				"HOLODECK_NONINTERACTIVE": "true",
			},
			expectedError: "provisioning failed",
			expectedOutput: []string{
				"❌ Provisioning failed:",
				"💡 To clean up the failed instance, run:",
				"holodeck delete i-67890",
			},
			expectUserInput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				oldVal := os.Getenv(k)
				err := os.Setenv(k, v)
				assert.NoError(t, err)
				defer func(key, val string) {
					err := os.Setenv(key, val)
					assert.NoError(t, err)
				}(k, oldVal)
			}

			// Create a buffer to capture log output
			var buf bytes.Buffer
			log := logger.NewLogger()
			log.Out = &buf

			// Create command with test logger
			cmd := &command{log: log}

			// Call the function
			err := cmd.handleProvisionFailure(tt.instanceID, tt.cachePath, tt.provisionErr)

			// Check error
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			// Check output
			output := buf.String()
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestProvideCleanupInstructions(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	log := logger.NewLogger()
	log.Out = &buf

	// Create command with test logger
	cmd := &command{log: log}

	// Call the function
	err := cmd.provideCleanupInstructions("test-instance", assert.AnError)

	// Check error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provisioning failed")

	// Check output
	output := buf.String()
	expectedOutputs := []string{
		"💡 The instance was created but provisioning failed",
		"You can manually investigate or clean up using the following commands:",
		"To delete this specific instance:",
		"holodeck delete test-instance",
		"To list all instances:",
		"holodeck list",
		"To see instance details:",
		"holodeck status test-instance",
		"💡 Additional debugging tips:",
		"Review the provisioning logs above for specific errors",
		"Check cloud provider console for instance status",
		"SSH into the instance to investigate further",
	}

	for _, expected := range expectedOutputs {
		assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
	}
}

func TestBuildCommand(t *testing.T) {
	// Create a logger
	var buf bytes.Buffer
	log := logger.NewLogger()
	log.Out = &buf

	// Create command
	cmd := NewCommand(log)

	// Verify command structure
	assert.Equal(t, "create", cmd.Name)
	assert.Equal(t, "create a test environment based on config file", cmd.Usage)

	// Check flags
	flagNames := make(map[string]bool)
	for _, flag := range cmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	expectedFlags := []string{
		"provision",
		"kubeconfig",
		"cachepath",
		"envFile",
	}

	for _, expected := range expectedFlags {
		assert.True(t, flagNames[expected], "Expected flag '%s' to be present", expected)
	}
}

func TestRunProvision(t *testing.T) {
	// This is a more complex function that involves SSH connections
	// We'll test the basic logic without actual connections

	tests := []struct {
		name          string
		opts          *options
		expectedError string
		skipProvision bool
	}{
		{
			name: "AWS provider without public DNS",
			opts: &options{
				cfg: v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
					},
				},
				cache: v1alpha1.Environment{
					Status: v1alpha1.EnvironmentStatus{
						Properties: []v1alpha1.Properties{},
					},
				},
			},
			expectedError: "failed to establish SSH connection",
			skipProvision: true,
		},
		{
			name: "SSH provider with host URL",
			opts: &options{
				cfg: v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderSSH,
						Instance: v1alpha1.Instance{
							HostUrl: "test-host",
						},
						Auth: v1alpha1.Auth{
							Username:   "test-user",
							PrivateKey: "test-key",
						},
					},
				},
			},
			expectedError: "no such file or directory",
			skipProvision: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipProvision {
				// Skip tests that require actual SSH connections
				t.Skip("Skipping test that requires SSH connection")
			}

			// Create a logger
			var buf bytes.Buffer
			log := logger.NewLogger()
			log.Out = &buf

			// Call the function
			err := runProvision(log, tt.opts)

			// Check error
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOptionsValidation(t *testing.T) {
	// Test that we can create valid options structures
	opts := &options{
		provision:  true,
		cachePath:  "/tmp/cache.yaml",
		envFile:    "/tmp/env.yaml",
		kubeconfig: "/tmp/kube.config",
		cfg: v1alpha1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-env",
			},
			Spec: v1alpha1.EnvironmentSpec{
				Provider: v1alpha1.ProviderAWS,
			},
		},
	}

	assert.True(t, opts.provision)
	assert.Equal(t, "/tmp/cache.yaml", opts.cachePath)
	assert.Equal(t, "test-env", opts.cfg.Name)
}

func TestCreateSSHProviderDoesNotPanic(t *testing.T) {
	log := logger.NewLogger()
	tmpDir := t.TempDir()

	opts := &options{
		cfg: v1alpha1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ssh-env",
			},
			Spec: v1alpha1.EnvironmentSpec{
				Provider: v1alpha1.ProviderSSH,
				Instance: v1alpha1.Instance{
					HostUrl: "192.168.1.100",
				},
				Auth: v1alpha1.Auth{
					PrivateKey: "/path/to/key",
					Username:   "root",
				},
			},
		},
		cachePath: tmpDir,
	}
	cmd := command{log: log}

	// Should not panic — should either succeed or return an error
	assert.NotPanics(t, func() {
		_ = cmd.run(nil, opts)
	})
}
