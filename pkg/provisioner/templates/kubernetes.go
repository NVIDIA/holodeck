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

package templates

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

const KubeadmTemplate = `

# Install kubeadm, kubectl, and k8s-cni
: ${K8S_VERSION:={{.Version}}}
: ${CNI_PLUGINS_VERSION:={{.CniPluginsVersion}}}
: ${CALICO_VERSION:={{.CalicoVersion}}}
: ${CRICTL_VERSION:={{.CrictlVersion}}}
: ${ARCH:={{.Arch}}} # amd64, arm64, ppc64le, s390x
: ${KUBELET_RELEASE_VERSION:={{.KubeletReleaseVersion}}} # v0.17.1

# Disable swap
# see https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#before-you-begin
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

# Configure persistent loading of modules
sudo tee /etc/modules-load.d/k8s.conf <<EOF
overlay
br_netfilter
EOF

# Ensure you load modules
sudo modprobe overlay
sudo modprobe br_netfilter

# Set up required sysctl params
sudo tee /etc/sysctl.d/kubernetes.conf<<EOF
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF

# Reload sysctl
sudo sysctl --system

# Install CNI plugins (required for most pod network):
DEST="/opt/cni/bin"
sudo mkdir -p "$DEST"
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz" | sudo tar -C "$DEST" -xz

# Install crictl (required for kubeadm / Kubelet Container Runtime Interface (CRI))
DOWNLOAD_DIR="/usr/local/bin"
sudo mkdir -p "$DOWNLOAD_DIR"
curl -L "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" | sudo tar -C $DOWNLOAD_DIR -xz

# Install kubeadm, kubelet, kubectl and add a kubelet systemd service:
# see https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#installing-kubeadm-kubelet-and-kubectl
cd $DOWNLOAD_DIR
sudo curl -L --remote-name-all https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${ARCH}/{kubeadm,kubelet,kubectl}
sudo chmod +x {kubeadm,kubelet,kubectl}

curl -sSL "https://raw.githubusercontent.com/kubernetes/release/${KUBELET_RELEASE_VERSION}/cmd/krel/templates/latest/kubelet/kubelet.service" | sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | sudo tee /etc/systemd/system/kubelet.service
sudo mkdir -p /etc/systemd/system/kubelet.service.d
curl -sSL "https://raw.githubusercontent.com/kubernetes/release/${KUBELET_RELEASE_VERSION}/cmd/krel/templates/latest/kubeadm/10-kubeadm.conf" | sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | sudo tee /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
sudo systemctl enable --now kubelet

# Start kubernetes
with_retry 3 10s sudo kubeadm init --config /etc/kubernetes/kubeadm-config.yaml
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
export KUBECONFIG="${HOME}/.kube/config"

# Install Calico
# based on https://docs.tigera.io/calico/latest/getting-started/kubernetes/quickstart
with_retry 3 10s kubectl --kubeconfig $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml
# Calico CRDs created. Now we sleep for 10s to ensure they are fully registered in the K8s etcd
sleep 10s
with_retry 3 10s kubectl --kubeconfig $KUBECONFIG apply -f https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/custom-resources.yaml
# Make single-node cluster schedulable
kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule-
kubectl label node --all node-role.kubernetes.io/worker=
kubectl label node --all nvidia.com/holodeck.managed=true
`

const KindTemplate = `

: ${INSTANCE_ENDPOINT_HOST:={{.K8sEndpointHost}}}

KIND_CONFIG=""
if [ -n "{{.KindConfig}}" ]; then
  KIND_CONFIG="--config /etc/kubernetes/kind.yaml"
fi

# Download kind
[ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
[ $(uname -m) = aarch64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-arm64
chmod +x ./kind
sudo install ./kind /usr/local/bin/kind

# Install crictl (required for kubeadm / Kubelet Container Runtime Interface (CRI))
DOWNLOAD_DIR="/usr/local/bin"
sudo mkdir -p "$DOWNLOAD_DIR"

# Install kubectl 
# see https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#installing-kubeadm-kubelet-and-kubectl
cd $DOWNLOAD_DIR
sudo curl -L --remote-name-all https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${ARCH}/kubectl
sudo chmod +x kubectl
cd $HOME

# Enable NVIDIA GPU support
sudo nvidia-ctk runtime configure --set-as-default
sudo systemctl restart docker
sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts --in-place

# Create a cluster with the config file
export KUBECONFIG="${HOME}/.kube/config:/var/run/kubernetes/admin.kubeconfig"
mkdir -p $HOME/.kube
sudo chown -R $(id -u):$(id -g) $HOME/.kube/
with_retry 3 10s kind create cluster --name holodeck $KIND_CONFIG --kubeconfig="${HOME}/.kube/config"

echo "KIND installed successfully"
echo "you can now access the cluster with:"
echo "ssh -i <your-private-key> ubuntu@${INSTANCE_ENDPOINT_HOST}"
`

const microk8sTemplate = `

: ${INSTANCE_ENDPOINT_HOST:={{.K8sEndpointHost}}}

# Install microk8s
sudo apt-get update

sudo snap install microk8s --classic --channel={{.Version}}
sudo microk8s enable gpu dashboard dns registry
sudo usermod -a -G microk8s ubuntu
mkdir -p ~/.kube
sudo chown -f -R ubuntu ~/.kube
sudo microk8s config > ~/.kube/config
sudo chown -f -R ubuntu ~/.kube
sudo snap alias microk8s.kubectl kubectl

echo "Microk8s {{.Version}} installed successfully"
echo "you can now access the cluster with:"
echo "ssh -i <your-private-key> ubuntu@${INSTANCE_ENDPOINT_HOST}"
`

const kubeadmTemplate = `apiVersion: kubeadm.k8s.io/v1beta4
kind: InitConfiguration
nodeRegistration:
  criSocket: "{{ .CriSocket }}"
---
apiVersion: kubeadm.k8s.io/v1beta4
kind: ClusterConfiguration
clusterName: "{{ .ClusterName }}"
kubernetesVersion: "{{.KubernetesVersion}}"
controlPlaneEndpoint: "{{.ControlPlaneEndpoint}}:6443"
networking:
  podSubnet: "{{ .PodSubnet }}"
{{- if .FeatureGates }}
apiServer:
  extraArgs:
    - name: "feature-gates"
      value: "{{ .FeatureGates }}"
    - name: "runtime-config"
      value: "{{ .RuntimeConfig }}"
controllerManager:
  extraArgs:
    - name: "feature-gates"
      value: "{{ .FeatureGates }}"
scheduler:
  extraArgs:
    - name: "feature-gates"
      value: "{{ .FeatureGates }}"
{{- end }}
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
{{- if .ParsedFeatureGates }}
featureGates:
  {{- range $key, $value := .ParsedFeatureGates }}
  {{ $key }}: {{ $value }}
  {{- end }}
{{- end }}
`

// Default Versions
const (
	defaultArch                  = "amd64"
	defaultKubernetesVersion     = "v1.32.1"
	defaultKubeletReleaseVersion = "v0.17.1"
	defaultCNIPluginsVersion     = "v1.6.2"
	defaultCRIVersion            = "v1.31.1"
	defaultCalicoVersion         = "v3.29.1"
)

type Kubernetes struct {
	Version               string
	Installer             string
	KubeletReleaseVersion string
	Arch                  string
	CniPluginsVersion     string
	CalicoVersion         string
	CrictlVersion         string
	K8sEndpointHost       string
	K8sFeatureGates       string
	// Kind exclusive
	KindConfig string
}

// KubeadmConfig holds configuration values for kubeadm
type KubeadmConfig struct {
	// Template defines the kubeadm configuration
	// template to use for generating the configuration
	// if no template is provided, the default template
	// is used
	Template string

	CriSocket            string
	ClusterName          string
	KubernetesVersion    string
	ControlPlaneEndpoint string
	PodSubnet            string
	FeatureGates         string // Feature gates as comma-separated string
	RuntimeConfig        string // Runtime config (for feature gates) resource.k8s.io/v1beta1=true
}

func NewKubernetes(env v1alpha1.Environment) (*Kubernetes, error) {
	kubernetes := &Kubernetes{
		Version: env.Spec.Kubernetes.KubernetesVersion,
	}
	// check if env.Spec.Kubernetes.KubernetesVersion is in the format of vX.Y.Z
	// if not, set the default version
	if !strings.HasPrefix(env.Spec.Kubernetes.KubernetesVersion, "v") && env.Spec.Kubernetes.KubernetesInstaller != "microk8s" {
		fmt.Printf("Kubernetes version %s is not in the format of vX.Y.Z, setting default version v1.32.1\n", env.Spec.Kubernetes.KubernetesVersion)
		kubernetes.Version = defaultKubernetesVersion
	}
	if env.Spec.Kubernetes.KubeletReleaseVersion != "" {
		kubernetes.KubeletReleaseVersion = env.Spec.Kubernetes.KubeletReleaseVersion
	} else {
		kubernetes.KubeletReleaseVersion = defaultKubeletReleaseVersion
	}
	if env.Spec.Kubernetes.Arch != "" {
		kubernetes.Arch = env.Spec.Kubernetes.Arch
	} else {
		kubernetes.Arch = "amd64"
	}
	if env.Spec.Kubernetes.CniPluginsVersion != "" {
		kubernetes.CniPluginsVersion = env.Spec.Kubernetes.CniPluginsVersion
	} else {
		kubernetes.CniPluginsVersion = defaultCNIPluginsVersion
	}
	if env.Spec.Kubernetes.CalicoVersion != "" {
		kubernetes.CalicoVersion = env.Spec.Kubernetes.CalicoVersion
	} else {
		kubernetes.CalicoVersion = defaultCalicoVersion
	}
	if env.Spec.Kubernetes.CrictlVersion != "" {
		kubernetes.CrictlVersion = env.Spec.Kubernetes.CrictlVersion
	} else {
		kubernetes.CrictlVersion = defaultCRIVersion
	}
	if env.Spec.Kubernetes.K8sEndpointHost != "" {
		kubernetes.K8sEndpointHost = env.Spec.Kubernetes.K8sEndpointHost
	}
	if env.Spec.Kubernetes.K8sFeatureGates != nil {
		kubernetes.K8sFeatureGates = strings.Join(env.Spec.Kubernetes.K8sFeatureGates, ",")
	}
	if env.Spec.Kubernetes.KindConfig != "" {
		kubernetes.KindConfig = env.Spec.Kubernetes.KindConfig
	}

	return kubernetes, nil
}

func (k *Kubernetes) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	kubernetesTemplate := new(template.Template)

	switch env.Spec.Kubernetes.KubernetesInstaller {
	case "kubeadm":
		kubernetesTemplate = template.Must(template.New("kubeadm").Parse(KubeadmTemplate))
	case "kind":
		kubernetesTemplate = template.Must(template.New("kind").Parse(KindTemplate))
	case "microk8s":
		kubernetesTemplate = template.Must(template.New("microk8s").Parse(microk8sTemplate))
	default:
		return fmt.Errorf("unknown kubernetes installer %s", env.Spec.Kubernetes.KubernetesInstaller)
	}

	err := kubernetesTemplate.Execute(tpl, k)
	if err != nil {
		return fmt.Errorf("failed to execute kubernetes template: %v", err)
	}

	return nil
}

// NewKubeadmConfig initializes a KubeadmConfig from a Kubernetes struct
func NewKubeadmConfig(env v1alpha1.Environment) (*KubeadmConfig, error) {
	// Convert feature gates slice into a comma-separated string
	featureGates := strings.Join(env.Spec.Kubernetes.K8sFeatureGates, ",")

	criSocket, err := GetCRISocket(string(env.Spec.ContainerRuntime.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to get CRI socket: %w", err)
	}

	kConfig := &KubeadmConfig{
		CriSocket:            criSocket, // Assuming containerd as default
		ClusterName:          "holodeck-cluster",
		KubernetesVersion:    env.Spec.Kubernetes.KubernetesVersion, // Uses provided Kubernetes version
		ControlPlaneEndpoint: env.Spec.Kubernetes.K8sEndpointHost,
		PodSubnet:            "192.168.0.0/16",               // Default subnet, modify if needed
		FeatureGates:         featureGates,                   // Convert slice to string for kubeadm
		RuntimeConfig:        "resource.k8s.io/v1beta1=true", // Example runtime config
	}

	if env.Spec.Kubernetes.KubernetesVersion == "" {
		kConfig.KubernetesVersion = defaultKubernetesVersion
	}

	if env.Spec.Kubernetes.KubeAdmConfig != "" {
		// open local file for reading
		// first check if file path is relative or absolute
		// if relative, then prepend the current working directory
		kubeAdmConfigPath := env.Spec.Kubernetes.KubeAdmConfig
		if !filepath.IsAbs(kubeAdmConfigPath) {
			cwd, err := os.Getwd()
			if err != nil {
				return &KubeadmConfig{}, fmt.Errorf("failed to get current working directory: %v", err)
			}

			kubeAdmConfigPath = filepath.Join(cwd, strings.TrimPrefix(env.Spec.Kubernetes.KubeAdmConfig, "./"))
		}
		data, err := os.ReadFile(kubeAdmConfigPath) // nolint:gosec
		if err != nil {
			return &KubeadmConfig{}, fmt.Errorf("failed to read kubeadm config file: %v", err)
		}

		kConfig.Template = string(data)

	} else {
		kConfig.Template = kubeadmTemplate
	}

	return kConfig, nil
}

// ParseFeatureGates converts "Feature1=true,Feature2=false" to a map of strings for YAML
func (c *KubeadmConfig) ParseFeatureGates() map[string]string {
	parsed := make(map[string]string)
	if c.FeatureGates == "" {
		return parsed
	}
	// Split "FeatureX=true,FeatureY=false" into a map
	for _, kv := range strings.Split(c.FeatureGates, ",") {
		parts := strings.Split(kv, "=")
		if len(parts) == 2 {
			parsed[parts[0]] = parts[1] // Store values as "true"/"false" strings
		}
	}
	return parsed
}

// GenerateKubeadmConfig generates the kubeadm YAML configuration
func (c *KubeadmConfig) GenerateKubeadmConfig() (string, error) {
	// Parse feature gates into a structured map for YAML formatting
	data := struct {
		*KubeadmConfig
		ParsedFeatureGates map[string]string
	}{
		KubeadmConfig:      c,
		ParsedFeatureGates: c.ParseFeatureGates(),
	}

	// Parse and execute the template
	tmpl, err := template.New("kubeadmConfig").Parse(c.Template)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return output.String(), nil
}

// GetCRISocket returns the CRI socket path based on the container runtime
func GetCRISocket(runtime string) (string, error) {
	switch runtime {
	case "docker":
		return "unix:///run/cri-dockerd.sock", nil
	case "containerd":
		return "unix:///run/containerd/containerd.sock", nil
	case "crio":
		return "unix:///run/crio/crio.sock", nil
	default:
		return "", fmt.Errorf("unsupported container runtime: %s", runtime)
	}
}
