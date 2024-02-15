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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentSpec defines the desired state of infra provider
type EnvironmentSpec struct {
	// +kubebuilder:validation:Enum=aws;ssh
	Provider Provider `json:"provider"`

	Auth `json:"auth"`
	// +optional
	Instance `json:"instance"`

	// +optional
	NVDriver NVDriver `json:"nvidiaDriver"`
	// +optional
	ContainerRuntime ContainerRuntime `json:"containerRuntime"`
	// +optional
	NVContainerToolkit NVContainerToolkit `json:"nvidiaContainerToolkit"`
	// +optional
	Kubernetes Kubernetes `json:"kubernetes"`
}

type Instance struct {
	Type   string `json:"type"`
	Image  Image  `json:"image"`
	Region string `json:"region"`

	// +optional
	IngresIpRanges []string `json:"ingressIpRanges"`
	// +optional
	HostUrl string `json:"hostUrl"`
}

type Provider string

const (
	// ProviderAWS means the infra provider is AWS
	ProviderAWS Provider = "aws"
	// ProviderSSH means the user already has a running instance
	// and wants to use it as the infra provider via SSH
	ProviderSSH Provider = "ssh"
)

// Describes an image.
type Image struct {
	// The architecture of the image.
	Architecture string `json:"architecture"`

	// The date and time the image was created.
	CreationDate *string `json:"creationDate"`

	// The description of the AMI that was provided during image creation.
	Description *string `json:"description"`

	// The ID of the AMI.
	ImageId *string `json:"imageId"`

	// The location of the AMI.
	ImageLocation *string `json:"imageLocation"`

	// The name of the AMI that was provided during image creation.
	Name *string `json:"name"`

	// The ID of the Amazon Web Services account that owns the image.
	OwnerId *string `json:"ownerId"`
}

// EnvironmentStatus defines the observed state of the infra provider
type EnvironmentStatus struct {
	// +listType=map
	// +listMapKey=name
	Properties []Properties `json:"properties"`
	// Conditions represents the latest available observations of current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Environment is the Schema for the Holodeck Environment API
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EnvironmentList contains a list of Holodeck
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}

type Properties struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Auth struct {
	KeyName string `json:"keyName"`
	// Username for the SSH connection
	Username string `json:"username"`
	// Path to the public key file on the local machine
	PublicKey string `json:"publicKey"`
	// Path to the private key file on the local machine
	PrivateKey string `json:"privateKey"`
}

type NVDriver struct {
	Install bool `json:"install"`
	// If not set the latest stable version will be used
	// +optional
	Version string `json:"version"`
}

type ContainerRuntime struct {
	Install bool `json:"install"`
	// +kubebuilder:validation:Enum=docker;containerd;crio
	Name ContainerRuntimeName `json:"name"`
	// If not set the latest stable version will be used
	// +optional
	Version string `json:"version"`
}

type ContainerRuntimeName string

const (
	// ContainerRuntimeDocker means the container runtime is Docker
	ContainerRuntimeDocker ContainerRuntimeName = "docker"
	// ContainerRuntimeContainerd means the container runtime is Containerd
	ContainerRuntimeContainerd ContainerRuntimeName = "containerd"
	// ContainerRuntimeCrio means the container runtime is Crio
	ContainerRuntimeCrio ContainerRuntimeName = "crio"
	// ContainerRuntimeNone means the container runtime is not defined
	ContainerRuntimeNone ContainerRuntimeName = ""
)

type Kubernetes struct {
	Install bool `json:"install"`
	// KubeConfig is the path to the kubeconfig file on the local machine
	KubeConfig            string   `json:"kubeConfig"`
	KubernetesFeatures    []string `json:"Features"`
	KubernetesVersion     string   `json:"Version"`
	KubernetesInstaller   string   `json:"Installer"`
	KubeletReleaseVersion string   `json:"KubeletReleaseVersion"`
	Arch                  string   `json:"Arch"`
	CniPluginsVersion     string   `json:"CniPluginsVersion"`
	CalicoVersion         string   `json:"CalicoVersion"`
	CrictlVersion         string   `json:"CrictlVersion"`
	K8sEndpointHost       string   `json:"K8sEndpointHost"`
	// A set of key=value pairs that describe feature gates for
	// alpha/experimental features
	K8sFeatureGates []string `json:"K8sFeatureGates"`

	// Kind exclusive
	KindConfig string `json:"kindConfig"`
}

type ExtraPortMapping struct {
	ContainerPort int `json:"containerPort"`
	HostPort      int `json:"hostPort"`
}

type NVContainerToolkit struct {
	Install bool `json:"install"`
	// If not set the latest stable version will be used
	// +optional
	Version string `json:"version"`
}
