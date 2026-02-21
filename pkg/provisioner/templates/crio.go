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

const crioPackageTemplate = `
COMPONENT="crio"
SOURCE="package"
DESIRED_VERSION="{{.Version}}"

holodeck_progress "$COMPONENT" 1 4 "Checking existing installation"

# Check if CRI-O is already installed and functional
if systemctl is-active --quiet crio 2>/dev/null; then
    INSTALLED_VERSION=$(crio --version 2>/dev/null | head -1 | awk '{print $3}' || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ -z "$DESIRED_VERSION" ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION" ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION."* ]]; then
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

            if holodeck_verify_crio; then
                holodeck_log "INFO" "$COMPONENT" "CRI-O verified functional"
                holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                exit 0
            else
                holodeck_log "WARN" "$COMPONENT" \
                    "CRI-O installed but not functional, attempting repair"
            fi
        else
            holodeck_log "INFO" "$COMPONENT" \
                "Version mismatch: installed=${INSTALLED_VERSION}, desired=${DESIRED_VERSION}"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 4 "Adding CRI-O repository"

CRIO_VERSION="${DESIRED_VERSION}"

# Default to latest stable CRI-O if no version specified
if [[ -z "$CRIO_VERSION" ]]; then
    CRIO_VERSION="v1.33"
    holodeck_log "INFO" "$COMPONENT" "No version specified, defaulting to ${CRIO_VERSION}"
fi

# Ensure version starts with 'v' and is in vX.Y format (strip patch if present)
CRIO_VERSION="${CRIO_VERSION#v}"
CRIO_VERSION="v$(echo "$CRIO_VERSION" | cut -d. -f1,2)"

# CRI-O migrated from pkgs.k8s.io to download.opensuse.org
# See: https://github.com/cri-o/packaging#readme
CRIO_REPO_URL="https://download.opensuse.org/repositories/isv:/cri-o:/stable:/${CRIO_VERSION}"

# Add CRI-O repo based on OS family
case "${HOLODECK_OS_FAMILY}" in
    debian)
        if [[ ! -f /etc/apt/keyrings/cri-o-apt-keyring.gpg ]]; then
            sudo mkdir -p /etc/apt/keyrings
            holodeck_retry 3 "$COMPONENT" curl -fsSL \
                "${CRIO_REPO_URL}/deb/Release.key" | \
                sudo gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg
        else
            holodeck_log "INFO" "$COMPONENT" "CRI-O GPG key already present"
        fi

        if [[ ! -f /etc/apt/sources.list.d/cri-o.list ]]; then
            echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] ${CRIO_REPO_URL}/deb/ /" | \
                sudo tee /etc/apt/sources.list.d/cri-o.list > /dev/null
        else
            holodeck_log "INFO" "$COMPONENT" "CRI-O repository already configured"
        fi
        ;;

    amazon|rhel)
        if [[ ! -f /etc/yum.repos.d/cri-o.repo ]]; then
            # Repo file name format: isv:cri-o:stable:vX.Y.repo
            # See: https://download.opensuse.org/repositories/isv:/cri-o:/stable:/vX.Y/rpm/
            CRIO_REPO_FILE="isv:cri-o:stable:${CRIO_VERSION}.repo"
            holodeck_retry 3 "$COMPONENT" sudo curl -fsSL -o /etc/yum.repos.d/cri-o.repo \
                "${CRIO_REPO_URL}/rpm/${CRIO_REPO_FILE}"
        else
            holodeck_log "INFO" "$COMPONENT" "CRI-O repository already configured"
        fi
        ;;

    *)
        holodeck_error 2 "$COMPONENT" \
            "Unsupported OS family: ${HOLODECK_OS_FAMILY}" \
            "Supported: debian, amazon, rhel"
        ;;
esac

holodeck_progress "$COMPONENT" 3 4 "Installing CRI-O"

holodeck_retry 3 "$COMPONENT" pkg_update

# The opensuse CRI-O package does not pull OCI runtime dependencies on RHEL-family.
# Install crun (OCI runtime) and containers-common (registry/storage config) explicitly.
case "${HOLODECK_OS_FAMILY}" in
    amazon|rhel)
        holodeck_retry 3 "$COMPONENT" pkg_install cri-o crun containers-common
        ;;
    *)
        holodeck_retry 3 "$COMPONENT" pkg_install cri-o
        ;;
esac

# Start and enable Service
sudo systemctl daemon-reload
sudo systemctl enable crio.service
sudo systemctl start crio.service

holodeck_progress "$COMPONENT" 4 4 "Verifying installation"

# Wait for CRI-O to be ready (120s for slow VMs)
timeout=120
while ! systemctl is-active --quiet crio; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" \
            "Timeout waiting for CRI-O to become ready" \
            "Check 'systemctl status crio' and 'journalctl -u crio'"
    fi
    if (( timeout % 15 == 0 )); then
        holodeck_log "INFO" "$COMPONENT" \
            "Waiting for CRI-O to become ready (${timeout}s remaining)"
    fi
    sleep 1
    ((timeout--))
done

if ! holodeck_verify_crio; then
    holodeck_error 5 "$COMPONENT" \
        "CRI-O installation verification failed" \
        "Run 'systemctl status crio' to diagnose"
fi

FINAL_VERSION=$(crio --version 2>/dev/null | head -1 | awk '{print $3}' || echo "$CRIO_VERSION")
holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed CRI-O ${FINAL_VERSION}"
`

// crioGitTemplate builds and installs CRI-O from source.
const crioGitTemplate = `
COMPONENT="crio"
SOURCE="git"
GIT_REPO="{{.GitRepo}}"
GIT_REF="{{.GitRef}}"
GIT_COMMIT="{{.GitCommit}}"

holodeck_progress "$COMPONENT" 1 5 "Checking existing installation"

if command -v crio &>/dev/null; then
    if [[ -f /etc/crio/PROVENANCE.json ]]; then
        if command -v jq &>/dev/null; then
            INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/crio/PROVENANCE.json)
            if [[ "$INSTALLED_COMMIT" == "$GIT_COMMIT" ]]; then
                if holodeck_verify_crio; then
                    holodeck_log "INFO" "$COMPONENT" "Already installed from commit: ${GIT_COMMIT}"
                    exit 0
                fi
            fi
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 5 "Installing build dependencies"

case "${HOLODECK_OS_FAMILY}" in
    debian)
        holodeck_retry 3 "$COMPONENT" pkg_update
        holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
            build-essential ca-certificates curl git libseccomp-dev libgpgme-dev \
            pkg-config libglib2.0-dev
        ;;

    amazon|rhel)
        holodeck_retry 3 "$COMPONENT" pkg_update
        holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
            ca-certificates curl git libseccomp-devel gpgme-devel \
            pkg-config glib2-devel gcc make
        ;;

    *)
        holodeck_error 2 "$COMPONENT" \
            "Unsupported OS family: ${HOLODECK_OS_FAMILY}" \
            "Supported: debian, amazon, rhel"
        ;;
esac

GO_VERSION="${CRIO_GO_VERSION:-1.23.4}"
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

holodeck_progress "$COMPONENT" 3 5 "Cloning repository"

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

holodeck_progress "$COMPONENT" 4 5 "Building and installing"

if ! make; then
    holodeck_log "ERROR" "$COMPONENT" "Build failed"
    exit 1
fi
sudo make install

# Install conmon if not present
if ! command -v conmon &>/dev/null; then
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry conmon
fi

# Install CNI plugins if not present
CNI_VERSION="v1.6.2"
if [[ ! -f /opt/cni/bin/bridge ]]; then
    ARCH="${GO_ARCH}"
    CNI_TAR="cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz"
    curl -fsSL -o "${CNI_TAR}" \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/${CNI_TAR}"
    sudo mkdir -p /opt/cni/bin
    sudo tar Cxzvf /opt/cni/bin "${CNI_TAR}"
fi

# Create default config
sudo mkdir -p /etc/crio
sudo crio config --default | sudo tee /etc/crio/crio.conf > /dev/null

sudo systemctl daemon-reload
sudo systemctl enable --now crio

holodeck_progress "$COMPONENT" 5 5 "Verifying installation"

timeout=120
while ! systemctl is-active --quiet crio; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" "Timeout waiting for CRI-O" \
            "Check 'systemctl status crio'"
    fi
    sleep 1; ((timeout--))
done

if ! holodeck_verify_crio; then
    holodeck_error 5 "$COMPONENT" "CRI-O verification failed after git build" \
        "Check build logs and 'systemctl status crio'"
fi

FINAL_VERSION=$(crio --version 2>/dev/null | head -1 | awk '{print $3}' || echo "${GIT_COMMIT}")

sudo mkdir -p /etc/crio
printf '%s\n' '{
  "source": "git",
  "repo": "'"${GIT_REPO}"'",
  "ref": "'"${GIT_REF}"'",
  "commit": "'"${GIT_COMMIT}"'",
  "version": "'"${FINAL_VERSION}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/crio/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${FINAL_VERSION}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed CRI-O from git: ${GIT_COMMIT}"
`

// CriO holds configuration for CRI-O installation.
type CriO struct {
	// Source configuration
	Source string // "package", "git"

	// Package source fields
	Version string

	// Git source fields
	GitRepo   string
	GitRef    string
	GitCommit string
}

// NewCriO creates a CriO from an Environment spec.
func NewCriO(env v1alpha1.Environment) (*CriO, error) {
	cr := env.Spec.ContainerRuntime

	c := &CriO{
		Source: string(cr.Source),
	}

	if c.Source == "" {
		c.Source = "package"
	}

	switch c.Source {
	case "package":
		if cr.Package != nil && cr.Package.Version != "" {
			c.Version = cr.Package.Version
		} else if cr.Version != "" {
			c.Version = cr.Version
		}

	case "git":
		if cr.Git == nil {
			return nil, fmt.Errorf("git source requires 'git' configuration")
		}
		c.GitRepo = cr.Git.Repo
		c.GitRef = cr.Git.Ref
		if c.GitRepo == "" {
			c.GitRepo = "https://github.com/cri-o/cri-o.git"
		}
	}

	return c, nil
}

// SetResolvedCommit sets the resolved git commit SHA.
func (t *CriO) SetResolvedCommit(shortSHA string) {
	t.GitCommit = shortSHA
}

// Execute renders the appropriate template based on source.
func (t *CriO) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	var templateContent string

	switch t.Source {
	case "package", "":
		templateContent = crioPackageTemplate
	case "git":
		templateContent = crioGitTemplate
	default:
		return fmt.Errorf("unknown crio source: %s", t.Source)
	}

	tmpl := template.Must(template.New("crio").Parse(templateContent))
	if err := tmpl.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute crio template: %w", err)
	}
	return nil
}
