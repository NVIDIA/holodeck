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

// containerdV1Template is used for containerd 1.x versions (default 1.7.27)
// Based on the approach from holodeck v0.2.5 that worked
const containerdV1Template = `
COMPONENT="containerd"
DESIRED_VERSION="{{.Version}}"

holodeck_progress "$COMPONENT" 1 4 "Checking existing installation"

# Check if containerd is already installed and functional
if systemctl is-active --quiet containerd 2>/dev/null; then
    INSTALLED_VERSION=$(containerd --version 2>/dev/null | awk '{print $3}' || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ -z "$DESIRED_VERSION" ]] || [[ "$INSTALLED_VERSION" == *"$DESIRED_VERSION"* ]]; then
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

            if holodeck_verify_containerd; then
                holodeck_log "INFO" "$COMPONENT" "Containerd verified functional"
                holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                exit 0
            else
                holodeck_log "WARN" "$COMPONENT" \
                    "Containerd installed but not functional, attempting repair"
            fi
        else
            holodeck_log "INFO" "$COMPONENT" \
                "Version mismatch: installed=${INSTALLED_VERSION}, desired=${DESIRED_VERSION}"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 4 "Installing containerd {{.Version}} using apt repository"

# Install required packages
holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" install_packages_with_retry ca-certificates curl gnupg

# Add Docker repository (idempotent)
if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
    sudo install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
        sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    sudo chmod a+r /etc/apt/keyrings/docker.gpg
else
    holodeck_log "INFO" "$COMPONENT" "Docker GPG key already present"
fi

if [[ ! -f /etc/apt/sources.list.d/docker.list ]]; then
    echo \
      "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
      sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    holodeck_retry 3 "$COMPONENT" sudo apt-get update
else
    holodeck_log "INFO" "$COMPONENT" "Docker repository already configured"
fi

# Install containerd with specific version if provided
if [[ -n "{{.Version}}" ]] && [[ "{{.Version}}" != "latest" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Attempting to install containerd.io={{.Version}}-1"
    if ! holodeck_retry 3 "$COMPONENT" install_packages_with_retry "containerd.io={{.Version}}-1"; then
        holodeck_log "WARN" "$COMPONENT" \
            "Specific version {{.Version}} not found, installing latest"
        holodeck_retry 3 "$COMPONENT" install_packages_with_retry containerd.io
    fi
else
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry containerd.io
fi

holodeck_progress "$COMPONENT" 3 4 "Configuring containerd"

# Configure containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null

# Set systemd as the cgroup driver
sudo sed -i 's/SystemdCgroup \= false/SystemdCgroup \= true/g' /etc/containerd/config.toml

# Ensure CNI paths are configured correctly
sudo sed -i 's|conf_dir = .*|conf_dir = "/etc/cni/net.d"|g' /etc/containerd/config.toml
sudo sed -i 's|bin_dir = .*|bin_dir = "/opt/cni/bin"|g' /etc/containerd/config.toml

# Restart containerd
sudo systemctl restart containerd
sudo systemctl enable containerd

holodeck_progress "$COMPONENT" 4 4 "Verifying installation"

# Wait for containerd to be ready with timeout
timeout=30
while ! sudo ctr version &>/dev/null; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" \
            "Timeout waiting for containerd to become ready" \
            "Check 'systemctl status containerd' and 'journalctl -u containerd'"
    fi
    sleep 1
    ((timeout--))
done

if ! holodeck_verify_containerd; then
    holodeck_error 5 "$COMPONENT" \
        "Containerd installation verification failed" \
        "Run 'systemctl status containerd' to diagnose"
fi

FINAL_VERSION=$(containerd --version | awk '{print $3}')
holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed containerd ${FINAL_VERSION}"
`

// containerdV2Template is used for containerd 2.x versions
// Based on official containerd installation guide for v2.x
const containerdV2Template = `
COMPONENT="containerd"
DESIRED_VERSION="{{.Version}}"

holodeck_progress "$COMPONENT" 1 6 "Checking existing installation"

# Check if containerd is already installed and functional
if systemctl is-active --quiet containerd 2>/dev/null; then
    INSTALLED_VERSION=$(containerd --version 2>/dev/null | awk '{print $3}' || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ -z "$DESIRED_VERSION" ]] || [[ "$INSTALLED_VERSION" == *"$DESIRED_VERSION"* ]]; then
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

            if holodeck_verify_containerd; then
                holodeck_log "INFO" "$COMPONENT" "Containerd verified functional"
                holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                exit 0
            else
                holodeck_log "WARN" "$COMPONENT" \
                    "Containerd installed but not functional, attempting repair"
            fi
        else
            holodeck_log "INFO" "$COMPONENT" \
                "Version mismatch: installed=${INSTALLED_VERSION}, desired=${DESIRED_VERSION}"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 6 "Setting up prerequisites"

# Set up kernel modules (idempotent)
sudo modprobe overlay
sudo modprobe br_netfilter

# Setup required sysctl params (idempotent)
if [[ ! -f /etc/sysctl.d/99-kubernetes-cri.conf ]]; then
    cat <<EOF | sudo tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF
    sudo sysctl --system
else
    holodeck_log "INFO" "$COMPONENT" "Sysctl params already configured"
fi

holodeck_progress "$COMPONENT" 3 6 "Installing dependencies"

holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" install_packages_with_retry ca-certificates curl

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "x86_64" ]]; then
    ARCH="amd64"
elif [[ "$ARCH" == "aarch64" ]]; then
    ARCH="arm64"
fi

holodeck_progress "$COMPONENT" 4 6 "Installing containerd {{.Version}} from official binaries"

# Download and install containerd (check if already installed)
if [[ ! -f /usr/local/bin/containerd ]] || \
   ! containerd --version 2>/dev/null | grep -q "{{.Version}}"; then
    CONTAINERD_TAR="containerd-{{.Version}}-linux-${ARCH}.tar.gz"
    CONTAINERD_URL="https://github.com/containerd/containerd/releases/download/v{{.Version}}/${CONTAINERD_TAR}"

    holodeck_log "INFO" "$COMPONENT" "Downloading containerd from $CONTAINERD_URL"
    holodeck_retry 3 "$COMPONENT" curl -fsSL -o "${CONTAINERD_TAR}" "${CONTAINERD_URL}"
    sudo tar Cxzvf /usr/local "${CONTAINERD_TAR}"
    rm -f "${CONTAINERD_TAR}"
else
    holodeck_log "INFO" "$COMPONENT" "Containerd binary already at correct version"
fi

# Download and install runc (idempotent)
RUNC_VERSION="1.2.3"
if [[ ! -f /usr/local/sbin/runc ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing runc ${RUNC_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -fsSL -o "runc.${ARCH}" \
        "https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.${ARCH}"
    sudo install -m 755 "runc.${ARCH}" /usr/local/sbin/runc
    rm -f "runc.${ARCH}"
else
    holodeck_log "INFO" "$COMPONENT" "runc already installed"
fi

# Install CNI plugins (idempotent)
CNI_VERSION="v1.6.2"
if [[ ! -f /opt/cni/bin/bridge ]]; then
    CNI_TAR="cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz"
    holodeck_log "INFO" "$COMPONENT" "Installing CNI plugins ${CNI_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -fsSL -o "${CNI_TAR}" \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/${CNI_TAR}"
    sudo mkdir -p /opt/cni/bin
    sudo tar Cxzvf /opt/cni/bin "${CNI_TAR}"
    rm -f "${CNI_TAR}"
else
    holodeck_log "INFO" "$COMPONENT" "CNI plugins already installed"
fi

holodeck_progress "$COMPONENT" 5 6 "Configuring containerd"

# Configure containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null

# Update config for systemd cgroup
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# Ensure CNI paths are configured correctly
sudo sed -i 's|conf_dir = .*|conf_dir = "/etc/cni/net.d"|g' /etc/containerd/config.toml
sudo sed -i 's|bin_dir = .*|bin_dir = "/opt/cni/bin"|g' /etc/containerd/config.toml

# Create containerd service (idempotent)
if [[ ! -f /etc/systemd/system/containerd.service ]]; then
    cat <<EOF | sudo tee /etc/systemd/system/containerd.service
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd
Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target
EOF
fi

# Start containerd
sudo systemctl daemon-reload
sudo systemctl enable --now containerd

holodeck_progress "$COMPONENT" 6 6 "Verifying installation"

# Wait for containerd to be ready with timeout
timeout=30
while ! sudo ctr version &>/dev/null; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" \
            "Timeout waiting for containerd to become ready" \
            "Check 'systemctl status containerd' and 'journalctl -u containerd'"
    fi
    sleep 1
    ((timeout--))
done

if ! holodeck_verify_containerd; then
    holodeck_error 5 "$COMPONENT" \
        "Containerd installation verification failed" \
        "Run 'systemctl status containerd' to diagnose"
fi

FINAL_VERSION=$(containerd --version | awk '{print $3}')
holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed containerd ${FINAL_VERSION} (v2.x)"
`

type Containerd struct {
	Version      string
	MajorVersion int
}

func NewContainerd(env v1alpha1.Environment) *Containerd {
	var version string

	if env.Spec.ContainerRuntime.Version == "" {
		version = "1.7.27" // Default to v1.7.x
	} else {
		// remove the 'v' prefix from the version if it exists
		version = strings.TrimPrefix(env.Spec.ContainerRuntime.Version, "v")
	}

	// Parse major version
	majorVersion := 1
	parts := strings.Split(version, ".")
	if len(parts) > 0 {
		if major := parts[0]; major == "2" {
			majorVersion = 2
			// If user specifies "2" without minor/patch, use latest stable v2
			if version == "2" {
				version = "2.0.0"
			}
		}
	}

	return &Containerd{
		Version:      version,
		MajorVersion: majorVersion,
	}
}

func (t *Containerd) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	// Choose template based on major version
	var templateContent string
	switch t.MajorVersion {
	case 2:
		templateContent = containerdV2Template
	default:
		templateContent = containerdV1Template
	}

	containerdTemplate := template.Must(template.New("containerd").Parse(templateContent))
	err := containerdTemplate.Execute(tpl, t)
	if err != nil {
		return fmt.Errorf("failed to execute containerd template: %v", err)
	}
	return nil
}
