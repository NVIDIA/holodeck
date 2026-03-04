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

package describe

import (
	"testing"
	"time"
)

func TestDescribeOutput_InstanceInfo(t *testing.T) {
	now := time.Now()
	output := &DescribeOutput{
		Instance: InstanceInfo{
			ID:          "test-123",
			Name:        "test-instance",
			CreatedAt:   now,
			Age:         "1h0m0s",
			CacheFile:   "/tmp/cache.yaml",
			Provisioned: true,
		},
		Provider: ProviderInfo{
			Type:     "aws",
			Region:   "us-west-2",
			Username: "ubuntu",
			KeyName:  "my-key",
		},
	}

	if output.Instance.ID != "test-123" {
		t.Errorf("expected ID test-123, got %s", output.Instance.ID)
	}
	if output.Instance.Name != "test-instance" {
		t.Errorf("expected name test-instance, got %s", output.Instance.Name)
	}
	if !output.Instance.Provisioned {
		t.Error("expected provisioned to be true")
	}
	if output.Provider.Type != "aws" {
		t.Errorf("expected provider aws, got %s", output.Provider.Type)
	}
	if output.Provider.Region != "us-west-2" {
		t.Errorf("expected region us-west-2, got %s", output.Provider.Region)
	}
}

func TestComponentsInfo_NilSafe(t *testing.T) {
	output := &DescribeOutput{
		Components: ComponentsInfo{},
	}

	if output.Components.Kernel != nil {
		t.Error("expected Kernel to be nil")
	}
	if output.Components.NVIDIADriver != nil {
		t.Error("expected NVIDIADriver to be nil")
	}
	if output.Components.ContainerRuntime != nil {
		t.Error("expected ContainerRuntime to be nil")
	}
	if output.Components.ContainerToolkit != nil {
		t.Error("expected ContainerToolkit to be nil")
	}
	if output.Components.Kubernetes != nil {
		t.Error("expected Kubernetes to be nil")
	}
}

func TestComponentsInfo_WithValues(t *testing.T) {
	output := &DescribeOutput{
		Components: ComponentsInfo{
			NVIDIADriver: &NVIDIADriverInfo{
				Install: true,
				Branch:  "560",
				Version: "560.35.03",
			},
			Kubernetes: &KubernetesInfo{
				Install:   true,
				Installer: "kubeadm",
				Version:   "v1.31.1",
			},
		},
	}

	if !output.Components.NVIDIADriver.Install {
		t.Error("expected driver install to be true")
	}
	if output.Components.NVIDIADriver.Version != "560.35.03" {
		t.Errorf("expected driver version 560.35.03, got %s", output.Components.NVIDIADriver.Version)
	}
	if output.Components.Kubernetes.Installer != "kubeadm" {
		t.Errorf("expected installer kubeadm, got %s", output.Components.Kubernetes.Installer)
	}
}

func TestStatusInfo_Conditions(t *testing.T) {
	status := StatusInfo{
		State: "running",
		Conditions: []ConditionInfo{
			{
				Type:    "Ready",
				Status:  "True",
				Reason:  "AllComponentsReady",
				Message: "All components are ready",
			},
		},
	}

	if status.State != "running" {
		t.Errorf("expected state running, got %s", status.State)
	}
	if len(status.Conditions) != 1 {
		t.Errorf("expected 1 condition, got %d", len(status.Conditions))
	}
	if status.Conditions[0].Type != "Ready" {
		t.Errorf("expected Ready condition, got %s", status.Conditions[0].Type)
	}
	if status.Conditions[0].Reason != "AllComponentsReady" {
		t.Errorf("expected reason AllComponentsReady, got %s", status.Conditions[0].Reason)
	}
}

func TestClusterInfo_WithHA(t *testing.T) {
	output := &DescribeOutput{
		Cluster: &ClusterInfo{
			Region: "us-west-2",
			ControlPlane: ControlPlaneInfo{
				Count:        3,
				InstanceType: "g4dn.xlarge",
				Dedicated:    true,
			},
			Workers: &WorkersInfo{
				Count:        2,
				InstanceType: "g4dn.2xlarge",
			},
			HighAvailability: &HAInfo{
				Enabled:          true,
				EtcdTopology:     "stacked",
				LoadBalancerType: "nlb",
			},
			TotalNodes: 5,
			ReadyNodes: 5,
		},
	}

	if output.Cluster == nil {
		t.Fatal("expected cluster to be non-nil")
	}
	if output.Cluster.ControlPlane.Count != 3 {
		t.Errorf("expected 3 control planes, got %d", output.Cluster.ControlPlane.Count)
	}
	if !output.Cluster.ControlPlane.Dedicated {
		t.Error("expected dedicated control plane")
	}
	if output.Cluster.Workers.Count != 2 {
		t.Errorf("expected 2 workers, got %d", output.Cluster.Workers.Count)
	}
	if !output.Cluster.HighAvailability.Enabled {
		t.Error("expected HA to be enabled")
	}
	if output.Cluster.TotalNodes != 5 {
		t.Errorf("expected 5 total nodes, got %d", output.Cluster.TotalNodes)
	}
}

func TestAWSResourcesInfo(t *testing.T) {
	output := &DescribeOutput{
		AWSResources: &AWSResourcesInfo{
			InstanceID:    "i-1234567890abcdef0",
			InstanceType:  "g4dn.xlarge",
			PublicDNS:     "ec2-1-2-3-4.compute.amazonaws.com",
			PublicIP:      "1.2.3.4",
			PrivateIP:     "10.0.0.5",
			VpcID:         "vpc-123",
			SubnetID:      "subnet-456",
			SecurityGroup: "sg-789",
			AMI:           "ami-abc123",
		},
	}

	if output.AWSResources.InstanceID != "i-1234567890abcdef0" {
		t.Errorf("expected instance ID, got %s", output.AWSResources.InstanceID)
	}
	if output.AWSResources.VpcID != "vpc-123" {
		t.Errorf("expected vpc-123, got %s", output.AWSResources.VpcID)
	}
}
