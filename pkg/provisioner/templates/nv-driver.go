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

// From https://docs.nvidia.com/datacenter/tesla/tesla-installation-notes/index.html#ubuntu-lts
const NvDriverTemplate = `
# Install Dependencies
with_retry 3 10s sudo apt-get update
install_packages_with_retry linux-headers-$(uname -r)
install_packages_with_retry apt-utils build-essential ca-certificates curl kmod file libelf-dev libglvnd-dev pkg-config make

install_packages_with_retry gcc-12 g++-12 && \
    update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-12 12 && \
    update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-12 12

# Install the new cuda-keyring package	
distribution=$(. /etc/os-release;echo $ID$VERSION_ID | sed -e 's/\.//g')
wget https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb
with_retry 3 10s sudo apt-get update

# Install the NVIDIA driver
install_packages_with_retry cuda-drivers{{if .Version}}={{.Version}}{{else if .Branch}}-{{.Branch}}{{end}}

# Check if NVIDIA module is loaded, if not load it
if ! lsmod | grep -q "^nvidia "; then
    sudo modprobe nvidia
fi

# Start nvidia-persistenced
sudo nvidia-persistenced --persistence-mode

# Quick check to see if the driver is installed
nvidia-smi
`

type NvDriver v1alpha1.NVIDIADriver

func NewNvDriver(env v1alpha1.Environment) *NvDriver {
	var nvDriver NvDriver

	// Propagate user-supplied settings
	nvDriver.Install = env.Spec.NVIDIADriver.Install
	nvDriver.Branch = env.Spec.NVIDIADriver.Branch
	nvDriver.Version = env.Spec.NVIDIADriver.Version

	// Apply default branch only when neither a specific version nor a branch was provided
	if nvDriver.Version == "" && nvDriver.Branch == "" {
		nvDriver.Branch = defaultNVBranch
	}

	return &nvDriver
}

func (t *NvDriver) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	nvDriverTemplate := template.Must(template.New("nv-driver").Parse(NvDriverTemplate))
	err := nvDriverTemplate.Execute(tpl, t)
	if err != nil {
		return fmt.Errorf("failed to execute nv-driver template: %v", err)
	}
	return nil
}
