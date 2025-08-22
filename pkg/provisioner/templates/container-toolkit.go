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

const containerToolkitTemplate = `{{if eq .Source "package"}}
# Install container toolkit from package
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg \
  && curl -s -L https://nvidia.github.io/libnvidia-container/{{.PackageChannel}}/deb/nvidia-container-toolkit.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list \
  && \
	with_retry 3 10s sudo apt-get update

{{if .PackageVersion}}install_packages_with_retry nvidia-container-toolkit={{.PackageVersion}} nvidia-container-toolkit-base \
						libnvidia-container-tools libnvidia-container1{{else}}install_packages_with_retry nvidia-container-toolkit nvidia-container-toolkit-base \
						libnvidia-container-tools libnvidia-container1{{end}}
{{else if eq .Source "git"}}
# Install container toolkit from git
echo "Installing NVIDIA Container Toolkit from git source..."
sudo apt-get update && sudo apt-get install -y git build-essential golang-1.21

# Set up Go path and version
export PATH=/usr/lib/go-1.21/bin:$PATH
export GOPATH=/tmp/go
export GOROOT=/usr/lib/go-1.21
mkdir -p $GOPATH

# Clone the repository
cd /tmp
git clone {{.GitRepo}} nvidia-container-toolkit
cd nvidia-container-toolkit
git checkout {{.GitRef}}

# Set up build environment
{{range $key, $value := .GitExtraEnv}}export {{$key}}="{{$value}}"
{{end}}

# Build the toolkit
{{if .GitMakeTargets}}make {{range $i, $target := .GitMakeTargets}}{{if $i}} {{end}}{{$target}}{{end}}{{else}}make all{{end}}

# Install binaries
sudo cp bin/* /usr/bin/
sudo chmod +x /usr/bin/nvidia-ctk /usr/bin/nvidia-container-runtime /usr/bin/nvidia-container-runtime-hook

# Create provenance file
sudo mkdir -p /etc/nvidia-container-toolkit
echo '{"source": "git", "repo": "{{.GitRepo}}", "ref": "{{.GitRef}}", "commit": "'$(git rev-parse HEAD)'", "timestamp": "'$(date -Iseconds)'"}' | sudo tee /etc/nvidia-container-toolkit/PROVENANCE.json
{{else if eq .Source "latest"}}
# Install container toolkit from latest commit
echo "Installing NVIDIA Container Toolkit from latest {{.LatestTrack}} branch..."
sudo apt-get update && sudo apt-get install -y git build-essential golang-1.21

# Set up Go path and version
export PATH=/usr/lib/go-1.21/bin:$PATH
export GOPATH=/tmp/go
export GOROOT=/usr/lib/go-1.21
mkdir -p $GOPATH

# Clone the repository and get latest commit
cd /tmp
git clone {{.LatestRepo}} nvidia-container-toolkit
cd nvidia-container-toolkit
git checkout {{.LatestTrack}}
LATEST_COMMIT=$(git rev-parse HEAD)

# Build the toolkit
make all

# Install binaries
sudo cp bin/* /usr/bin/
sudo chmod +x /usr/bin/nvidia-ctk /usr/bin/nvidia-container-runtime /usr/bin/nvidia-container-runtime-hook

# Create provenance file
sudo mkdir -p /etc/nvidia-container-toolkit
echo '{"source": "latest", "repo": "{{.LatestRepo}}", "track": "{{.LatestTrack}}", "commit": "'$LATEST_COMMIT'", "timestamp": "'$(date -Iseconds)'"}' | sudo tee /etc/nvidia-container-toolkit/PROVENANCE.json
{{end}}

# Configure container runtime
sudo nvidia-ctk runtime configure --runtime={{.ContainerRuntime}} --set-as-default --enable-cdi={{.EnableCDI}}

# Verify CNI configuration is preserved after nvidia-ctk
echo "Verifying CNI configuration after nvidia-ctk..."
if [ "{{.ContainerRuntime}}" = "containerd" ]; then
    if ! sudo grep -q 'bin_dir = "/opt/cni/bin"' /etc/containerd/config.toml; then
        echo "WARNING: CNI bin_dir configuration may have been modified by nvidia-ctk"
        echo "Restoring CNI paths..."
        # This is a safeguard, but nvidia-ctk should preserve existing CNI config
        sudo sed -i '/\[plugins."io.containerd.grpc.v1.cri".cni\]/,/\[/{s|bin_dir = .*|bin_dir = "/opt/cni/bin"|g}' /etc/containerd/config.toml
    fi
fi

sudo systemctl restart {{.ContainerRuntime}}
`

type ContainerToolkit struct {
	ContainerRuntime string
	EnableCDI        bool
	Source           string
	// Package source fields
	PackageChannel string
	PackageVersion string
	// Git source fields
	GitRepo        string
	GitRef         string
	GitMakeTargets []string
	GitExtraEnv    map[string]string
	// Latest source fields
	LatestRepo  string
	LatestTrack string
}

func NewContainerToolkit(env v1alpha1.Environment) *ContainerToolkit {
	runtime := string(env.Spec.ContainerRuntime.Name)
	if runtime == "" {
		runtime = "containerd"
	}

	ctk := &ContainerToolkit{
		ContainerRuntime: runtime,
		EnableCDI:        env.Spec.NVIDIAContainerToolkit.EnableCDI,
		Source:           "package", // default to package source
		PackageChannel:   "stable",  // default channel
	}

	// Determine source based on configuration
	nct := env.Spec.NVIDIAContainerToolkit
	if nct.Source != "" {
		ctk.Source = string(nct.Source)
	}

	// Handle backward compatibility with legacy Version field
	if nct.Version != "" && nct.Source == "" {
		ctk.Source = "package"
		ctk.PackageVersion = nct.Version
	}

	// Configure based on source type
	switch ctk.Source {
	case "package":
		if nct.Package != nil {
			if nct.Package.Channel != "" {
				ctk.PackageChannel = nct.Package.Channel
			}
			if nct.Package.Version != "" {
				ctk.PackageVersion = nct.Package.Version
			}
		}
	case "git":
		if nct.Git != nil {
			ctk.GitRepo = nct.Git.Repo
			if ctk.GitRepo == "" {
				ctk.GitRepo = "https://github.com/NVIDIA/nvidia-container-toolkit.git"
			}
			ctk.GitRef = nct.Git.Ref
			if nct.Git.Build != nil {
				ctk.GitMakeTargets = nct.Git.Build.MakeTargets
				ctk.GitExtraEnv = nct.Git.Build.ExtraEnv
			}
		}
	case "latest":
		if nct.Latest != nil {
			ctk.LatestRepo = nct.Latest.Repo
			if ctk.LatestRepo == "" {
				ctk.LatestRepo = "https://github.com/NVIDIA/nvidia-container-toolkit.git"
			}
			ctk.LatestTrack = nct.Latest.Track
		}
	}

	return ctk
}

func (t *ContainerToolkit) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	containerTlktTemplate := template.Must(template.New("container-toolkit").Parse(containerToolkitTemplate))
	if err := containerTlktTemplate.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute container-toolkit template: %v", err)
	}

	return nil
}
