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

package get

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/cmd/cli/common"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
)

func TestGetHostURL_AWS_SingleNode(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
		},
		Status: v1alpha1.EnvironmentStatus{
			Properties: []v1alpha1.Properties{
				{Name: aws.PublicDnsName, Value: "ec2-1-2-3-4.compute.amazonaws.com"},
			},
		},
	}

	url, err := common.GetHostURL(env, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "ec2-1-2-3-4.compute.amazonaws.com" {
		t.Errorf("expected DNS name, got %s", url)
	}
}

func TestGetHostURL_SSH(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderSSH,
			Instance: v1alpha1.Instance{
				HostUrl: "192.168.1.100",
			},
		},
	}

	url, err := common.GetHostURL(env, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %s", url)
	}
}

func TestGetHostURL_NoProperties(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
		},
		Status: v1alpha1.EnvironmentStatus{
			Properties: []v1alpha1.Properties{},
		},
	}

	_, err := common.GetHostURL(env, "", false)
	if err == nil {
		t.Error("expected error for missing properties")
	}
}

func TestGetHostURL_Cluster_ControlPlaneOnly(t *testing.T) {
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

	url, err := common.GetHostURL(env, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "10.0.0.2" {
		t.Errorf("expected control-plane IP 10.0.0.2, got %s", url)
	}
}

func TestGetHostURL_Cluster_SpecificNode(t *testing.T) {
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

	url, err := common.GetHostURL(env, "worker-0", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "10.0.0.2" {
		t.Errorf("expected worker IP 10.0.0.2, got %s", url)
	}
}

func TestGetHostURL_Cluster_NodeNotFound(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Cluster:  &v1alpha1.ClusterSpec{},
		},
		Status: v1alpha1.EnvironmentStatus{
			Cluster: &v1alpha1.ClusterStatus{
				Nodes: []v1alpha1.NodeStatus{
					{Name: "cp-0", Role: "control-plane", PublicIP: "10.0.0.1"},
				},
			},
		},
	}

	_, err := common.GetHostURL(env, "nonexistent", false)
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}
