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
)

func (p *Provider) DryRun() error {
	// Check if the desired instance type is supported in the region
	p.log.Wg.Add(1)
	go p.log.Loading("Checking if instance type %s is supported in region %s", p.Spec.Instance.Type, p.Spec.Instance.Region)
	err := p.checkInstanceTypes()
	if err != nil {
		p.fail()
		return err
	}
	p.done()

	// Check if the desired image is supported in the region
	p.log.Wg.Add(1)
	go p.log.Loading("Checking image")
	err = p.checkImages()
	if err != nil {
		p.fail()
		return fmt.Errorf("failed to get images: %v", err)
	}
	p.done()

	return nil
}
