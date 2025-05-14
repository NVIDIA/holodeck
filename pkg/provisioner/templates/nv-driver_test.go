/**
# SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package templates

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNVDriverTemplate(t *testing.T) {
	testCases := []struct {
		description    string
		driver         *NvDriver
		expecteError   error
		expectedOutput string
	}{
		{
			description: "Version is set",
			driver: &NvDriver{
				Version: "123.4.5",
			},
			expectedOutput: `

sudo apt-get update
install_packages_with_retry linux-headers-$(uname -r)
distribution=$(. /etc/os-release;echo $ID$VERSION_ID | sed -e 's/\.//g')
wget https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb

with_retry 3 10s sudo apt-get update
install_packages_with_retry cuda-drivers=123.4.5

nvidia-smi
`,
		},
		{
			description: "Branch is set",
			driver: &NvDriver{
				Branch: "550",
			},
			expectedOutput: `

sudo apt-get update
install_packages_with_retry linux-headers-$(uname -r)
distribution=$(. /etc/os-release;echo $ID$VERSION_ID | sed -e 's/\.//g')
wget https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb

with_retry 3 10s sudo apt-get update
install_packages_with_retry cuda-drivers-550

nvidia-smi
`,
		},
		{
			description: "Version is preferred",
			driver: &NvDriver{
				Branch:  "550",
				Version: "123.4.5",
			},
			expectedOutput: `

sudo apt-get update
install_packages_with_retry linux-headers-$(uname -r)
distribution=$(. /etc/os-release;echo $ID$VERSION_ID | sed -e 's/\.//g')
wget https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb

with_retry 3 10s sudo apt-get update
install_packages_with_retry cuda-drivers=123.4.5

nvidia-smi
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {

			var output bytes.Buffer

			err := tc.driver.Execute(&output, v1alpha1.Environment{})
			require.EqualValues(t, tc.expecteError, err)

			require.EqualValues(t, tc.expectedOutput, output.String())
		})

	}
}
