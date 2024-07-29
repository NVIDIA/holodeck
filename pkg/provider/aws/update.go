/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Update updates an AWS resources tags
func (a *Client) UpdateResourcesTags(tags []types.Tag, resources ...string) error {
	a.log.Wg.Add(1)
	go a.log.Loading("Tagging AWS resources...")

	createTagsIn := &ec2.CreateTagsInput{
		Resources: resources,
		Tags:      tags,
	}

	_, err := a.ec2.CreateTags(context.Background(), createTagsIn)
	if err != nil {
		a.fail()
		return err
	}
	a.done()

	return nil
}
