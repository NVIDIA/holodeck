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
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestKubeadmInitConfig_Execute(t *testing.T) {
	tests := []struct {
		name                 string
		controlPlaneEndpoint string
		isHA                 bool
		wantContains         []string
	}{
		{
			name:                 "Single node init",
			controlPlaneEndpoint: "10.0.0.1",
			isHA:                 false,
			wantContains: []string{
				"kubernetes-kubeadm-init",
				"CONTROL_PLANE_ENDPOINT=\"10.0.0.1\"",
				"IS_HA=\"false\"",
				"kubeadm init",
				"calico",
			},
		},
		{
			name:                 "HA cluster init",
			controlPlaneEndpoint: "my-lb.elb.amazonaws.com",
			isHA:                 true,
			wantContains: []string{
				"kubernetes-kubeadm-init",
				"CONTROL_PLANE_ENDPOINT=\"my-lb.elb.amazonaws.com\"",
				"IS_HA=\"true\"",
				"--upload-certs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Name: "containerd",
					},
					Kubernetes: v1alpha1.Kubernetes{
						KubernetesVersion: "v1.31.0",
					},
				},
			}

			config := KubeadmInitConfig{
				Environment:          env,
				ControlPlaneEndpoint: tt.controlPlaneEndpoint,
				IsHA:                 tt.isHA,
			}

			var buf bytes.Buffer
			err := config.Execute(&buf)
			require.NoError(t, err)

			output := buf.String()
			for _, s := range tt.wantContains {
				assert.Contains(t, output, s, "Output should contain: %s", s)
			}
		})
	}
}

func TestKubeadmJoinConfig_Execute(t *testing.T) {
	tests := []struct {
		name           string
		isControlPlane bool
		hasCertKey     bool
		wantContains   []string
	}{
		{
			name:           "Worker join",
			isControlPlane: false,
			hasCertKey:     false,
			wantContains: []string{
				"kubernetes-kubeadm-join",
				"IS_CONTROL_PLANE=\"false\"",
				"kubeadm join",
				"as worker node",
			},
		},
		{
			name:           "Control plane join",
			isControlPlane: true,
			hasCertKey:     true,
			wantContains: []string{
				"kubernetes-kubeadm-join",
				"IS_CONTROL_PLANE=\"true\"",
				"--control-plane",
				"--certificate-key",
				"as control-plane node",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := KubeadmJoinConfig{
				ControlPlaneEndpoint: "10.0.0.1",
				Token:                "abc123.def456ghi789",
				CACertHash:           "sha256:1234567890abcdef",
				IsControlPlane:       tt.isControlPlane,
			}
			if tt.hasCertKey {
				config.CertificateKey = "abcdef1234567890"
			}

			var buf bytes.Buffer
			err := config.Execute(&buf)
			require.NoError(t, err)

			output := buf.String()
			for _, s := range tt.wantContains {
				assert.Contains(t, output, s, "Output should contain: %s", s)
			}
		})
	}
}

func TestKubeadmPrereqConfig_Execute(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			Kubernetes: v1alpha1.Kubernetes{
				KubernetesVersion: "v1.31.0",
			},
		},
	}

	config := KubeadmPrereqConfig{
		Environment: env,
	}

	var buf bytes.Buffer
	err := config.Execute(&buf)
	require.NoError(t, err)

	output := buf.String()

	// Should contain prerequisites installation
	assert.Contains(t, output, "kubernetes-prereq")
	assert.Contains(t, output, "kubeadm")
	assert.Contains(t, output, "kubelet")
	assert.Contains(t, output, "kubectl")
	assert.Contains(t, output, "cni-plugins")

	// Should NOT contain init or join commands
	assert.NotContains(t, output, "kubeadm init")
	assert.NotContains(t, output, "kubeadm join")
}

func TestKubeadmInitTemplate_Variables(t *testing.T) {
	// Verify template has all expected variables
	assert.Contains(t, KubeadmInitTemplate, "{{.Version}}")
	assert.Contains(t, KubeadmInitTemplate, "{{.ControlPlaneEndpoint}}")
	assert.Contains(t, KubeadmInitTemplate, "{{.IsHA}}")
	assert.Contains(t, KubeadmInitTemplate, "{{.CalicoVersion}}")
	assert.Contains(t, KubeadmInitTemplate, "{{.CniPluginsVersion}}")
}

func TestKubeadmJoinTemplate_Variables(t *testing.T) {
	// Verify template has all expected variables
	assert.Contains(t, KubeadmJoinTemplate, "{{.ControlPlaneEndpoint}}")
	assert.Contains(t, KubeadmJoinTemplate, "{{.Token}}")
	assert.Contains(t, KubeadmJoinTemplate, "{{.CACertHash}}")
	assert.Contains(t, KubeadmJoinTemplate, "{{.IsControlPlane}}")
	assert.Contains(t, KubeadmJoinTemplate, "{{.CertificateKey}}")
}

func TestKubeadmInitTemplate_HAUploadCerts(t *testing.T) {
	// When HA is true, --upload-certs should be included
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			Kubernetes: v1alpha1.Kubernetes{
				KubernetesVersion: "v1.31.0",
			},
		},
	}

	config := KubeadmInitConfig{
		Environment:          env,
		ControlPlaneEndpoint: "10.0.0.1",
		IsHA:                 true,
	}

	var buf bytes.Buffer
	err := config.Execute(&buf)
	require.NoError(t, err)

	output := buf.String()

	// Find the kubeadm init command and verify --upload-certs is conditionally included
	lines := strings.Split(output, "\n")
	foundUploadCerts := false
	for _, line := range lines {
		if strings.Contains(line, "--upload-certs") {
			foundUploadCerts = true
			break
		}
	}
	assert.True(t, foundUploadCerts, "HA mode should include --upload-certs")
}

func TestKubeadmJoinTemplate_ControlPlaneArgs(t *testing.T) {
	// Control plane join should include --control-plane flag
	config := KubeadmJoinConfig{
		ControlPlaneEndpoint: "10.0.0.1",
		Token:                "abc123.def456",
		CACertHash:           "sha256:1234",
		CertificateKey:       "certkey123",
		IsControlPlane:       true,
	}

	var buf bytes.Buffer
	err := config.Execute(&buf)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--control-plane")
	assert.Contains(t, output, "--certificate-key")
}
