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

// containerToolkitPackageTemplate installs CTK from distribution packages.
const containerToolkitPackageTemplate = `
COMPONENT="nvidia-container-toolkit"
SOURCE="package"
CHANNEL="{{.Channel}}"
VERSION="{{.Version}}"
CONTAINER_RUNTIME="{{.ContainerRuntime}}"
ENABLE_CDI="{{.EnableCDI}}"

holodeck_progress "$COMPONENT" 1 4 "Checking existing installation"

# Check if NVIDIA Container Toolkit is already installed and functional
if command -v nvidia-ctk &>/dev/null; then
    INSTALLED_VERSION=$(nvidia-ctk --version 2>/dev/null | head -1 || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ -z "$VERSION" ]] || echo "$INSTALLED_VERSION" | grep -q "$VERSION"; then
            if holodeck_verify_toolkit; then
        holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"
                    holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                    exit 0
            fi
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 4 "Adding NVIDIA repository (${CHANNEL})"

# Add keyring
if [[ ! -f /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg ]]; then
    holodeck_retry 3 "$COMPONENT" curl -fsSL \
        https://nvidia.github.io/libnvidia-container/gpgkey | \
        sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
fi

# Add repository (respecting channel)
REPO_URL="https://nvidia.github.io/libnvidia-container/${CHANNEL}/deb/nvidia-container-toolkit.list"
if [[ ! -f /etc/apt/sources.list.d/nvidia-container-toolkit.list ]]; then
    curl -s -L "$REPO_URL" | \
        sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
        sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list > /dev/null
fi

holodeck_retry 3 "$COMPONENT" sudo apt-get update

holodeck_progress "$COMPONENT" 3 4 "Installing NVIDIA Container Toolkit"

if [[ -n "$VERSION" ]]; then
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
        "nvidia-container-toolkit=${VERSION}"
else
holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
        nvidia-container-toolkit
fi

holodeck_progress "$COMPONENT" 4 4 "Configuring runtime"
sudo nvidia-ctk runtime configure \
    --runtime="${CONTAINER_RUNTIME}" \
    --set-as-default \
    --enable-cdi="${ENABLE_CDI}"

# Verify CNI configuration is preserved after nvidia-ctk
if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    if ! sudo grep -q 'bin_dir = "/opt/cni/bin"' /etc/containerd/config.toml 2>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Restoring CNI paths"
        sudo sed -i '/\[plugins."io.containerd.grpc.v1.cri".cni\]/,/\[/{s|bin_dir = .*|bin_dir = "/opt/cni/bin"|g}' \
            /etc/containerd/config.toml
    fi
fi

sudo systemctl restart "${CONTAINER_RUNTIME}"

# Write provenance
FINAL_VERSION=$(nvidia-ctk --version 2>/dev/null | head -1 || echo "installed")
sudo mkdir -p /etc/nvidia-container-toolkit
printf '%s\n' '{
  "source": "package",
  "channel": "'"${CHANNEL}"'",
  "version": "'"${FINAL_VERSION}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/nvidia-container-toolkit/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed from package: ${FINAL_VERSION}"
`

// containerToolkitGitTemplate installs CTK from a git ref via GHCR or source.
const containerToolkitGitTemplate = `
COMPONENT="nvidia-container-toolkit"
SOURCE="git"
GIT_REPO="{{.GitRepo}}"
GIT_REF="{{.GitRef}}"
GIT_COMMIT="{{.GitCommit}}"
CONTAINER_RUNTIME="{{.ContainerRuntime}}"
ENABLE_CDI="{{.EnableCDI}}"

holodeck_progress "$COMPONENT" 1 5 "Checking existing installation"

# Check if already installed with this commit
if command -v nvidia-ctk &>/dev/null; then
    if [[ -f /etc/nvidia-container-toolkit/PROVENANCE.json ]]; then
        if command -v jq &>/dev/null; then
            INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/nvidia-container-toolkit/PROVENANCE.json)
            if [[ "$INSTALLED_COMMIT" == "$GIT_COMMIT" ]]; then
                if holodeck_verify_toolkit; then
                    holodeck_log "INFO" "$COMPONENT" "Already installed from commit: ${GIT_COMMIT}"
                    exit 0
                fi
            fi
        else
            holodeck_log "WARN" "$COMPONENT" "jq not found; skipping provenance check"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 5 "Checking GHCR for pre-built packages"

GHCR_IMAGE="ghcr.io/nvidia/container-toolkit:${GIT_COMMIT}"
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

# Try to pull from GHCR first
GHCR_AVAILABLE=false
if sudo docker pull "${GHCR_IMAGE}" 2>/dev/null || \
   sudo podman pull "${GHCR_IMAGE}" 2>/dev/null; then
    GHCR_AVAILABLE=true
    holodeck_log "INFO" "$COMPONENT" "Found pre-built image in GHCR"
fi

if [[ "$GHCR_AVAILABLE" == "true" ]]; then
    holodeck_progress "$COMPONENT" 3 5 "Extracting packages from GHCR"

    # Detect distro family
    if [[ -f /etc/debian_version ]]; then
        PKG_PATTERN="*.deb"
        INSTALL_CMD="sudo dpkg -i"
    else
        PKG_PATTERN="*.rpm"
        INSTALL_CMD="sudo rpm -Uvh"
    fi

    # Extract packages from image
    CONTAINER_ID=$(sudo docker create "${GHCR_IMAGE}" 2>/dev/null || \
                   sudo podman create "${GHCR_IMAGE}" 2>/dev/null)
    if ! sudo docker cp "${CONTAINER_ID}:/artifacts" "${WORK_DIR}/" 2>/dev/null && \
       ! sudo podman cp "${CONTAINER_ID}:/artifacts" "${WORK_DIR}/" 2>/dev/null; then
        holodeck_log "ERROR" "$COMPONENT" "Failed to copy artifacts from container ${CONTAINER_ID}"
        exit 1
    fi
    sudo docker rm "${CONTAINER_ID}" 2>/dev/null || \
    sudo podman rm "${CONTAINER_ID}" 2>/dev/null

    holodeck_progress "$COMPONENT" 4 5 "Installing extracted packages"

    # Install packages
    for pkg in "${WORK_DIR}"/artifacts/${PKG_PATTERN}; do
        [[ -e "$pkg" ]] || continue
        holodeck_log "INFO" "$COMPONENT" "Installing: $(basename "$pkg")"
        ${INSTALL_CMD} "$pkg"
    done

    GHCR_DIGEST=$(sudo docker inspect --format='{{ "{{" }}.RepoDigests{{ "}}" }}' "${GHCR_IMAGE}" 2>/dev/null | \
                  head -1 || echo "unknown")
else
    holodeck_log "WARN" "$COMPONENT" "No pre-built image in GHCR, building from source"

    holodeck_progress "$COMPONENT" 3 5 "Cloning repository"

    # Validate repository URL (security: only allow GitHub URLs)
    if [[ -z "${GIT_REPO}" ]]; then
        holodeck_log "ERROR" "$COMPONENT" "GIT_REPO is empty; refusing to clone"
        exit 1
    fi
    if [[ "${GIT_REPO}" != https://github.com/* && "${GIT_REPO}" != git@github.com:* ]]; then
        holodeck_log "ERROR" "$COMPONENT" "GIT_REPO must be a GitHub URL: ${GIT_REPO}"
        exit 1
    fi

    if ! git clone --depth 1 "${GIT_REPO}" "${WORK_DIR}/src"; then
        holodeck_log "ERROR" "$COMPONENT" "Failed to clone repository ${GIT_REPO}"
        exit 1
    fi
    cd "${WORK_DIR}/src" || {
        holodeck_log "ERROR" "$COMPONENT" "Failed to enter source directory"
        exit 1
    }
    if ! git fetch --depth 1 origin "${GIT_REF}"; then
        holodeck_log "ERROR" "$COMPONENT" "Failed to fetch git ref ${GIT_REF}"
        exit 1
    fi
    if ! git checkout FETCH_HEAD; then
        holodeck_log "ERROR" "$COMPONENT" "Failed to checkout FETCH_HEAD"
        exit 1
    fi

    holodeck_progress "$COMPONENT" 4 5 "Building from source"

    # Install build dependencies - need recent Go version
    install_packages_with_retry make curl

    # Allow override via env var; default to known-good version
    GO_VERSION="${NVIDIA_CTK_GO_VERSION:-1.23.4}"

    # Detect architecture for Go download
    GO_ARCH="$(uname -m)"
    case "${GO_ARCH}" in
        x86_64|amd64)  GO_ARCH="amd64" ;;
        aarch64|arm64) GO_ARCH="arm64" ;;
        ppc64le)       GO_ARCH="ppc64le" ;;
        s390x)         GO_ARCH="s390x" ;;
        *)
            holodeck_log "ERROR" "$COMPONENT" "Unsupported architecture: ${GO_ARCH}"
            exit 1
            ;;
    esac

    # Install Go from official tarball (apt version is too old)
    if ! command -v /usr/local/go/bin/go &>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Installing Go ${GO_VERSION} (${GO_ARCH})"
        curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" | \
            sudo tar -C /usr/local -xzf -
    fi
    export PATH="/usr/local/go/bin:$PATH"
    export GOTOOLCHAIN=auto

    # Build command binaries
    if ! make cmds; then
        holodeck_log "ERROR" "$COMPONENT" "Failed to build command binaries with 'make cmds'"
        exit 1
    fi

    # Install binaries
    sudo install -m 755 nvidia-ctk /usr/local/bin/
    sudo install -m 755 nvidia-cdi-hook /usr/local/bin/
    # Install additional binaries if they exist
    for bin in nvidia-container-runtime nvidia-container-runtime-hook; do
        if [[ -f "$bin" ]]; then
            sudo install -m 755 "$bin" /usr/local/bin/
        fi
    done

    GHCR_DIGEST="source-build"
fi

holodeck_progress "$COMPONENT" 5 5 "Configuring runtime"

sudo nvidia-ctk runtime configure \
    --runtime="${CONTAINER_RUNTIME}" \
    --set-as-default \
    --enable-cdi="${ENABLE_CDI}"

# Verify CNI configuration is preserved after nvidia-ctk
if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    if ! sudo grep -q 'bin_dir = "/opt/cni/bin"' /etc/containerd/config.toml 2>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Restoring CNI paths"
        sudo sed -i '/\[plugins."io.containerd.grpc.v1.cri".cni\]/,/\[/{s|bin_dir = .*|bin_dir = "/opt/cni/bin"|g}' \
            /etc/containerd/config.toml
    fi
fi

sudo systemctl restart "${CONTAINER_RUNTIME}"

# Verify
if ! holodeck_verify_toolkit; then
    holodeck_error 12 "$COMPONENT" \
        "NVIDIA Container Toolkit verification failed after git install" \
        "Check build logs and ${CONTAINER_RUNTIME} configuration"
fi

# Write provenance
FINAL_VERSION=$(nvidia-ctk --version 2>/dev/null | head -1 || echo "${GIT_COMMIT}")
sudo mkdir -p /etc/nvidia-container-toolkit
printf '%s\n' '{
  "source": "git",
  "repo": "'"${GIT_REPO}"'",
  "ref": "'"${GIT_REF}"'",
  "commit": "'"${GIT_COMMIT}"'",
  "ghcr_digest": "'"${GHCR_DIGEST}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/nvidia-container-toolkit/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${GIT_COMMIT}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed from git: ${GIT_COMMIT}"
`

// containerToolkitLatestTemplate tracks a branch at provision time.
const containerToolkitLatestTemplate = `
COMPONENT="nvidia-container-toolkit"
SOURCE="latest"
GIT_REPO="{{.GitRepo}}"
TRACK_BRANCH="{{.TrackBranch}}"
CONTAINER_RUNTIME="{{.ContainerRuntime}}"
ENABLE_CDI="{{.EnableCDI}}"

holodeck_progress "$COMPONENT" 1 5 "Resolving latest commit on ${TRACK_BRANCH}"

# Validate repository URL (security: only allow GitHub URLs)
if [[ -z "${GIT_REPO}" ]]; then
    holodeck_log "ERROR" "$COMPONENT" "GIT_REPO is empty; refusing to continue"
    exit 1
fi
if [[ "${GIT_REPO}" != https://github.com/* && "${GIT_REPO}" != git@github.com:* ]]; then
    holodeck_log "ERROR" "$COMPONENT" "GIT_REPO must be a GitHub URL: ${GIT_REPO}"
    exit 1
fi

# Resolve branch to latest commit
if ! LATEST_COMMIT=$(git ls-remote "${GIT_REPO}" "refs/heads/${TRACK_BRANCH}" | cut -f1); then
    holodeck_log "ERROR" "$COMPONENT" "Failed to resolve latest commit from ${GIT_REPO} branch ${TRACK_BRANCH}"
    exit 1
fi
if [[ -z "$LATEST_COMMIT" ]]; then
    holodeck_log "ERROR" "$COMPONENT" "No commit found for branch ${TRACK_BRANCH} in ${GIT_REPO}"
    exit 1
fi
SHORT_COMMIT="${LATEST_COMMIT:0:8}"

holodeck_log "INFO" "$COMPONENT" "Tracking ${TRACK_BRANCH} at ${SHORT_COMMIT}"

# Check if already at latest
if command -v nvidia-ctk &>/dev/null; then
    if [[ -f /etc/nvidia-container-toolkit/PROVENANCE.json ]]; then
        if command -v jq &>/dev/null; then
            INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/nvidia-container-toolkit/PROVENANCE.json)
            if [[ "$INSTALLED_COMMIT" == "$SHORT_COMMIT" ]]; then
                if holodeck_verify_toolkit; then
                    holodeck_log "INFO" "$COMPONENT" "Already at latest: ${SHORT_COMMIT}"
                    exit 0
                fi
            fi
        else
            holodeck_log "WARN" "$COMPONENT" "jq not found; skipping provenance check"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 5 "Checking GHCR for pre-built packages"

GHCR_IMAGE="ghcr.io/nvidia/container-toolkit:${SHORT_COMMIT}"
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

# Try to pull from GHCR first
GHCR_AVAILABLE=false
if sudo docker pull "${GHCR_IMAGE}" 2>/dev/null || \
   sudo podman pull "${GHCR_IMAGE}" 2>/dev/null; then
    GHCR_AVAILABLE=true
    holodeck_log "INFO" "$COMPONENT" "Found pre-built image in GHCR"
fi

if [[ "$GHCR_AVAILABLE" == "true" ]]; then
    holodeck_progress "$COMPONENT" 3 5 "Extracting packages from GHCR"

    if [[ -f /etc/debian_version ]]; then
        PKG_PATTERN="*.deb"
        INSTALL_CMD="sudo dpkg -i"
    else
        PKG_PATTERN="*.rpm"
        INSTALL_CMD="sudo rpm -Uvh"
    fi

    CONTAINER_ID=$(sudo docker create "${GHCR_IMAGE}" 2>/dev/null || \
                   sudo podman create "${GHCR_IMAGE}" 2>/dev/null)
    if ! sudo docker cp "${CONTAINER_ID}:/artifacts" "${WORK_DIR}/" 2>/dev/null && \
       ! sudo podman cp "${CONTAINER_ID}:/artifacts" "${WORK_DIR}/" 2>/dev/null; then
        holodeck_log "ERROR" "$COMPONENT" "Failed to copy artifacts from container ${CONTAINER_ID}"
        exit 1
    fi
    sudo docker rm "${CONTAINER_ID}" 2>/dev/null || \
    sudo podman rm "${CONTAINER_ID}" 2>/dev/null

    holodeck_progress "$COMPONENT" 4 5 "Installing extracted packages"

    for pkg in "${WORK_DIR}"/artifacts/${PKG_PATTERN}; do
        [[ -e "$pkg" ]] || continue
        holodeck_log "INFO" "$COMPONENT" "Installing: $(basename "$pkg")"
        ${INSTALL_CMD} "$pkg"
    done

    GHCR_DIGEST=$(sudo docker inspect --format='{{ "{{" }}.RepoDigests{{ "}}" }}' "${GHCR_IMAGE}" 2>/dev/null | \
                  head -1 || echo "unknown")
else
    holodeck_log "WARN" "$COMPONENT" "No pre-built image in GHCR, building from source"

    holodeck_progress "$COMPONENT" 3 5 "Cloning repository"

    if ! git clone --depth 1 --branch "${TRACK_BRANCH}" "${GIT_REPO}" "${WORK_DIR}/src"; then
        holodeck_log "ERROR" "$COMPONENT" "Failed to clone ${GIT_REPO} branch ${TRACK_BRANCH}"
        exit 1
    fi
    cd "${WORK_DIR}/src" || {
        holodeck_log "ERROR" "$COMPONENT" "Failed to enter source directory"
        exit 1
    }

    holodeck_progress "$COMPONENT" 4 5 "Building from source"

    # Install build dependencies - need recent Go version
    install_packages_with_retry make curl

    # Allow override via env var; default to known-good version
    GO_VERSION="${NVIDIA_CTK_GO_VERSION:-1.23.4}"

    # Detect architecture for Go download
    GO_ARCH="$(uname -m)"
    case "${GO_ARCH}" in
        x86_64|amd64)  GO_ARCH="amd64" ;;
        aarch64|arm64) GO_ARCH="arm64" ;;
        ppc64le)       GO_ARCH="ppc64le" ;;
        s390x)         GO_ARCH="s390x" ;;
        *)
            holodeck_log "ERROR" "$COMPONENT" "Unsupported architecture: ${GO_ARCH}"
            exit 1
            ;;
    esac

    # Install Go from official tarball (apt version is too old)
    if ! command -v /usr/local/go/bin/go &>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Installing Go ${GO_VERSION} (${GO_ARCH})"
        curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" | \
            sudo tar -C /usr/local -xzf -
    fi
    export PATH="/usr/local/go/bin:$PATH"
    export GOTOOLCHAIN=auto

    # Build command binaries
    if ! make cmds; then
        holodeck_log "ERROR" "$COMPONENT" "Failed to build command binaries with 'make cmds'"
        exit 1
    fi

    # Install binaries
    sudo install -m 755 nvidia-ctk /usr/local/bin/
    sudo install -m 755 nvidia-cdi-hook /usr/local/bin/
    # Install additional binaries if they exist
    for bin in nvidia-container-runtime nvidia-container-runtime-hook; do
        if [[ -f "$bin" ]]; then
            sudo install -m 755 "$bin" /usr/local/bin/
        fi
    done

    GHCR_DIGEST="source-build"
fi

holodeck_progress "$COMPONENT" 5 5 "Configuring runtime"

sudo nvidia-ctk runtime configure \
    --runtime="${CONTAINER_RUNTIME}" \
    --set-as-default \
    --enable-cdi="${ENABLE_CDI}"

if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    if ! sudo grep -q 'bin_dir = "/opt/cni/bin"' /etc/containerd/config.toml 2>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Restoring CNI paths"
        sudo sed -i '/\[plugins."io.containerd.grpc.v1.cri".cni\]/,/\[/{s|bin_dir = .*|bin_dir = "/opt/cni/bin"|g}' \
            /etc/containerd/config.toml
    fi
fi

sudo systemctl restart "${CONTAINER_RUNTIME}"

if ! holodeck_verify_toolkit; then
    holodeck_error 12 "$COMPONENT" \
        "NVIDIA Container Toolkit verification failed" \
        "Check build logs and ${CONTAINER_RUNTIME} configuration"
fi

FINAL_VERSION=$(nvidia-ctk --version 2>/dev/null | head -1 || echo "${SHORT_COMMIT}")
sudo mkdir -p /etc/nvidia-container-toolkit
printf '%s\n' '{
  "source": "latest",
  "repo": "'"${GIT_REPO}"'",
  "branch": "'"${TRACK_BRANCH}"'",
  "commit": "'"${SHORT_COMMIT}"'",
  "ghcr_digest": "'"${GHCR_DIGEST}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/nvidia-container-toolkit/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${SHORT_COMMIT}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed from ${TRACK_BRANCH}: ${SHORT_COMMIT}"
`

// ContainerToolkit holds configuration for NVIDIA Container Toolkit installation.
type ContainerToolkit struct {
	ContainerRuntime string
	EnableCDI        bool

	// Source configuration
	Source      string // "package", "git", "latest"
	Version     string // For package source
	Channel     string // stable/experimental
	GitRepo     string
	GitRef      string
	GitCommit   string // Resolved short SHA
	TrackBranch string // For latest source
}

// NewContainerToolkit creates a ContainerToolkit from an Environment spec.
func NewContainerToolkit(env v1alpha1.Environment) (*ContainerToolkit, error) {
	runtime := string(env.Spec.ContainerRuntime.Name)
	if runtime == "" {
		runtime = "containerd"
	}

	nct := env.Spec.NVIDIAContainerToolkit

	ctk := &ContainerToolkit{
		ContainerRuntime: runtime,
		EnableCDI:        nct.EnableCDI,
		Source:           string(nct.Source),
	}

	// Default to package source
	if ctk.Source == "" {
		ctk.Source = "package"
	}

	switch ctk.Source {
	case "package":
		if nct.Package != nil {
			ctk.Version = nct.Package.Version
			ctk.Channel = nct.Package.Channel
		} else if nct.Version != "" {
			// Legacy field support
			ctk.Version = nct.Version
		}
		if ctk.Channel == "" {
			ctk.Channel = "stable"
		}

	case "git":
		if nct.Git == nil {
			return nil, fmt.Errorf("git source requires 'git' configuration")
		}
		ctk.GitRepo = nct.Git.Repo
		ctk.GitRef = nct.Git.Ref
		if ctk.GitRepo == "" {
			ctk.GitRepo = "https://github.com/NVIDIA/nvidia-container-toolkit.git"
		}

	case "latest":
		ctk.TrackBranch = "main"
		ctk.GitRepo = "https://github.com/NVIDIA/nvidia-container-toolkit.git"
		if nct.Latest != nil {
			if nct.Latest.Track != "" {
				ctk.TrackBranch = nct.Latest.Track
			}
			if nct.Latest.Repo != "" {
				ctk.GitRepo = nct.Latest.Repo
			}
		}
	}

	return ctk, nil
}

// SetResolvedCommit sets the resolved git commit SHA.
func (t *ContainerToolkit) SetResolvedCommit(shortSHA string) {
	t.GitCommit = shortSHA
}

// Execute renders the appropriate template based on source.
func (t *ContainerToolkit) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	var templateContent string

	switch t.Source {
	case "package", "":
		templateContent = containerToolkitPackageTemplate
	case "git":
		templateContent = containerToolkitGitTemplate
	case "latest":
		templateContent = containerToolkitLatestTemplate
	default:
		return fmt.Errorf("unknown CTK source: %s", t.Source)
	}

	tmpl := template.Must(template.New("container-toolkit").Parse(templateContent))
	if err := tmpl.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute container-toolkit template: %v", err)
	}

	return nil
}
