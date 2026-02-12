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

const dockerPackageTemplate = `
COMPONENT="docker"
SOURCE="package"
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
    holodeck_log "INFO" "$COMPONENT" "Installing latest Docker version"
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
        docker-ce docker-ce-cli containerd.io
else
    holodeck_log "INFO" "$COMPONENT" "Installing Docker version: ${DESIRED_VERSION}"
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

// dockerGitTemplate builds Docker (moby) from source.
const dockerGitTemplate = `
COMPONENT="docker"
SOURCE="git"
GIT_REPO="{{.GitRepo}}"
GIT_REF="{{.GitRef}}"
GIT_COMMIT="{{.GitCommit}}"

holodeck_progress "$COMPONENT" 1 6 "Checking existing installation"

if command -v dockerd &>/dev/null; then
    if [[ -f /etc/docker/PROVENANCE.json ]]; then
        if command -v jq &>/dev/null; then
            INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/docker/PROVENANCE.json)
            if [[ "$INSTALLED_COMMIT" == "$GIT_COMMIT" ]]; then
                if holodeck_verify_docker; then
                    holodeck_log "INFO" "$COMPONENT" "Already installed from commit: ${GIT_COMMIT}"
                    exit 0
                fi
            fi
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 6 "Installing build dependencies"

holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
    build-essential ca-certificates curl git libseccomp-dev pkg-config

GO_VERSION="${DOCKER_GO_VERSION:-1.23.4}"
GO_ARCH="$(uname -m)"
case "${GO_ARCH}" in
    x86_64|amd64)  GO_ARCH="amd64" ;;
    aarch64|arm64) GO_ARCH="arm64" ;;
    *) holodeck_log "ERROR" "$COMPONENT" "Unsupported arch: ${GO_ARCH}"; exit 1 ;;
esac
if ! command -v /usr/local/go/bin/go &>/dev/null; then
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" | \
        sudo tar -C /usr/local -xzf -
fi
export PATH="/usr/local/go/bin:$PATH"
export GOTOOLCHAIN=auto

holodeck_progress "$COMPONENT" 3 6 "Cloning moby repository"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

if ! git clone --depth 1 "${GIT_REPO}" "${WORK_DIR}/src"; then
    holodeck_log "ERROR" "$COMPONENT" "Failed to clone ${GIT_REPO}"
    exit 1
fi
cd "${WORK_DIR}/src" || exit 1
if ! git fetch --depth 1 origin "${GIT_REF}"; then
    holodeck_log "ERROR" "$COMPONENT" "Failed to fetch ref ${GIT_REF}"
    exit 1
fi
git checkout FETCH_HEAD

holodeck_progress "$COMPONENT" 4 6 "Building from source"

# Build dockerd and docker CLI
DOCKER_BUILDTAGS="seccomp" hack/make.sh binary || {
    holodeck_log "ERROR" "$COMPONENT" "Build failed"
    exit 1
}

holodeck_progress "$COMPONENT" 5 6 "Installing binaries"

# Install built binaries
sudo install -m 755 bundles/binary-daemon/dockerd /usr/local/bin/
sudo install -m 755 bundles/binary-daemon/docker-proxy /usr/local/bin/
# Install CLI if built
if [[ -f bundles/binary-client/docker ]]; then
    sudo install -m 755 bundles/binary-client/docker /usr/local/bin/
fi

# Install containerd.io and runc as dependencies
holodeck_retry 3 "$COMPONENT" install_packages_with_retry containerd.io

# Create Docker daemon config
sudo mkdir -p /etc/docker
if [[ ! -f /etc/docker/daemon.json ]]; then
    sudo tee /etc/docker/daemon.json > /dev/null <<EOF
{
  "exec-opts": ["native.cgroupdriver=systemd"],
  "log-driver": "json-file",
  "log-opts": { "max-size": "100m" },
  "storage-driver": "overlay2"
}
EOF
fi

# Create systemd service
if [[ ! -f /etc/systemd/system/docker.service ]]; then
    cat <<EOF | sudo tee /etc/systemd/system/docker.service
[Unit]
Description=Docker Application Container Engine
After=network-online.target containerd.service
[Service]
ExecStart=/usr/local/bin/dockerd
Type=notify
Restart=always
RestartSec=5
[Install]
WantedBy=multi-user.target
EOF
fi

sudo systemctl daemon-reload
sudo systemctl enable --now docker

# Install cri-dockerd for Kubernetes compatibility
CRI_DOCKERD_VERSION="0.3.17"
if [[ ! -f /usr/local/bin/cri-dockerd ]]; then
    CRI_DOCKERD_URL="https://github.com/Mirantis/cri-dockerd/releases/download/v${CRI_DOCKERD_VERSION}/cri-dockerd-${CRI_DOCKERD_VERSION}.${GO_ARCH}.tgz"
    curl -L "${CRI_DOCKERD_URL}" | sudo tar xzv -C /usr/local/bin --strip-components=1
fi

holodeck_progress "$COMPONENT" 6 6 "Verifying installation"

timeout=120
while ! sudo docker info &>/dev/null; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" "Timeout waiting for Docker" \
            "Check 'systemctl status docker'"
    fi
    sleep 1; ((timeout--))
done

if ! holodeck_verify_docker; then
    holodeck_error 5 "$COMPONENT" "Docker verification failed after git build" \
        "Check build logs and 'systemctl status docker'"
fi

FINAL_VERSION=$(sudo docker version --format '{{"{{"}}{{".Server.Version"}}{{"}}"}}' || echo "${GIT_COMMIT}")

sudo mkdir -p /etc/docker
printf '%s\n' '{
  "source": "git",
  "repo": "'"${GIT_REPO}"'",
  "ref": "'"${GIT_REF}"'",
  "commit": "'"${GIT_COMMIT}"'",
  "version": "'"${FINAL_VERSION}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/docker/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${FINAL_VERSION}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed Docker from git: ${GIT_COMMIT}"
`

// Docker holds configuration for Docker installation.
type Docker struct {
	// Source configuration
	Source string // "package", "git"

	// Package source fields
	Version string

	// Git source fields
	GitRepo   string
	GitRef    string
	GitCommit string
}

// NewDocker creates a Docker from an Environment spec.
func NewDocker(env v1alpha1.Environment) (*Docker, error) {
	cr := env.Spec.ContainerRuntime

	d := &Docker{
		Source: string(cr.Source),
	}

	if d.Source == "" {
		d.Source = "package"
	}

	switch d.Source {
	case "package":
		switch {
		case cr.Package != nil && cr.Package.Version != "":
			d.Version = cr.Package.Version
		case cr.Version != "":
			d.Version = cr.Version
		default:
			d.Version = "latest"
		}

	case "git":
		if cr.Git == nil {
			return nil, fmt.Errorf("git source requires 'git' configuration")
		}
		d.GitRepo = cr.Git.Repo
		d.GitRef = cr.Git.Ref
		if d.GitRepo == "" {
			d.GitRepo = "https://github.com/moby/moby.git"
		}
	}

	return d, nil
}

// SetResolvedCommit sets the resolved git commit SHA.
func (t *Docker) SetResolvedCommit(shortSHA string) {
	t.GitCommit = shortSHA
}

// Execute renders the appropriate template based on source.
func (t *Docker) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	var templateContent string

	switch t.Source {
	case "package", "":
		templateContent = dockerPackageTemplate
	case "git":
		templateContent = dockerGitTemplate
	default:
		return fmt.Errorf("unknown docker source: %s", t.Source)
	}

	tmpl := template.Must(template.New("docker").Parse(templateContent))
	if err := tmpl.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute docker template: %w", err)
	}
	return nil
}
