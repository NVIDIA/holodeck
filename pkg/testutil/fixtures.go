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

package testutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

// ValidAWSEnvironment returns a minimal valid AWS environment configuration
// for testing purposes.
func ValidAWSEnvironment() *v1alpha1.Environment {
	return &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-env",
		},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Instance: v1alpha1.Instance{
				Type:   "t3.medium",
				Region: "us-east-1",
			},
			Auth: v1alpha1.Auth{
				KeyName:    "test-key",
				PrivateKey: "/path/to/key.pem",
				Username:   "ubuntu",
			},
		},
	}
}

// ValidSSHEnvironment returns a minimal valid SSH environment configuration
// for testing purposes.
func ValidSSHEnvironment() *v1alpha1.Environment {
	return &v1alpha1.Environment{
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
	}
}

// ValidEnvironmentWithKubernetes returns an environment with Kubernetes
// configuration for testing.
func ValidEnvironmentWithKubernetes() *v1alpha1.Environment {
	env := ValidAWSEnvironment()
	env.Spec.Kubernetes = v1alpha1.Kubernetes{
		Install:             true,
		KubernetesInstaller: "kubeadm",
		KubernetesVersion:   "v1.28.0",
	}
	return env
}

// ValidEnvironmentWithContainerRuntime returns an environment with container
// runtime configuration for testing.
func ValidEnvironmentWithContainerRuntime() *v1alpha1.Environment {
	env := ValidAWSEnvironment()
	env.Spec.ContainerRuntime = v1alpha1.ContainerRuntime{
		Install: true,
		Name:    "containerd",
	}
	return env
}

// ValidEnvironmentWithNVDriver returns an environment with NVIDIA driver
// configuration for testing.
func ValidEnvironmentWithNVDriver() *v1alpha1.Environment {
	env := ValidAWSEnvironment()
	env.Spec.NVIDIADriver = v1alpha1.NVIDIADriver{
		Install: true,
	}
	return env
}

// ValidEnvironmentWithContainerToolkit returns an environment with container
// toolkit configuration for testing.
func ValidEnvironmentWithContainerToolkit() *v1alpha1.Environment {
	env := ValidAWSEnvironment()
	env.Spec.NVIDIAContainerToolkit = v1alpha1.NVIDIAContainerToolkit{
		Install: true,
	}
	return env
}

// EnvironmentWithStatus returns an environment with status properties set.
func EnvironmentWithStatus() *v1alpha1.Environment {
	env := ValidAWSEnvironment()
	env.Status = v1alpha1.EnvironmentStatus{
		Properties: []v1alpha1.Properties{
			{Name: "vpc-id", Value: "vpc-12345"},
			{Name: "subnet-id", Value: "subnet-12345"},
			{Name: "internet-gateway-id", Value: "igw-12345"},
			{Name: "route-table-id", Value: "rtb-12345"},
			{Name: "security-group-id", Value: "sg-12345"},
			{Name: "instance-id", Value: "i-12345"},
			{Name: "public-dns-name", Value: "ec2-1-2-3-4.compute.amazonaws.com"},
		},
		Conditions: []metav1.Condition{
			{
				Type:   v1alpha1.ConditionAvailable,
				Status: metav1.ConditionTrue,
			},
		},
	}
	return env
}

// InvalidEnvironment returns an intentionally invalid environment for
// testing error handling.
func InvalidEnvironment() *v1alpha1.Environment {
	return &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: "",
		},
	}
}

// SampleYAMLEnvironment returns a YAML string representation of a valid
// environment for testing file parsing.
func SampleYAMLEnvironment() string {
	return `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
  region: us-east-1
  instance:
    type: t3.medium
  auth:
    keyName: test-key
    privateKey: /path/to/key.pem
    username: ubuntu
`
}

// SampleCacheYAML returns a YAML string representation of a cached
// environment with status for testing.
func SampleCacheYAML() string {
	return `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
  region: us-east-1
status:
  properties:
    - name: vpc-id
      value: vpc-12345
    - name: subnet-id
      value: subnet-12345
    - name: instance-id
      value: i-12345
    - name: public-dns-name
      value: ec2-1-2-3-4.compute.amazonaws.com
`
}
