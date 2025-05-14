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

// From https://docs.nvidia.com/datacenter/tesla/tesla-installation-notes/index.html#ubuntu-lts
const NvDriverTemplate = `

sudo apt-get update
install_packages_with_retry linux-headers-$(uname -r)
distribution=$(. /etc/os-release;echo $ID$VERSION_ID | sed -e 's/\.//g')
wget https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb

with_retry 3 10s sudo apt-get update
install_packages_with_retry cuda-drivers{{if .Version}}={{.Version}}{{else if .Branch}}-{{.Branch}}{{end}}

nvidia-smi
`

type NvDriver v1alpha1.NVIDIADriver

func NewNvDriver(env v1alpha1.Environment) *NvDriver {
	return (*NvDriver)(&env.Spec.NVIDIADriver)
}

func (t *NvDriver) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	nvDriverTemplate := template.Must(template.New("nv-driver").Parse(NvDriverTemplate))
	err := nvDriverTemplate.Execute(tpl, t)
	if err != nil {
		return fmt.Errorf("failed to execute nv-driver template: %v", err)
	}
	return nil
}
