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
