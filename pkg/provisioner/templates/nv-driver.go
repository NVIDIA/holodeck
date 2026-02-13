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

const defaultNVBranch = "575"

// nvDriverPackageTemplate installs the NVIDIA driver from CUDA repository packages.
// From https://docs.nvidia.com/datacenter/tesla/tesla-installation-notes/index.html#ubuntu-lts
const nvDriverPackageTemplate = `
COMPONENT="nvidia-driver"
SOURCE="package"
DESIRED_VERSION="{{.Version}}"
DESIRED_BRANCH="{{.Branch}}"

# Check for NVIDIA GPU hardware before attempting installation
# This allows mixed CPU/GPU clusters to work correctly
holodeck_log "INFO" "$COMPONENT" "Checking for NVIDIA GPU hardware..."
if ! lspci 2>/dev/null | grep -qi 'nvidia\|3d controller'; then
    holodeck_log "INFO" "$COMPONENT" "No NVIDIA GPU detected on this node, skipping driver installation"
    exit 0
fi
holodeck_log "INFO" "$COMPONENT" "NVIDIA GPU detected, proceeding with driver installation"

holodeck_progress "$COMPONENT" 1 5 "Checking existing installation"

# Check if driver is already installed and functional
if command -v nvidia-smi &>/dev/null; then
    INSTALLED_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -1 || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        # Check if version matches (if specified)
        if [[ -z "$DESIRED_VERSION" ]] || [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION" ]]; then
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

            # Verify driver is functional
            if holodeck_verify_driver; then
                holodeck_log "INFO" "$COMPONENT" "Driver verified functional"
                holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                exit 0
            else
                holodeck_log "WARN" "$COMPONENT" \
                    "Driver installed but not functional, attempting repair"
            fi
        else
            holodeck_log "INFO" "$COMPONENT" \
                "Version mismatch: installed=${INSTALLED_VERSION}, desired=${DESIRED_VERSION}"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 5 "Installing dependencies"

# Check kernel headers availability BEFORE attempting install
KERNEL_VERSION=$(uname -r)
holodeck_log "INFO" "$COMPONENT" "Checking kernel headers for ${KERNEL_VERSION}"

holodeck_retry 3 "$COMPONENT" sudo apt-get update

if ! apt-cache show "linux-headers-${KERNEL_VERSION}" >/dev/null 2>&1; then
    holodeck_log "WARN" "$COMPONENT" \
        "Kernel headers for ${KERNEL_VERSION} not found in repositories"

    # Try to find a compatible kernel header package
    KERNEL_BASE=$(echo "${KERNEL_VERSION}" | cut -d- -f1-2)
    holodeck_log "INFO" "$COMPONENT" \
        "Searching for compatible headers with base version ${KERNEL_BASE}"
    COMPATIBLE_HEADERS=$(apt-cache search linux-headers | \
        grep -E "linux-headers-${KERNEL_BASE}" | head -1 | awk '{print $1}')

    if [[ -n "$COMPATIBLE_HEADERS" ]]; then
        holodeck_log "WARN" "$COMPONENT" \
            "Using potentially compatible headers: $COMPATIBLE_HEADERS"
        holodeck_retry 3 "$COMPONENT" install_packages_with_retry "$COMPATIBLE_HEADERS"
    else
        holodeck_error 4 "$COMPONENT" \
            "No compatible kernel headers found for ${KERNEL_VERSION}" \
            "Update kernel or use a different AMI with available headers"
    fi
else
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry "linux-headers-${KERNEL_VERSION}"
fi

holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
    apt-utils build-essential ca-certificates curl kmod file \
    libelf-dev libglvnd-dev pkg-config make

holodeck_retry 3 "$COMPONENT" install_packages_with_retry gcc-12 g++-12
sudo update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-12 12
sudo update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-12 12

holodeck_progress "$COMPONENT" 3 5 "Adding CUDA repository"

# Add CUDA repository (idempotent - install if either list or keyring is missing)
if [[ ! -f /etc/apt/sources.list.d/cuda*.list ]] || \
   [[ ! -f /usr/share/keyrings/cuda-archive-keyring.gpg ]]; then
    distribution=$(. /etc/os-release; echo "${ID}${VERSION_ID}" | sed -e 's/\.//g')
    # Determine CUDA repo architecture (NVIDIA uses "sbsa" for arm64 servers)
    CUDA_ARCH="$(uname -m)"
    if [[ "$CUDA_ARCH" == "aarch64" ]]; then
        CUDA_ARCH="sbsa"
    fi
    holodeck_retry 3 "$COMPONENT" wget -q \
        "https://developer.download.nvidia.com/compute/cuda/repos/$distribution/${CUDA_ARCH}/cuda-keyring_1.1-1_all.deb"
    sudo dpkg -i cuda-keyring_1.1-1_all.deb
    rm -f cuda-keyring_1.1-1_all.deb
    holodeck_retry 3 "$COMPONENT" sudo apt-get update
else
    holodeck_log "INFO" "$COMPONENT" "CUDA repository already configured"
fi

holodeck_progress "$COMPONENT" 4 5 "Installing NVIDIA driver"

# Install driver
DRIVER_PACKAGE="cuda-drivers"
if [[ -n "$DESIRED_VERSION" ]]; then
    DRIVER_PACKAGE="${DRIVER_PACKAGE}=${DESIRED_VERSION}"
elif [[ -n "$DESIRED_BRANCH" ]]; then
    DRIVER_PACKAGE="${DRIVER_PACKAGE}-${DESIRED_BRANCH}"
fi

holodeck_log "INFO" "$COMPONENT" "Installing package: ${DRIVER_PACKAGE}"
holodeck_retry 3 "$COMPONENT" install_packages_with_retry "$DRIVER_PACKAGE"

holodeck_progress "$COMPONENT" 5 5 "Verifying installation"

# Load module if not loaded
if ! lsmod | grep -q "^nvidia "; then
    sudo modprobe nvidia || holodeck_error 10 "$COMPONENT" \
        "Failed to load nvidia kernel module" \
        "Check dmesg for kernel module errors: dmesg | grep -i nvidia"
fi

# Start persistenced
sudo nvidia-persistenced --persistence-mode || true

# Final verification
if ! holodeck_verify_driver; then
    holodeck_error 5 "$COMPONENT" \
        "Driver installation verification failed" \
        "Run 'dmesg | grep -i nvidia' and 'nvidia-smi' to diagnose"
fi

FINAL_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -1)

# Write provenance
sudo mkdir -p /etc/nvidia-driver
printf '%s\n' '{
  "source": "package",
  "branch": "'"${DESIRED_BRANCH}"'",
  "version": "'"${FINAL_VERSION}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/nvidia-driver/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed driver version ${FINAL_VERSION}"
`

// nvDriverRunfileTemplate installs the NVIDIA driver from a .run file.
const nvDriverRunfileTemplate = `
COMPONENT="nvidia-driver"
SOURCE="runfile"
RUNFILE_URL="{{.RunfileURL}}"
RUNFILE_CHECKSUM="{{.RunfileChecksum}}"

# Check for NVIDIA GPU hardware before attempting installation
holodeck_log "INFO" "$COMPONENT" "Checking for NVIDIA GPU hardware..."
if ! lspci 2>/dev/null | grep -qi 'nvidia\|3d controller'; then
    holodeck_log "INFO" "$COMPONENT" "No NVIDIA GPU detected on this node, skipping driver installation"
    exit 0
fi
holodeck_log "INFO" "$COMPONENT" "NVIDIA GPU detected, proceeding with runfile driver installation"

holodeck_progress "$COMPONENT" 1 5 "Checking existing installation"

# Check if driver is already installed and functional
if command -v nvidia-smi &>/dev/null; then
    INSTALLED_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -1 || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if holodeck_verify_driver; then
            # Check provenance to see if installed from same runfile
            if [[ -f /etc/nvidia-driver/PROVENANCE.json ]]; then
                if command -v jq &>/dev/null; then
                    INSTALLED_URL=$(jq -r '.url // empty' /etc/nvidia-driver/PROVENANCE.json)
                    if [[ "$INSTALLED_URL" == "$RUNFILE_URL" ]]; then
                        holodeck_log "INFO" "$COMPONENT" "Already installed from this runfile: ${INSTALLED_VERSION}"
                        exit 0
                    fi
                fi
            fi
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"
            holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
            exit 0
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 5 "Installing dependencies"

KERNEL_VERSION=$(uname -r)
holodeck_log "INFO" "$COMPONENT" "Checking kernel headers for ${KERNEL_VERSION}"

holodeck_retry 3 "$COMPONENT" sudo apt-get update

if ! apt-cache show "linux-headers-${KERNEL_VERSION}" >/dev/null 2>&1; then
    KERNEL_BASE=$(echo "${KERNEL_VERSION}" | cut -d- -f1-2)
    COMPATIBLE_HEADERS=$(apt-cache search linux-headers | \
        grep -E "linux-headers-${KERNEL_BASE}" | head -1 | awk '{print $1}')
    if [[ -n "$COMPATIBLE_HEADERS" ]]; then
        holodeck_retry 3 "$COMPONENT" install_packages_with_retry "$COMPATIBLE_HEADERS"
    else
        holodeck_error 4 "$COMPONENT" \
            "No compatible kernel headers found for ${KERNEL_VERSION}" \
            "Update kernel or use a different AMI with available headers"
    fi
else
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry "linux-headers-${KERNEL_VERSION}"
fi

holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
    build-essential ca-certificates curl kmod file \
    libelf-dev libglvnd-dev pkg-config make dkms

holodeck_progress "$COMPONENT" 3 5 "Downloading runfile"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

# Validate URL
if [[ -z "${RUNFILE_URL}" ]]; then
    holodeck_log "ERROR" "$COMPONENT" "RUNFILE_URL is empty"
    exit 1
fi

holodeck_retry 3 "$COMPONENT" wget -q -O "${WORK_DIR}/driver.run" "${RUNFILE_URL}"

# Verify checksum if provided
if [[ -n "${RUNFILE_CHECKSUM}" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Verifying checksum"
    EXPECTED_HASH="${RUNFILE_CHECKSUM#sha256:}"
    ACTUAL_HASH=$(sha256sum "${WORK_DIR}/driver.run" | cut -d' ' -f1)
    if [[ "${ACTUAL_HASH}" != "${EXPECTED_HASH}" ]]; then
        holodeck_error 6 "$COMPONENT" \
            "Checksum mismatch: expected=${EXPECTED_HASH}, actual=${ACTUAL_HASH}" \
            "Verify the download URL and checksum"
    fi
    holodeck_log "INFO" "$COMPONENT" "Checksum verified"
fi

holodeck_progress "$COMPONENT" 4 5 "Installing driver from runfile"

chmod +x "${WORK_DIR}/driver.run"
if ! sudo "${WORK_DIR}/driver.run" --silent --dkms; then
    holodeck_error 7 "$COMPONENT" \
        "Runfile installation failed" \
        "Check /var/log/nvidia-installer.log for details"
fi

holodeck_progress "$COMPONENT" 5 5 "Verifying installation"

# Load module if not loaded
if ! lsmod | grep -q "^nvidia "; then
    sudo modprobe nvidia || holodeck_error 10 "$COMPONENT" \
        "Failed to load nvidia kernel module" \
        "Check dmesg for kernel module errors: dmesg | grep -i nvidia"
fi

sudo nvidia-persistenced --persistence-mode || true

if ! holodeck_verify_driver; then
    holodeck_error 5 "$COMPONENT" \
        "Driver installation verification failed after runfile install" \
        "Run 'dmesg | grep -i nvidia' and 'nvidia-smi' to diagnose"
fi

FINAL_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -1)

# Write provenance
sudo mkdir -p /etc/nvidia-driver
printf '%s\n' '{
  "source": "runfile",
  "url": "'"${RUNFILE_URL}"'",
  "checksum": "'"${RUNFILE_CHECKSUM}"'",
  "version": "'"${FINAL_VERSION}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/nvidia-driver/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed driver ${FINAL_VERSION} from runfile"
`

// nvDriverGitTemplate builds and installs the NVIDIA driver from open-gpu-kernel-modules.
const nvDriverGitTemplate = `
COMPONENT="nvidia-driver"
SOURCE="git"
GIT_REPO="{{.GitRepo}}"
GIT_REF="{{.GitRef}}"
GIT_COMMIT="{{.GitCommit}}"

# Check for NVIDIA GPU hardware before attempting installation
holodeck_log "INFO" "$COMPONENT" "Checking for NVIDIA GPU hardware..."
if ! lspci 2>/dev/null | grep -qi 'nvidia\|3d controller'; then
    holodeck_log "INFO" "$COMPONENT" "No NVIDIA GPU detected on this node, skipping driver installation"
    exit 0
fi
holodeck_log "INFO" "$COMPONENT" "NVIDIA GPU detected, proceeding with git driver installation"

holodeck_progress "$COMPONENT" 1 6 "Checking existing installation"

# Check if already installed with this commit
if command -v nvidia-smi &>/dev/null; then
    if [[ -f /etc/nvidia-driver/PROVENANCE.json ]]; then
        if command -v jq &>/dev/null; then
            INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/nvidia-driver/PROVENANCE.json)
            if [[ "$INSTALLED_COMMIT" == "$GIT_COMMIT" ]]; then
                if holodeck_verify_driver; then
                    holodeck_log "INFO" "$COMPONENT" "Already installed from commit: ${GIT_COMMIT}"
                    exit 0
                fi
            fi
        else
            holodeck_log "WARN" "$COMPONENT" "jq not found; skipping provenance check"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 6 "Installing build dependencies"

KERNEL_VERSION=$(uname -r)
holodeck_log "INFO" "$COMPONENT" "Checking kernel headers for ${KERNEL_VERSION}"

holodeck_retry 3 "$COMPONENT" sudo apt-get update

if ! apt-cache show "linux-headers-${KERNEL_VERSION}" >/dev/null 2>&1; then
    KERNEL_BASE=$(echo "${KERNEL_VERSION}" | cut -d- -f1-2)
    COMPATIBLE_HEADERS=$(apt-cache search linux-headers | \
        grep -E "linux-headers-${KERNEL_BASE}" | head -1 | awk '{print $1}')
    if [[ -n "$COMPATIBLE_HEADERS" ]]; then
        holodeck_retry 3 "$COMPONENT" install_packages_with_retry "$COMPATIBLE_HEADERS"
    else
        holodeck_error 4 "$COMPONENT" \
            "No compatible kernel headers found for ${KERNEL_VERSION}" \
            "Update kernel or use a different AMI with available headers"
    fi
else
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry "linux-headers-${KERNEL_VERSION}"
fi

holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
    build-essential ca-certificates curl kmod file git \
    libelf-dev libglvnd-dev pkg-config make dkms

holodeck_retry 3 "$COMPONENT" install_packages_with_retry gcc-12 g++-12
sudo update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-12 12
sudo update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-12 12

holodeck_progress "$COMPONENT" 3 6 "Cloning repository"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

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

holodeck_progress "$COMPONENT" 4 6 "Building kernel modules"

holodeck_log "INFO" "$COMPONENT" "Building open kernel modules at ${GIT_COMMIT}"

if ! make modules -j$(nproc); then
    holodeck_error 8 "$COMPONENT" \
        "Failed to build kernel modules from source" \
        "Check build output and ensure kernel headers are compatible"
fi

holodeck_progress "$COMPONENT" 5 6 "Installing kernel modules"

if ! sudo make modules_install; then
    holodeck_error 9 "$COMPONENT" \
        "Failed to install kernel modules" \
        "Check dmesg and module installation logs"
fi

holodeck_progress "$COMPONENT" 6 6 "Verifying installation"

# Load module
sudo depmod -a
if ! lsmod | grep -q "^nvidia "; then
    sudo modprobe nvidia || holodeck_error 10 "$COMPONENT" \
        "Failed to load nvidia kernel module" \
        "Check dmesg for kernel module errors: dmesg | grep -i nvidia"
fi

sudo nvidia-persistenced --persistence-mode || true

if ! holodeck_verify_driver; then
    holodeck_error 5 "$COMPONENT" \
        "Driver installation verification failed after git build" \
        "Run 'dmesg | grep -i nvidia' and 'nvidia-smi' to diagnose"
fi

FINAL_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -1)

# Write provenance
sudo mkdir -p /etc/nvidia-driver
printf '%s\n' '{
  "source": "git",
  "repo": "'"${GIT_REPO}"'",
  "ref": "'"${GIT_REF}"'",
  "commit": "'"${GIT_COMMIT}"'",
  "version": "'"${FINAL_VERSION}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/nvidia-driver/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${FINAL_VERSION}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed driver ${FINAL_VERSION} from git: ${GIT_COMMIT}"
`

// NvDriver holds configuration for NVIDIA driver installation.
type NvDriver struct {
	// Source configuration
	Source string // "package", "runfile", "git"

	// Package source fields
	Branch  string
	Version string

	// Runfile source fields
	RunfileURL      string
	RunfileChecksum string

	// Git source fields
	GitRepo   string
	GitRef    string
	GitCommit string // Resolved short SHA
}

// NewNvDriver creates an NvDriver from an Environment spec.
func NewNvDriver(env v1alpha1.Environment) (*NvDriver, error) {
	d := env.Spec.NVIDIADriver

	nvd := &NvDriver{
		Source: string(d.Source),
	}

	// Default to package source
	if nvd.Source == "" {
		nvd.Source = "package"
	}

	switch nvd.Source {
	case "package":
		if d.Package != nil {
			nvd.Branch = d.Package.Branch
			nvd.Version = d.Package.Version
		} else {
			// Legacy field support
			nvd.Branch = d.Branch
			nvd.Version = d.Version
		}
		// Apply default branch only when neither a specific version nor a branch was provided
		if nvd.Version == "" && nvd.Branch == "" {
			nvd.Branch = defaultNVBranch
		}

	case "runfile":
		if d.Runfile == nil {
			return nil, fmt.Errorf("runfile source requires 'runfile' configuration")
		}
		nvd.RunfileURL = d.Runfile.URL
		nvd.RunfileChecksum = d.Runfile.Checksum

	case "git":
		if d.Git == nil {
			return nil, fmt.Errorf("git source requires 'git' configuration")
		}
		nvd.GitRepo = d.Git.Repo
		nvd.GitRef = d.Git.Ref
		if nvd.GitRepo == "" {
			nvd.GitRepo = "https://github.com/NVIDIA/open-gpu-kernel-modules.git"
		}
	}

	return nvd, nil
}

// SetResolvedCommit sets the resolved git commit SHA.
func (t *NvDriver) SetResolvedCommit(shortSHA string) {
	t.GitCommit = shortSHA
}

// Execute renders the appropriate template based on source.
func (t *NvDriver) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	var templateContent string

	switch t.Source {
	case "package", "":
		templateContent = nvDriverPackageTemplate
	case "runfile":
		templateContent = nvDriverRunfileTemplate
	case "git":
		templateContent = nvDriverGitTemplate
	default:
		return fmt.Errorf("unknown driver source: %s", t.Source)
	}

	tmpl := template.Must(template.New("nv-driver").Parse(templateContent))
	if err := tmpl.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute nv-driver template: %w", err)
	}

	return nil
}
