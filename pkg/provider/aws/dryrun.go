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

package aws

import (
	"fmt"

	"github.com/NVIDIA/holodeck/internal/logger"
)

func (p *Provider) DryRun() error {
	// Check if the desired instance type is supported in the region
	cancel := p.log.Loading("Checking if instance type %s is supported in region %s", p.Spec.Type, p.Spec.Region)
	err := p.checkInstanceTypes()
	if err != nil {
		cancel(logger.ErrLoadingFailed)
		return err
	}
	cancel(nil)

	// Check if the desired image is supported in the region
	cancel = p.log.Loading("Checking image")
	err = p.checkImages()
	if err != nil {
		cancel(logger.ErrLoadingFailed)
		return fmt.Errorf("failed to get images: %w", err)
	}
	cancel(nil)

	// Cross-validate architecture compatibility
	if p.Spec.Image.Architecture != "" {
		cancel = p.log.Loading("Validating architecture compatibility")
		archs, err := p.getInstanceTypeArch(p.Spec.Type)
		if err != nil {
			cancel(logger.ErrLoadingFailed)
			return fmt.Errorf("failed to check instance type architecture: %w", err)
		}
		archMatch := false
		for _, a := range archs {
			if a == p.Spec.Image.Architecture {
				archMatch = true
				break
			}
		}
		if !archMatch {
			cancel(logger.ErrLoadingFailed)
			return fmt.Errorf(
				"architecture mismatch: AMI architecture is %s but instance type %s supports %v",
				p.Spec.Image.Architecture, p.Spec.Type, archs,
			)
		}
		cancel(nil)
	}

	return nil
}
