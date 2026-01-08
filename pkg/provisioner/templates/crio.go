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

const criOTemplate = `
COMPONENT="crio"
DESIRED_VERSION="{{.Version}}"

holodeck_progress "$COMPONENT" 1 4 "Checking existing installation"

# Check if CRI-O is already installed and functional
if systemctl is-active --quiet crio 2>/dev/null; then
    INSTALLED_VERSION=$(crio --version 2>/dev/null | head -1 | awk '{print $3}' || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ -z "$DESIRED_VERSION" ]] || [[ "$INSTALLED_VERSION" == *"$DESIRED_VERSION"* ]]; then
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

# Add CRI-O repo (idempotent)
if [[ ! -f /etc/apt/keyrings/cri-o-apt-keyring.gpg ]]; then
    sudo mkdir -p /etc/apt/keyrings
    holodeck_retry 3 "$COMPONENT" curl -fsSL \
        "https://pkgs.k8s.io/addons:/cri-o:/stable:/${CRIO_VERSION}/deb/Release.key" | \
        sudo gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg
else
    holodeck_log "INFO" "$COMPONENT" "CRI-O GPG key already present"
fi

if [[ ! -f /etc/apt/sources.list.d/cri-o.list ]]; then
    echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/stable:/${CRIO_VERSION}/deb/ /" | \
        sudo tee /etc/apt/sources.list.d/cri-o.list > /dev/null
else
    holodeck_log "INFO" "$COMPONENT" "CRI-O repository already configured"
fi

holodeck_progress "$COMPONENT" 3 4 "Installing CRI-O"

holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" sudo apt-get install -y cri-o

# Start and enable Service
sudo systemctl daemon-reload
sudo systemctl enable crio.service
sudo systemctl start crio.service

holodeck_progress "$COMPONENT" 4 4 "Verifying installation"

# Wait for CRI-O to be ready
timeout=30
while ! systemctl is-active --quiet crio; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" \
            "Timeout waiting for CRI-O to become ready" \
            "Check 'systemctl status crio' and 'journalctl -u crio'"
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

type CriO struct {
	Version string
}

func NewCriO(env v1alpha1.Environment) *CriO {
	return &CriO{
		Version: env.Spec.ContainerRuntime.Version,
	}
}

func (t *CriO) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	criOTemplate := template.Must(template.New("crio").Parse(criOTemplate))
	if err := criOTemplate.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute crio template: %v", err)
	}

	return nil
}
