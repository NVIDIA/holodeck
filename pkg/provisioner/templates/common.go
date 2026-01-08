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

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

const CommonFunctions = `

export DEBIAN_FRONTEND=noninteractive
export HOLODECK_ENVIRONMENT=true

# === CLOUD-INIT SYNCHRONIZATION ===
# Wait for cloud-init to complete before any provisioning (PR #552)
# This prevents race conditions with apt, systemd, and network configuration
if command -v cloud-init &>/dev/null; then
    echo "[holodeck] Waiting for cloud-init to complete..."
    /usr/bin/cloud-init status --wait || true
    echo "[holodeck] cloud-init completed, proceeding with provisioning"
fi

echo "APT::Get::AllowUnauthenticated 1;" | sudo tee /etc/apt/apt.conf.d/99allow-unauthenticated

# === HOLODECK IDEMPOTENCY FRAMEWORK ===

export HOLODECK_STATE_DIR="/var/lib/holodeck/state"
export HOLODECK_LOG_FORMAT="${HOLODECK_LOG_FORMAT:-text}"

# Initialize state directory
sudo mkdir -p "${HOLODECK_STATE_DIR}"

# Exit codes:
# 0  = Success
# 1  = General error
# 2  = Invalid input/configuration
# 3  = Network error (retryable)
# 4  = Dependency error
# 5  = Verification failed
# 10 = Driver error
# 11 = Runtime error
# 12 = Toolkit error
# 13 = Kubernetes error

# Check if a component is installed with the expected version
holodeck_is_installed() {
    local component="$1"
    local version="$2"
    local state_file="${HOLODECK_STATE_DIR}/${component}.state"

    if [[ ! -f "$state_file" ]]; then
        return 1
    fi

    if [[ -n "$version" ]]; then
        grep -q "^version=${version}$" "$state_file" || return 1
    fi

    grep -q "^status=installed$" "$state_file" || return 1
    return 0
}

# Mark a component as installed
holodeck_mark_installed() {
    local component="$1"
    local version="$2"
    local state_file="${HOLODECK_STATE_DIR}/${component}.state"
    local installed_at
    installed_at=$(date -Iseconds)

    printf 'status=installed\nversion=%s\ninstalled_at=%s\n' \
        "$version" "$installed_at" | sudo tee "$state_file" > /dev/null
}

# Log with structure
holodeck_log() {
    local level="$1"
    local component="$2"
    local message="$3"
    local timestamp
    timestamp=$(date -Iseconds)

    if [[ "$HOLODECK_LOG_FORMAT" == "json" ]]; then
        # Escape special characters for JSON
        local escaped_message
        escaped_message=$(printf '%s' "$message" | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g')
        printf '{"timestamp":"%s","level":"%s","component":"%s","message":"%s"}\n' \
            "$timestamp" "$level" "$component" "$escaped_message"
    else
        printf "[%s] [%-5s] [%s] %s\n" "$timestamp" "$level" "$component" "$message"
    fi
}

# Report error with context and exit
holodeck_error() {
    local code="$1"
    local component="$2"
    local message="$3"
    local remediation="${4:-}"

    holodeck_log "ERROR" "$component" "$message"
    if [[ -n "$remediation" ]]; then
        holodeck_log "INFO" "$component" "Remediation: $remediation"
    fi
    exit "$code"
}

# Progress reporting for multi-step operations
holodeck_progress() {
    local component="$1"
    local current="$2"
    local total="$3"
    local message="$4"

    holodeck_log "INFO" "$component" "[${current}/${total}] ${message}"
}

# Smart retry with exponential backoff for network operations
holodeck_retry() {
    local max_attempts="$1"
    local component="$2"
    shift 2
    local cmd=("$@")

    local attempt=1
    local delay=5
    local rc

    while true; do
        set +e
        "${cmd[@]}"
        rc=$?
        set -e

        if [[ $rc -eq 0 ]]; then
            return 0
        fi

        if [[ $attempt -ge $max_attempts ]]; then
            holodeck_log "ERROR" "$component" "Failed after ${max_attempts} attempts"
            return $rc
        fi

        holodeck_log "WARN" "$component" \
            "Attempt ${attempt}/${max_attempts} failed, retrying in ${delay}s"
        sleep "$delay"
        ((attempt++))
        if (( delay < 60 )); then
            delay=$((delay * 2))
            if (( delay > 60 )); then
                delay=60
            fi
        fi
    done
}

# Verify a command exists
holodeck_require_command() {
    local cmd="$1"
    local component="$2"

    if ! command -v "$cmd" &>/dev/null; then
        holodeck_error 4 "$component" "Required command not found: $cmd" \
            "Ensure $cmd is installed and in PATH"
    fi
}

# === VERIFICATION FUNCTIONS ===

holodeck_verify_driver() {
    nvidia-smi &>/dev/null || return 1
    return 0
}

holodeck_verify_containerd() {
    systemctl is-active --quiet containerd || return 1
    sudo ctr version &>/dev/null || return 1
    return 0
}

holodeck_verify_docker() {
    systemctl is-active --quiet docker || return 1
    # Use sudo because usermod -aG docker doesn't apply to current session
    sudo docker info &>/dev/null || return 1
    return 0
}

holodeck_verify_crio() {
    systemctl is-active --quiet crio || return 1
    return 0
}

holodeck_verify_toolkit() {
    command -v nvidia-ctk &>/dev/null || return 1
    nvidia-ctk --version &>/dev/null || return 1
    return 0
}

holodeck_verify_kubernetes() {
    local kubeconfig="${1:-/etc/kubernetes/admin.conf}"
    kubectl --kubeconfig="$kubeconfig" get nodes &>/dev/null || return 1
    return 0
}

# === LEGACY FUNCTIONS (preserved for compatibility) ===

install_packages_with_retry() {
	local max_retries=5 retry_delay=5
	local packages=("$@")
	
	for ((i=1; i<=max_retries; i++)); do
		echo "[$i/$max_retries] apt-get update"
		if sudo apt-get -o Acquire::Retries=3 update; then
			echo "[$i/$max_retries] installing: ${packages[*]}"
			if sudo DEBIAN_FRONTEND=noninteractive \
				   apt-get install -y --no-install-recommends "${packages[@]}"; then
				return 0            # success
			fi
		fi
		echo "Attempt $i failed; sleeping ${retry_delay}s" >&2
		sleep "$retry_delay"
	done
	echo "All ${max_retries} attempts failed" >&2
	return 1
}

with_retry() {
	local max_attempts="$1"
	local delay="$2"
	local count=0
	local rc
	shift 2

	while true; do
		set +e
		"$@"
		rc="$?"
		set -e

		count="$((count+1))"

		if [[ "${rc}" -eq 0 ]]; then
			return 0
		fi

		if [[ "${max_attempts}" -le 0 ]] || [[ "${count}" -lt "${max_attempts}" ]]; then
			sleep "${delay}"
		else
			break
		fi
	done

	return 1
}
`

// Template is the interface that wraps the Execute method.
type Template interface {
	Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error
}
