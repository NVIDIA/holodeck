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
	// Instance is required for AWS provider (single-node mode)
	// +optional
	Instance `json:"instance"`

	// Cluster defines multi-node cluster configuration.
	// When specified, Instance is ignored and nodes are created based on
	// the cluster specification.
	// +optional
	Cluster *ClusterSpec `json:"cluster,omitempty"`

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

// Instance defines an AWS instance
type Instance struct {
	Type   string `json:"type"`
	Region string `json:"region"`

	// OS specifies the operating system by ID (e.g., "ubuntu-22.04").
	// When set, the AMI is automatically resolved for the region and
	// architecture. Takes precedence over Image.ImageId if both are specified.
	// Run 'holodeck os list' for available options.
	// +optional
	OS string `json:"os,omitempty"`

	// Image allows explicit AMI specification. If OS is set, ImageId is
	// automatically resolved and other Image fields are ignored.
	// +optional
	Image Image `json:"image"`

	// +optional
	IngressIpRanges []string `json:"ingressIpRanges"`
	// +optional
	HostUrl string `json:"hostUrl"`
	// +optional
	// if not set, the default size is 64GB
	RootVolumeSizeGB *int32 `json:"rootVolumeSizeGB"`
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

// EtcdTopology defines where etcd runs in a HA cluster.
// +kubebuilder:validation:Enum=stacked;external
type EtcdTopology string

const (
	// EtcdTopologyStacked runs etcd on control-plane nodes (simpler, default)
	EtcdTopologyStacked EtcdTopology = "stacked"
	// EtcdTopologyExternal runs etcd on separate dedicated nodes
	EtcdTopologyExternal EtcdTopology = "external"
)

// ClusterSpec defines multi-node cluster configuration.
type ClusterSpec struct {
	// Region specifies the AWS region for all cluster nodes.
	// +required
	Region string `json:"region"`

	// ControlPlane defines the control-plane node configuration.
	// +required
	ControlPlane ControlPlaneSpec `json:"controlPlane"`

	// Workers defines the worker node pool configuration.
	// +optional
	Workers *WorkerPoolSpec `json:"workers,omitempty"`

	// HighAvailability configures HA settings for the control plane.
	// +optional
	HighAvailability *HAConfig `json:"highAvailability,omitempty"`
}

// ControlPlaneSpec defines control-plane node configuration.
type ControlPlaneSpec struct {
	// Count is the number of control-plane nodes.
	// For HA, use an odd number (1, 3, 5) to maintain etcd quorum.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=7
	// +kubebuilder:default=1
	Count int32 `json:"count"`

	// InstanceType specifies the EC2 instance type for control-plane nodes.
	// +kubebuilder:default="m5.xlarge"
	// +optional
	InstanceType string `json:"instanceType,omitempty"`

	// OS specifies the operating system by ID (e.g., "ubuntu-22.04").
	// When set, the AMI is automatically resolved for the region and
	// architecture.
	// +optional
	OS string `json:"os,omitempty"`

	// Image allows explicit AMI specification. If OS is set, this is ignored.
	// +optional
	Image *Image `json:"image,omitempty"`

	// Dedicated indicates whether control-plane nodes should be tainted
	// to prevent workload scheduling (NoSchedule taint).
	// +kubebuilder:default=false
	// +optional
	Dedicated bool `json:"dedicated,omitempty"`

	// RootVolumeSizeGB specifies the root volume size in GB.
	// +kubebuilder:default=64
	// +optional
	RootVolumeSizeGB *int32 `json:"rootVolumeSizeGB,omitempty"`

	// Labels are additional Kubernetes labels to apply to control-plane nodes.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// WorkerPoolSpec defines worker node pool configuration.
type WorkerPoolSpec struct {
	// Count is the number of worker nodes.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Count int32 `json:"count"`

	// InstanceType specifies the EC2 instance type for worker nodes.
	// For GPU workloads, use GPU instance types (g4dn, p4d, etc.).
	// +kubebuilder:default="g4dn.xlarge"
	// +optional
	InstanceType string `json:"instanceType,omitempty"`

	// OS specifies the operating system by ID (e.g., "ubuntu-22.04").
	// When set, the AMI is automatically resolved for the region and
	// architecture.
	// +optional
	OS string `json:"os,omitempty"`

	// Image allows explicit AMI specification. If OS is set, this is ignored.
	// +optional
	Image *Image `json:"image,omitempty"`

	// RootVolumeSizeGB specifies the root volume size in GB.
	// +kubebuilder:default=64
	// +optional
	RootVolumeSizeGB *int32 `json:"rootVolumeSizeGB,omitempty"`

	// Labels are additional Kubernetes labels to apply to worker nodes.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HAConfig defines high availability configuration for the control plane.
type HAConfig struct {
	// Enabled activates high availability mode.
	// When true, multiple control-plane nodes are configured with leader
	// election and an API server load balancer is created.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// EtcdTopology specifies where etcd runs.
	// - stacked: etcd runs on control-plane nodes (default, simpler)
	// - external: etcd runs on separate dedicated nodes (for large clusters)
	// +kubebuilder:default=stacked
	// +optional
	EtcdTopology EtcdTopology `json:"etcdTopology,omitempty"`

	// LoadBalancerType specifies the type of load balancer for the API server.
	// +kubebuilder:validation:Enum=nlb;alb
	// +kubebuilder:default=nlb
	// +optional
	LoadBalancerType string `json:"loadBalancerType,omitempty"`
}

// NodeStatus represents the status of a single node in the cluster.
type NodeStatus struct {
	// Name is the node's hostname or identifier.
	Name string `json:"name"`

	// Role indicates whether this is a control-plane or worker node.
	// +kubebuilder:validation:Enum=control-plane;worker
	Role string `json:"role"`

	// InstanceID is the cloud provider's instance identifier.
	InstanceID string `json:"instanceId,omitempty"`

	// PublicIP is the node's public IP address.
	PublicIP string `json:"publicIp,omitempty"`

	// PrivateIP is the node's private IP address within the VPC.
	PrivateIP string `json:"privateIp,omitempty"`

	// Phase indicates the current lifecycle phase of the node.
	// +kubebuilder:validation:Enum=Pending;Provisioning;Running;Ready;Failed;Terminating
	Phase string `json:"phase"`

	// Message provides additional details about the current phase.
	// +optional
	Message string `json:"message,omitempty"`
}

// ClusterStatus represents the status of a multi-node cluster.
type ClusterStatus struct {
	// Nodes contains the status of each node in the cluster.
	// +optional
	Nodes []NodeStatus `json:"nodes,omitempty"`

	// ControlPlaneEndpoint is the API server endpoint (load balancer DNS for HA).
	// +optional
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint,omitempty"`

	// LoadBalancerDNS is the DNS name of the HA load balancer (if HA enabled).
	// +optional
	LoadBalancerDNS string `json:"loadBalancerDns,omitempty"`

	// TotalNodes is the total number of nodes in the cluster.
	TotalNodes int32 `json:"totalNodes,omitempty"`

	// ReadyNodes is the number of nodes in Ready state.
	ReadyNodes int32 `json:"readyNodes,omitempty"`

	// Phase indicates the overall cluster lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Creating;Provisioning;Ready;Degraded;Deleting;Failed
	// +optional
	Phase string `json:"phase,omitempty"`
}

// EnvironmentStatus defines the observed state of the infra provider
type EnvironmentStatus struct {
	// +listType=map
	// +listMapKey=name
	Properties []Properties `json:"properties"`
	// Conditions represents the latest available observations of current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Cluster contains status information for multi-node clusters.
	// Only populated when spec.cluster is defined.
	// +optional
	Cluster *ClusterStatus `json:"cluster,omitempty"`
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
	// Username for the SSH connection.
	// Auto-detected from OS if not specified and OS field is set.
	// +optional
	Username string `json:"username,omitempty"`
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

// K8sSource defines the installation source for Kubernetes.
// +kubebuilder:validation:Enum=release;git;latest
type K8sSource string

const (
	// K8sSourceRelease installs from official releases (default)
	K8sSourceRelease K8sSource = "release"
	// K8sSourceGit installs from a specific git reference
	K8sSourceGit K8sSource = "git"
	// K8sSourceLatest tracks a moving branch at provision time
	K8sSourceLatest K8sSource = "latest"
)

// K8sReleaseSpec defines configuration for release-based installation.
type K8sReleaseSpec struct {
	// Version specifies the Kubernetes version (e.g., "v1.31.0").
	// +required
	Version string `json:"version"`
}

// K8sGitSpec defines configuration for git-based installation.
type K8sGitSpec struct {
	// Repo is the git repository URL.
	// +kubebuilder:default="https://github.com/kubernetes/kubernetes.git"
	// +optional
	Repo string `json:"repo,omitempty"`

	// Ref is the git reference (commit SHA, tag, branch, or PR ref).
	// Examples: "v1.32.0-alpha.1", "refs/tags/v1.31.0", "refs/heads/master",
	//           "abc123", "refs/pull/123456/head"
	// +required
	Ref string `json:"ref"`
}

// K8sLatestSpec defines configuration for latest branch tracking.
type K8sLatestSpec struct {
	// Track specifies the branch to track at provision time.
	// +kubebuilder:default=master
	// +optional
	Track string `json:"track,omitempty"`

	// Repo is the git repository URL.
	// +kubebuilder:default="https://github.com/kubernetes/kubernetes.git"
	// +optional
	Repo string `json:"repo,omitempty"`
}

// Kubernetes defines the Kubernetes cluster configuration.
type Kubernetes struct {
	Install bool `json:"install"`

	// Source determines installation method.
	// +kubebuilder:default=release
	// +optional
	Source K8sSource `json:"source,omitempty"`

	// Release source configuration (when source=release).
	// +optional
	Release *K8sReleaseSpec `json:"release,omitempty"`

	// Git source configuration (when source=git).
	// +optional
	Git *K8sGitSpec `json:"git,omitempty"`

	// Latest source configuration (when source=latest).
	// +optional
	Latest *K8sLatestSpec `json:"latest,omitempty"`

	// KubernetesInstaller specifies the installer to use.
	// +kubebuilder:validation:Enum=kubeadm;kind;microk8s
	// +kubebuilder:default=kubeadm
	// +optional
	KubernetesInstaller string `json:"Installer,omitempty"`

	// KubeConfig is the path to the kubeconfig file on the local machine.
	// +optional
	KubeConfig string `json:"kubeConfig,omitempty"`

	// KubernetesVersion is deprecated, use Release.Version instead.
	// Preserved for backward compatibility.
	// +optional
	KubernetesVersion string `json:"Version,omitempty"`

	// KubernetesFeatures is a list of Kubernetes features to enable.
	// +optional
	KubernetesFeatures []string `json:"Features,omitempty"`

	// KubeletReleaseVersion specifies the kubelet release version.
	// +optional
	KubeletReleaseVersion string `json:"KubeletReleaseVersion,omitempty"`

	// Arch specifies the architecture (e.g., "amd64", "arm64").
	// +optional
	Arch string `json:"Arch,omitempty"`

	// CniPluginsVersion specifies the CNI plugins version.
	// +optional
	CniPluginsVersion string `json:"CniPluginsVersion,omitempty"`

	// CalicoVersion specifies the Calico version.
	// +optional
	CalicoVersion string `json:"CalicoVersion,omitempty"`

	// CrictlVersion specifies the crictl version.
	// +optional
	CrictlVersion string `json:"CrictlVersion,omitempty"`

	// K8sEndpointHost is the Kubernetes API endpoint host.
	// +optional
	K8sEndpointHost string `json:"K8sEndpointHost,omitempty"`

	// K8sFeatureGates is a set of key=value pairs that describe feature gates
	// for alpha/experimental features.
	// +optional
	K8sFeatureGates []string `json:"K8sFeatureGates,omitempty"`

	// KubeAdmConfig is the path to the kubeadm config file on the local machine.
	// +optional
	KubeAdmConfig string `json:"kubeAdmConfig,omitempty"`

	// KindConfig is the path to the KIND config file (KIND installer only).
	// +optional
	KindConfig string `json:"kindConfig,omitempty"`
}

type ExtraPortMapping struct {
	ContainerPort int `json:"containerPort"`
	HostPort      int `json:"hostPort"`
}

// CTKSource defines where to install the NVIDIA Container Toolkit from.
// +kubebuilder:validation:Enum=package;git;latest
type CTKSource string

const (
	// CTKSourcePackage installs from distribution packages (default)
	CTKSourcePackage CTKSource = "package"
	// CTKSourceGit installs from a specific git reference
	CTKSourceGit CTKSource = "git"
	// CTKSourceLatest tracks a moving branch at provision time
	CTKSourceLatest CTKSource = "latest"
)

// CTKPackageSpec defines configuration for package-based installation.
type CTKPackageSpec struct {
	// Channel selects stable or experimental packages.
	// +kubebuilder:validation:Enum=stable;experimental
	// +kubebuilder:default=stable
	// +optional
	Channel string `json:"channel,omitempty"`

	// Version pins to a specific package version (e.g., "1.17.3-1").
	// +optional
	Version string `json:"version,omitempty"`
}

// CTKGitSpec defines configuration for git-based installation.
type CTKGitSpec struct {
	// Repo is the git repository URL.
	// +kubebuilder:default="https://github.com/NVIDIA/nvidia-container-toolkit.git"
	// +optional
	Repo string `json:"repo,omitempty"`

	// Ref is the git reference (commit SHA, tag, branch, or PR ref).
	// Examples: "v1.17.3", "refs/tags/v1.17.3", "refs/heads/main", "abc123"
	// +required
	Ref string `json:"ref"`
}

// CTKLatestSpec defines configuration for latest branch tracking.
type CTKLatestSpec struct {
	// Track specifies the branch to track at provision time.
	// +kubebuilder:default=main
	// +optional
	Track string `json:"track,omitempty"`

	// Repo is the git repository URL.
	// +kubebuilder:default="https://github.com/NVIDIA/nvidia-container-toolkit.git"
	// +optional
	Repo string `json:"repo,omitempty"`
}

// NVIDIAContainerToolkit defines the NVIDIA Container Toolkit configuration.
type NVIDIAContainerToolkit struct {
	Install bool `json:"install"`

	// Source determines installation method.
	// +kubebuilder:default=package
	// +optional
	Source CTKSource `json:"source,omitempty"`

	// Package source configuration (when source=package).
	// +optional
	Package *CTKPackageSpec `json:"package,omitempty"`

	// Git source configuration (when source=git).
	// +optional
	Git *CTKGitSpec `json:"git,omitempty"`

	// Latest source configuration (when source=latest).
	// +optional
	Latest *CTKLatestSpec `json:"latest,omitempty"`

	// EnableCDI enables the Container Device Interface (CDI) in the selected
	// container runtime.
	// +optional
	EnableCDI bool `json:"enableCDI,omitempty"`

	// Version is deprecated, use Package.Version instead.
	// +optional
	Version string `json:"version,omitempty"`
}

// Kernel defines the kernel configuration
type Kernel struct {
	// Version specifies the kernel version to install
	// If not set, no kernel changes will be made
	// +optional
	Version string `json:"version,omitempty"`
}
