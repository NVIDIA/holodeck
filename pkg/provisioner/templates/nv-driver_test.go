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

func TestNewNvDriver(t *testing.T) {
	testCases := []struct {
		description    string
		env            v1alpha1.Environment
		expectedBranch string
		expectedVer    string
	}{
		{
			description:    "empty spec defaults to default branch",
			env:            v1alpha1.Environment{},
			expectedBranch: defaultNVBranch,
			expectedVer:    "",
		},
		{
			description: "custom branch is used",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					NVIDIADriver: v1alpha1.NVIDIADriver{
						Branch: "550",
					},
				},
			},
			expectedBranch: "550",
			expectedVer:    "",
		},
		{
			description: "custom version is used",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					NVIDIADriver: v1alpha1.NVIDIADriver{
						Version: "535.129.03",
					},
				},
			},
			expectedBranch: "",
			expectedVer:    "535.129.03",
		},
		{
			description: "version takes precedence over default branch",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					NVIDIADriver: v1alpha1.NVIDIADriver{
						Version: "535.129.03",
						Branch:  "",
					},
				},
			},
			expectedBranch: "",
			expectedVer:    "535.129.03",
		},
		{
			description: "both version and branch are preserved",
			env: v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					NVIDIADriver: v1alpha1.NVIDIADriver{
						Version: "535.129.03",
						Branch:  "535",
					},
				},
			},
			expectedBranch: "535",
			expectedVer:    "535.129.03",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			driver := NewNvDriver(tc.env)
			require.Equal(t, tc.expectedBranch, driver.Branch)
			require.Equal(t, tc.expectedVer, driver.Version)
		})
	}
}

func TestNVDriverTemplate(t *testing.T) {
	testCases := []struct {
		description       string
		driver            *NvDriver
		expecteError      error
		expectedStrings   []string
		unexpectedStrings []string
	}{
		{
			description: "Version is set",
			driver: &NvDriver{
				Version: "123.4.5",
			},
			expectedStrings: []string{
				`COMPONENT="nvidia-driver"`,
				`DESIRED_VERSION="123.4.5"`,
				"holodeck_progress",
				"holodeck_verify_driver",
				"holodeck_mark_installed",
				`DRIVER_PACKAGE="${DRIVER_PACKAGE}=${DESIRED_VERSION}"`,
				"sudo modprobe nvidia",
				"nvidia-persistenced",
			},
		},
		{
			description: "Branch is set",
			driver: &NvDriver{
				Branch: "550",
			},
			expectedStrings: []string{
				`COMPONENT="nvidia-driver"`,
				`DESIRED_BRANCH="550"`,
				"holodeck_progress",
				"holodeck_verify_driver",
				"holodeck_mark_installed",
				`DRIVER_PACKAGE="${DRIVER_PACKAGE}-${DESIRED_BRANCH}"`,
			},
		},
		{
			description: "Version is preferred over branch",
			driver: &NvDriver{
				Branch:  "550",
				Version: "123.4.5",
			},
			expectedStrings: []string{
				`COMPONENT="nvidia-driver"`,
				`DESIRED_VERSION="123.4.5"`,
				`DESIRED_BRANCH="550"`,
				"holodeck_progress",
				// Version check comes first
				`if [[ -n "$DESIRED_VERSION" ]]`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			var output bytes.Buffer

			err := tc.driver.Execute(&output, v1alpha1.Environment{})
			require.EqualValues(t, tc.expecteError, err)

			outStr := output.String()

			// Check expected strings
			for _, expected := range tc.expectedStrings {
				require.Contains(t, outStr, expected,
					"Expected output to contain: %s", expected)
			}

			// Check unexpected strings
			for _, unexpected := range tc.unexpectedStrings {
				require.NotContains(t, outStr, unexpected,
					"Expected output NOT to contain: %s", unexpected)
			}
		})
	}
}

func TestNVDriverTemplate_CUDARepoArch(t *testing.T) {
	driver := &NvDriver{
		Branch: defaultNVBranch,
	}

	var output bytes.Buffer
	err := driver.Execute(&output, v1alpha1.Environment{})
	require.NoError(t, err)

	outStr := output.String()

	// Must NOT contain hardcoded x86_64 in the CUDA repo URL
	require.NotContains(t, outStr, "cuda/repos/$distribution/x86_64/",
		"Template must not hardcode x86_64 in the CUDA repository URL")

	// Must contain runtime architecture detection
	require.Contains(t, outStr, `CUDA_ARCH="$(uname -m)"`,
		"Template must detect architecture at runtime via uname -m")

	// Must contain aarch64 -> sbsa mapping
	require.Contains(t, outStr, `if [[ "$CUDA_ARCH" == "aarch64" ]]; then`,
		"Template must check for aarch64 architecture")
	require.Contains(t, outStr, `CUDA_ARCH="sbsa"`,
		"Template must map aarch64 to sbsa for NVIDIA CUDA repos")

	// Must use CUDA_ARCH variable in the wget URL
	require.Contains(t, outStr, "${CUDA_ARCH}/cuda-keyring",
		"Template must use CUDA_ARCH variable in the wget URL")
}
