/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package v1alpha1

import (
	"fmt"
)

// Validate validates the NVIDIAContainerToolkit configuration.
func (nct *NVIDIAContainerToolkit) Validate() error {
	if !nct.Install {
		return nil
	}

	source := nct.Source
	if source == "" {
		source = CTKSourcePackage
	}

	switch source {
	case CTKSourcePackage:
		// Package source is always valid
		if nct.Package != nil && nct.Package.Channel != "" {
			if nct.Package.Channel != "stable" && nct.Package.Channel != "experimental" {
				return fmt.Errorf(
					"invalid CTK package channel: %s (must be 'stable' or 'experimental')",
					nct.Package.Channel,
				)
			}
		}
		return nil

	case CTKSourceGit:
		if nct.Git == nil {
			return fmt.Errorf("CTK git source requires 'git' configuration")
		}
		if nct.Git.Ref == "" {
			return fmt.Errorf("CTK git source requires 'ref' to be specified")
		}
		return nil

	case CTKSourceLatest:
		// Latest source is valid with or without explicit config
		return nil

	default:
		return fmt.Errorf("unknown CTK source: %s", source)
	}
}
