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

package awsfake

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	internalaws "github.com/NVIDIA/holodeck/internal/aws"
)

// FakeSSM is a stateful in-memory implementation of internalaws.SSMClient.
type FakeSSM struct {
	store *Store
}

var _ internalaws.SSMClient = (*FakeSSM)(nil)

// GetParameter returns a seeded parameter value, or a permissive default AMI id
// (defaultAMI) for any unseeded name so the AMI resolver's SSM path resolves.
func (f *FakeSSM) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("GetParameter", params)
	if err := f.store.failure("GetParameter"); err != nil {
		return nil, err
	}
	name := aws.ToString(params.Name)
	value, ok := f.store.Parameters[name]
	if !ok {
		value = defaultAMI
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  aws.String(name),
			Value: aws.String(value),
		},
	}, nil
}
