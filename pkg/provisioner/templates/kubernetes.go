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
KUBEADMIN_OPTIONS="--node-name=holodeck --pod-network-cidr=192.168.0.0/16 --ignore-preflight-errors=all --control-plane-endpoint={{.K8sEndpointHost}}:6443"
# If K8S_FEATURE_GATES is set and not empty, add it to the kubeadm init options
if [ -n "{{.K8sFeatureGates}}"]; then
  KUBEADMIN_OPTIONS="${KUBEADMIN_OPTIONS} --feature-gates={{.K8sFeatureGates}}"
fi
with_retry 3 10s sudo kubeadm init ${KUBEADMIN_OPTIONS} 
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
export KUBECONFIG="${HOME}/.kube/config"

# Install Calico
# based on https://docs.tigera.io/calico/latest/getting-started/kubernetes/quickstart
with_retry 3 10s kubectl --kubeconfig $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml
with_retry 3 10s kubectl --kubeconfig $KUBECONFIG create -f https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/custom-resources.yaml
# Make single-node cluster schedulable
kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule-
kubectl label node --all node-role.kubernetes.io/worker=
kubectl label node --all nvidia.com/holodeck.managed=true
`

const KindTemplate = `

: ${INSTANCE_ENDPOINT_HOST:={{.K8sEndpointHost}}}
KIND_CONFIG=""
if [ -n "{{.KindConfig}}"]; then
  KIND_CONFIG="--config {{.KindConfig}}"
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

// Default Versions
const (
	defaultArch                  = "amd64"
	defaultKubernetesVersion     = "v1.30.2"
	defaultKubeletReleaseVersion = "v0.17.1"
	defaultCNIPluginsVersion     = "v1.5.1"
	defaultCRIVersion            = "v1.30.0"
	defaultCalicoVersion         = "v3.28.1"
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

func NewKubernetes(env v1alpha1.Environment) (*Kubernetes, error) {
	kubernetes := &Kubernetes{
		Version: env.Spec.Kubernetes.KubernetesVersion,
	}
	// check if env.Spec.Kubernetes.KubernetesVersion is in the format of vX.Y.Z
	// if not, set the default version
	if !strings.HasPrefix(env.Spec.Kubernetes.KubernetesVersion, "v") && env.Spec.Kubernetes.KubernetesInstaller != "microk8s" {
		fmt.Printf("Kubernetes version %s is not in the format of vX.Y.Z, setting default version v1.27.9\n", env.Spec.Kubernetes.KubernetesVersion)
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
