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

package provision

import (
	"os"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v2"
)

func TestNewCommand(t *testing.T) {
	log := logger.NewLogger()
	cmd := NewCommand(log)

	assert.NotNil(t, cmd)
	assert.Equal(t, "provision", cmd.Name)
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
		"cachepath",
		"c",
		"kubeconfig",
		"k",
		"ssh",
		"host",
		"key",
		"user",
		"u",
		"envFile",
		"f",
	}

	for _, expected := range expectedFlags {
		assert.True(t, flagNames[expected], "Expected flag '%s' to be present", expected)
	}
}

func TestCommandAction_InstanceMode(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		instanceID  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing instance ID",
			args:        []string{"holodeck", "provision"},
			expectError: true,
			errorMsg:    "instance ID is required",
		},
		{
			name:        "valid instance ID",
			args:        []string{"holodeck", "provision", "test-instance"},
			instanceID:  "test-instance",
			expectError: true, // Will fail because instance doesn't exist, but validates argument parsing
			errorMsg:    "failed to get instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewLogger()
			cmd := NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			err := app.Run(tt.args)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommandAction_SSHMode(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "SSH mode missing host",
			args:        []string{"holodeck", "provision", "--ssh", "--key", "/tmp/key", "-f", "/tmp/env.yaml"},
			expectError: true,
			errorMsg:    "--host is required",
		},
		{
			name:        "SSH mode missing key",
			args:        []string{"holodeck", "provision", "--ssh", "--host", "1.2.3.4", "-f", "/tmp/env.yaml"},
			expectError: true,
			errorMsg:    "--key is required",
		},
		{
			name:        "SSH mode missing env file",
			args:        []string{"holodeck", "provision", "--ssh", "--host", "1.2.3.4", "--key", "/tmp/key"},
			expectError: true,
			errorMsg:    "--envFile/-f is required",
		},
		{
			name:        "SSH mode with all required flags",
			args:        []string{"holodeck", "provision", "--ssh", "--host", "1.2.3.4", "--key", "/tmp/key", "-f", "/tmp/env.yaml"},
			expectError: true, // Will fail because files don't exist, but validates flag parsing
			errorMsg:    "failed to read environment file",
		},
		{
			name:        "SSH mode with custom username",
			args:        []string{"holodeck", "provision", "--ssh", "--host", "1.2.3.4", "--key", "/tmp/key", "-u", "ec2-user", "-f", "/tmp/env.yaml"},
			expectError: true,
			errorMsg:    "failed to read environment file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewLogger()
			cmd := NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			err := app.Run(tt.args)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetHostURL(t *testing.T) {
	tests := []struct {
		name        string
		env         *v1alpha1.Environment
		expectError bool
		expectedURL string
	}{
		{
			name: "AWS single node with public DNS",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
				},
				Status: v1alpha1.EnvironmentStatus{
					Properties: []v1alpha1.Properties{
						{
							Name:  aws.PublicDnsName,
							Value: "ec2-1-2-3-4.compute.amazonaws.com",
						},
					},
				},
			},
			expectError: false,
			expectedURL: "ec2-1-2-3-4.compute.amazonaws.com",
		},
		{
			name: "SSH provider with host URL",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderSSH,
					Instance: v1alpha1.Instance{
						HostUrl: "192.168.1.100"},
				},
			},
			expectError: false,
			expectedURL: "192.168.1.100",
		},
		{
			name: "Cluster with control-plane node",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Cluster:  &v1alpha1.ClusterSpec{},
				},
				Status: v1alpha1.EnvironmentStatus{
					Cluster: &v1alpha1.ClusterStatus{
						Nodes: []v1alpha1.NodeStatus{
							{
								Name:     "cp-1",
								PublicIP: "1.2.3.4",
								Role:     "control-plane",
							},
							{
								Name:     "worker-1",
								PublicIP: "5.6.7.8",
								Role:     "worker",
							},
						},
					},
				},
			},
			expectError: false,
			expectedURL: "1.2.3.4",
		},
		{
			name: "Cluster without control-plane, uses first node",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Cluster:  &v1alpha1.ClusterSpec{},
				},
				Status: v1alpha1.EnvironmentStatus{
					Cluster: &v1alpha1.ClusterStatus{
						Nodes: []v1alpha1.NodeStatus{
							{
								Name:     "worker-1",
								PublicIP: "5.6.7.8",
								Role:     "worker",
							},
						},
					},
				},
			},
			expectError: false,
			expectedURL: "5.6.7.8",
		},
		{
			name: "AWS without public DNS",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
				},
				Status: v1alpha1.EnvironmentStatus{
					Properties: []v1alpha1.Properties{},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewLogger()
			cmd := &command{log: log}

			url, err := cmd.getHostURL(tt.env)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unable to determine host URL")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedURL, url)
			}
		})
	}
}

func TestGetKubeconfigPath(t *testing.T) {
	tests := []struct {
		name         string
		instanceID   string
		expectInHome bool
	}{
		{
			name:         "valid instance ID",
			instanceID:   "test-instance-123",
			expectInHome: true,
		},
		{
			name:         "empty instance ID",
			instanceID:   "",
			expectInHome: false, // Falls back to simple name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := getKubeconfigPath(tt.instanceID)
			assert.NotEmpty(t, path)
			if tt.expectInHome {
				homeDir, err := os.UserHomeDir()
				if err == nil {
					assert.Contains(t, path, homeDir)
					assert.Contains(t, path, tt.instanceID)
				}
			}
		})
	}
}

func TestCommandStructure(t *testing.T) {
	log := logger.NewLogger()
	cmd := NewCommand(log)

	// Verify command metadata
	assert.Equal(t, "provision", cmd.Name)
	assert.Equal(t, "Provision or re-provision a Holodeck instance", cmd.Usage)
	assert.NotEmpty(t, cmd.Description)
	assert.Equal(t, "[instance-id]", cmd.ArgsUsage)

	// Verify flags exist
	hasCachePath := false
	hasKubeconfig := false
	hasSSH := false
	hasHost := false
	hasKey := false
	hasUser := false
	hasEnvFile := false

	for _, flag := range cmd.Flags {
		names := flag.Names()
		for _, name := range names {
			switch name {
			case "cachepath", "c":
				hasCachePath = true
			case "kubeconfig", "k":
				hasKubeconfig = true
			case "ssh":
				hasSSH = true
			case "host":
				hasHost = true
			case "key":
				hasKey = true
			case "user", "u":
				hasUser = true
			case "envFile", "f":
				hasEnvFile = true
			}
		}
	}

	assert.True(t, hasCachePath, "cachepath flag should exist")
	assert.True(t, hasKubeconfig, "kubeconfig flag should exist")
	assert.True(t, hasSSH, "ssh flag should exist")
	assert.True(t, hasHost, "host flag should exist")
	assert.True(t, hasKey, "key flag should exist")
	assert.True(t, hasUser, "user flag should exist")
	assert.True(t, hasEnvFile, "envFile flag should exist")
}
