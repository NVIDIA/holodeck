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
)

func (a *Client) getInstanceTypes() ([]string, error) {
	// Use the DescribeInstanceTypes API to get a list of supported instance types in the current region
	resp, err := a.ec2.DescribeInstanceTypes(context.TODO(), &ec2.DescribeInstanceTypesInput{})
	if err != nil {
		return nil, err
	}

	instanceTypes := make([]string, 0)
	for _, it := range resp.InstanceTypes {
		instanceTypes = append(instanceTypes, string(it.InstanceType))
	}

	return instanceTypes, nil
}

func (a *Client) isInstanceTypeSupported(desiredType string, supportedTypes []string) bool {
	for _, t := range supportedTypes {
		if t == desiredType {
			return true
		}
	}
	return false
}

func (a *Client) DryRun() error {
	// Check if the desired instance type is supported in the region
	instanceTypes, err := a.getInstanceTypes()
	if err != nil {
		return fmt.Errorf("failed to get instance types: %v", err)
	}

	if !a.isInstanceTypeSupported(string(a.Spec.Instance.Type), instanceTypes) {
		return fmt.Errorf("instance type %s is not supported in the current region", string(a.Spec.Instance.Type))
	}

	return nil
}
