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
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (p *Provider) checkInstanceTypes() error {
	var nextToken *string

	for {
		// Use the DescribeInstanceTypes API to get a list of supported instance types in the current region
		resp, err := p.ec2.DescribeInstanceTypes(context.TODO(), &ec2.DescribeInstanceTypesInput{NextToken: nextToken})
		if err != nil {
			return err
		}

		for _, it := range resp.InstanceTypes {
			if it.InstanceType == types.InstanceType(p.Spec.Instance.Type) {
				return nil
			}
		}

		if resp.NextToken != nil {
			nextToken = resp.NextToken
		} else {
			break
		}
	}

	return fmt.Errorf("instance type %s is not supported in the current region %s", string(p.Spec.Instance.Type), p.Spec.Instance.Region)
}

func (p *Provider) DryRun() error {
	// Check if the desired instance type is supported in the region
	p.log.Wg.Add(1)
	go p.log.Loading("Checking if instance type %s is supported in region %s", string(p.Spec.Instance.Type), p.Spec.Instance.Region)
	err := p.checkInstanceTypes()
	if err != nil {
		p.fail()
		return err
	}
	p.done()

	// Check if the desired image is supported in the region
	p.log.Wg.Add(1)
	go p.log.Loading("Checking if image %s is supported in region %s", *p.Spec.Instance.Image.ImageId, p.Spec.Instance.Region)
	err = p.checkImages()
	if err != nil {
		p.fail()
		return fmt.Errorf("failed to get images: %v", err)
	}
	p.done()

	return nil
}
