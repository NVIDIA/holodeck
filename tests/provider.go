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

package e2e

import (
	"fmt"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provider"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provider/vsphere"
)

func newProvider(log *logger.FunLogger, cfg v1alpha1.Environment, cacheFile string) (provider.Provider, error) {
	var provider provider.Provider
	var err error

	switch cfg.Spec.Provider {
	case v1alpha1.ProviderAWS:
		provider, err = newAwsProvider(log, cfg, cacheFile)
		if err != nil {
			return nil, err
		}
	case v1alpha1.ProviderVSphere:
		provider, err = newVsphereProvider(log, cfg, cacheFile)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("provider %s not supported", cfg.Spec.Provider)
	}

	return provider, nil
}

func newAwsProvider(log *logger.FunLogger, cfg v1alpha1.Environment, cacheFile string) (*aws.Provider, error) {
	a, err := aws.New(log, cfg, cacheFile)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func newVsphereProvider(log *logger.FunLogger, cfg v1alpha1.Environment, cacheFile string) (*vsphere.Provider, error) {
	v, err := vsphere.New(log, cfg, cacheFile)
	if err != nil {
		return nil, err
	}

	return v, nil
}
