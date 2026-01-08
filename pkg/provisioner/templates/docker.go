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
	"text/template"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

const dockerTemplate = `
COMPONENT="docker"
DESIRED_VERSION="{{.Version}}"

holodeck_progress "$COMPONENT" 1 6 "Checking existing installation"

# Check if Docker is already installed and functional
if systemctl is-active --quiet docker 2>/dev/null; then
    INSTALLED_VERSION=$(sudo docker version --format '{{"{{"}}{{".Server.Version"}}{{"}}"}}' 2>/dev/null || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ "$DESIRED_VERSION" == "latest" ]] || \
           [[ -z "$DESIRED_VERSION" ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION" ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION."* ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION-"* ]]; then
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

            if holodeck_verify_docker; then
                holodeck_log "INFO" "$COMPONENT" "Docker verified functional"
                holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                exit 0
            else
                holodeck_log "WARN" "$COMPONENT" \
                    "Docker installed but not functional, attempting repair"
            fi
        else
            holodeck_log "INFO" "$COMPONENT" \
                "Version mismatch: installed=${INSTALLED_VERSION}, desired=${DESIRED_VERSION}"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 6 "Adding Docker repository"

# Based on https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository
holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" install_packages_with_retry ca-certificates curl gnupg

# Add Docker's official GPG key (idempotent)
if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
    sudo install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
        sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    sudo chmod a+r /etc/apt/keyrings/docker.gpg
else
    holodeck_log "INFO" "$COMPONENT" "Docker GPG key already present"
fi

# Add the repository to Apt sources (idempotent)
if [[ ! -f /etc/apt/sources.list.d/docker.list ]]; then
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
      sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
fi
holodeck_retry 3 "$COMPONENT" sudo apt-get update

holodeck_progress "$COMPONENT" 3 6 "Installing Docker"

# Install Docker
if [[ "$DESIRED_VERSION" == "latest" ]] || [[ -z "$DESIRED_VERSION" ]]; then
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
        docker-ce docker-ce-cli containerd.io
else
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
        "docker-ce=$DESIRED_VERSION" "docker-ce-cli=$DESIRED_VERSION" containerd.io
fi

holodeck_progress "$COMPONENT" 4 6 "Configuring Docker"

# Create required directories
sudo mkdir -p /etc/systemd/system/docker.service.d

# Create daemon json config file (idempotent)
if [[ ! -f /etc/docker/daemon.json ]]; then
    sudo mkdir -p /etc/docker
    sudo tee /etc/docker/daemon.json > /dev/null <<EOF
{
  "exec-opts": ["native.cgroupdriver=systemd"],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m"
  },
  "storage-driver": "overlay2"
}
EOF
else
    holodeck_log "INFO" "$COMPONENT" "Docker daemon.json already exists"
fi

# Start and enable Services
sudo systemctl daemon-reload
sudo systemctl enable docker
sudo systemctl restart docker

# Wait for Docker to be ready BEFORE installing cri-dockerd (cri-dockerd depends on Docker)
# Note: Use 'sudo docker info' because usermod -aG docker doesn't apply to current session
holodeck_log "INFO" "$COMPONENT" "Waiting for Docker daemon to be ready..."
timeout=120
while ! sudo docker info &>/dev/null; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" \
            "Timeout waiting for Docker to become ready after restart" \
            "Check 'systemctl status docker' and 'journalctl -u docker'"
    fi
    if (( timeout % 15 == 0 )); then
        holodeck_log "INFO" "$COMPONENT" \
            "Waiting for Docker to become ready (${timeout}s remaining)"
    fi
    sleep 1
    ((timeout--))
done
holodeck_log "INFO" "$COMPONENT" "Docker daemon is ready"

# Post-installation steps for Linux
sudo usermod -aG docker "$USER" || true
# Note: newgrp docker would spawn a new shell, skip for idempotency

holodeck_progress "$COMPONENT" 5 6 "Installing cri-dockerd"

# Install cri-dockerd (idempotent)
CRI_DOCKERD_VERSION="0.3.17"
CRI_DOCKERD_ARCH="amd64"

if [[ ! -f /usr/local/bin/cri-dockerd ]]; then
    CRI_DOCKERD_URL="https://github.com/Mirantis/cri-dockerd/releases/download/v${CRI_DOCKERD_VERSION}/cri-dockerd-${CRI_DOCKERD_VERSION}.${CRI_DOCKERD_ARCH}.tgz"
    holodeck_log "INFO" "$COMPONENT" "Installing cri-dockerd ${CRI_DOCKERD_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -L "${CRI_DOCKERD_URL}" | \
        sudo tar xzv -C /usr/local/bin --strip-components=1
else
    holodeck_log "INFO" "$COMPONENT" "cri-dockerd already installed"
fi

# Create systemd service file for cri-dockerd (idempotent)
if [[ ! -f /etc/systemd/system/cri-docker.service ]]; then
    sudo tee /etc/systemd/system/cri-docker.service > /dev/null <<'EOF'
[Unit]
Description=CRI Interface for Docker Application Container Engine
Documentation=https://docs.mirantis.com
After=network-online.target firewalld.service docker.service
Wants=network-online.target
Requires=cri-docker.socket

[Service]
Type=notify
ExecStart=/usr/local/bin/cri-dockerd --container-runtime-endpoint fd://
ExecReload=/bin/kill -s HUP $MAINPID
TimeoutSec=0
RestartSec=2
Restart=always
StartLimitBurst=3
StartLimitInterval=60s
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
Delegate=yes
KillMode=process

[Install]
WantedBy=multi-user.target
EOF
fi

# Create socket file for cri-dockerd (idempotent)
if [[ ! -f /etc/systemd/system/cri-docker.socket ]]; then
    sudo tee /etc/systemd/system/cri-docker.socket > /dev/null <<EOF
[Unit]
Description=CRI Docker Socket for the API
PartOf=cri-docker.service

[Socket]
ListenStream=/run/cri-dockerd.sock
SocketMode=0660
SocketUser=root
SocketGroup=docker

[Install]
WantedBy=sockets.target
EOF
fi

# Enable and start cri-dockerd
sudo systemctl daemon-reload
sudo systemctl enable cri-docker.service
sudo systemctl enable cri-docker.socket
sudo systemctl start cri-docker.service

holodeck_progress "$COMPONENT" 6 6 "Verifying installation"

# Docker should already be ready from earlier wait, just verify
if ! holodeck_verify_docker; then
    holodeck_error 5 "$COMPONENT" \
        "Docker installation verification failed" \
        "Run 'systemctl status docker' to diagnose"
fi

FINAL_VERSION=$(sudo docker version --format '{{"{{"}}{{".Server.Version"}}{{"}}"}}')
holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed Docker ${FINAL_VERSION}"
`

type Docker struct {
	Version string
}

func NewDocker(env v1alpha1.Environment) *Docker {
	var version string

	if env.Spec.ContainerRuntime.Version != "" {
		version = env.Spec.ContainerRuntime.Version
	} else {
		version = "latest"
	}
	return &Docker{
		Version: version,
	}
}

func (t *Docker) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	dockerTemplate := template.Must(template.New("docker").Parse(dockerTemplate))
	if err := dockerTemplate.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute docker template: %v", err)
	}

	return nil
}
