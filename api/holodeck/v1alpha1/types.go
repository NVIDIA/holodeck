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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentSpec defines the desired state of infra provider
type EnvironmentSpec struct {
	// +kubebuilder:validation:Enum=aws;ssh
	Provider Provider `json:"provider"`

	Auth `json:"auth"`
	// Instance is required for AWS provider
	// +optional
	Instance `json:"instance"`

	// +optional
	Kernel Kernel `json:"kernel"`
	// +optional
	NVIDIADriver NVIDIADriver `json:"nvidiaDriver"`
	// +optional
	ContainerRuntime ContainerRuntime `json:"containerRuntime"`
	// +optional
	NVIDIAContainerToolkit NVIDIAContainerToolkit `json:"nvidiaContainerToolkit"`
	// +optional
	Kubernetes Kubernetes `json:"kubernetes"`
}

type Provider string

const (
	// ProviderAWS means the infra provider is AWS
	ProviderAWS Provider = "aws"
	// ProviderSSH means the user already has a running instance
	// and wants to use it as the infra provider via SSH
	ProviderSSH Provider = "ssh"

	// Possible values for the Conditions field
	ConditionProgressing string = "Progressing"
	ConditionDegraded    string = "Degraded"
	ConditionAvailable   string = "Available"
	ConditionTerminated  string = "Terminated"
)

// Instance defines and AWS instance
type Instance struct {
	Type   string `json:"type"`
	Image  Image  `json:"image"`
	Region string `json:"region"`

	// +optional
	IngressIpRanges []string `json:"ingressIpRanges"`
	// +optional
	HostUrl string `json:"hostUrl"`
}

// Describes an image or vm template.
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
	metav1.ObjectMeta `json:"metadata"`

	Spec   EnvironmentSpec   `json:"spec"`
	Status EnvironmentStatus `json:"status"`
}

//+kubebuilder:object:root=true

// EnvironmentList contains a list of Holodeck
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
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
	// KeyName for the SSH connection
	KeyName string `json:"keyName"`
	// Username for the SSH connection
	Username string `json:"username"`
	// Path to the public key file on the local machine
	PublicKey string `json:"publicKey"`
	// Path to the private key file on the local machine
	PrivateKey string `json:"privateKey"`
}

type NVIDIADriver struct {
	Install bool `json:"install"`
	// Branch specifies the driver branch.
	// If a version is specified, this takes precedence.
	// +optional
	Branch string `json:"branch"`
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

	// KubeAdmConfig is the path to the KubeAdmConfig file on the local machine
	// +optional
	KubeAdmConfig string `json:"kubeAdmConfig"`

	// Kind exclusive
	KindConfig string `json:"kindConfig"`
}

type ExtraPortMapping struct {
	ContainerPort int `json:"containerPort"`
	HostPort      int `json:"hostPort"`
}

type NVIDIAContainerToolkit struct {
	Install bool `json:"install"`
	// If not set the latest stable version will be used
	// +optional
	Version string `json:"version"`
	// EnableCDI enables the Container Device Interface (CDI) in the selected
	// container runtime.
	// +optional
	EnableCDI bool `json:"enableCDI"`
	// Source specifies how to install the NVIDIA Container Toolkit
	// +kubebuilder:validation:Enum=package;git;latest
	// +optional
	Source NCTSource `json:"source,omitempty"`
	// Package configuration for package-based installation
	// +optional
	Package *NCTPackageConfig `json:"package,omitempty"`
	// Git configuration for git-based installation
	// +optional
	Git *NCTGitConfig `json:"git,omitempty"`
	// Latest configuration for latest-tracking installation
	// +optional
	Latest *NCTLatestConfig `json:"latest,omitempty"`
}

type NCTSource string

const (
	// NCTSourcePackage installs from distro packages (default)
	NCTSourcePackage NCTSource = "package"
	// NCTSourceGit installs from a specific git ref
	NCTSourceGit NCTSource = "git"
	// NCTSourceLatest installs from latest commit of a branch
	NCTSourceLatest NCTSource = "latest"
)

type NCTPackageConfig struct {
	// Channel specifies the package channel
	// +kubebuilder:validation:Enum=stable;experimental
	// +optional
	Channel string `json:"channel,omitempty"`
	// Version specifies a specific version to pin
	// +optional
	Version string `json:"version,omitempty"`
}

type NCTGitConfig struct {
	// Repo is the git repository URL
	// +optional
	Repo string `json:"repo,omitempty"`
	// Ref is the git reference (commit SHA, tag, branch, or PR ref)
	Ref string `json:"ref"`
	// Build configuration for git-based installation
	// +optional
	Build *NCTBuildConfig `json:"build,omitempty"`
}

type NCTLatestConfig struct {
	// Track specifies the branch to track
	Track string `json:"track"`
	// Repo is the git repository URL
	// +optional
	Repo string `json:"repo,omitempty"`
}

type NCTBuildConfig struct {
	// MakeTargets specifies the make targets to build
	// +optional
	MakeTargets []string `json:"makeTargets,omitempty"`
	// ExtraEnv provides additional environment variables for the build
	// +optional
	ExtraEnv map[string]string `json:"extraEnv,omitempty"`
}

// Kernel defines the kernel configuration
type Kernel struct {
	// Version specifies the kernel version to install
	// If not set, no kernel changes will be made
	// +optional
	Version string `json:"version,omitempty"`
}
