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

package provider

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Provider is the base interface for all other Provider interfaces
type Provider interface {
	// Name returns a friendly name for this Provider
	Name() string

	// Core methods

	// Create creates the resources in the provider
	Create() error
	// Delete deletes the resources in the provider
	Delete() error
	// DryRun preflight checks
	DryRun() error
	// Status returns the status of the resources in the provider
	Status() ([]metav1.Condition, error)

	// Metada methods
	UpdateResourcesTags(tags map[string]string, resources ...string) error
}
