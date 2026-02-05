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

package ssh

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
)

func TestGetHostURL_AWS(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
		},
		Status: v1alpha1.EnvironmentStatus{
			Properties: []v1alpha1.Properties{
				{Name: aws.PublicDnsName, Value: "ec2-test.compute.amazonaws.com"},
			},
		},
	}

	url, err := cmd.getHostURL(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "ec2-test.compute.amazonaws.com" {
		t.Errorf("expected DNS name, got %s", url)
	}
}

func TestGetHostURL_SSHProvider(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderSSH,
			Instance: v1alpha1.Instance{
				HostUrl: "10.0.0.5",
			},
		},
	}

	url, err := cmd.getHostURL(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "10.0.0.5" {
		t.Errorf("expected 10.0.0.5, got %s", url)
	}
}

func TestGetHostURL_Cluster_DefaultControlPlane(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Cluster:  &v1alpha1.ClusterSpec{},
		},
		Status: v1alpha1.EnvironmentStatus{
			Cluster: &v1alpha1.ClusterStatus{
				Nodes: []v1alpha1.NodeStatus{
					{Name: "worker-0", Role: "worker", PublicIP: "10.0.0.1"},
					{Name: "cp-0", Role: "control-plane", PublicIP: "10.0.0.2"},
				},
			},
		},
	}

	url, err := cmd.getHostURL(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "10.0.0.2" {
		t.Errorf("expected control-plane IP 10.0.0.2, got %s", url)
	}
}

func TestGetHostURL_Cluster_SpecificNode(t *testing.T) {
	cmd := command{node: "worker-0"}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Cluster:  &v1alpha1.ClusterSpec{},
		},
		Status: v1alpha1.EnvironmentStatus{
			Cluster: &v1alpha1.ClusterStatus{
				Nodes: []v1alpha1.NodeStatus{
					{Name: "cp-0", Role: "control-plane", PublicIP: "10.0.0.1"},
					{Name: "worker-0", Role: "worker", PublicIP: "10.0.0.2"},
				},
			},
		},
	}

	url, err := cmd.getHostURL(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "10.0.0.2" {
		t.Errorf("expected worker-0 IP 10.0.0.2, got %s", url)
	}
}

func TestGetHostURL_NoProperties(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
		},
		Status: v1alpha1.EnvironmentStatus{
			Properties: []v1alpha1.Properties{},
		},
	}

	_, err := cmd.getHostURL(env)
	if err == nil {
		t.Error("expected error for missing properties")
	}
}

func TestContainsSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello", false},
		{"hello world", true},
		{"hello\tworld", true},
		{"", false},
		{"nospaces", false},
		{" leading", true},
		{"trailing ", true},
	}

	for _, tt := range tests {
		if got := containsSpace(tt.input); got != tt.expected {
			t.Errorf("containsSpace(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
