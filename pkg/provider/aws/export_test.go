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

package aws

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

// Export unexported functions for testing

// GetAvailableConditions exports getAvailableConditions for testing.
func GetAvailableConditions() []metav1.Condition {
	return getAvailableConditions()
}

// GetDegradedConditions exports getDegradedConditions for testing.
func GetDegradedConditions(reason, message string) []metav1.Condition {
	return getDegradedConditions(reason, message)
}

// GetProgressingConditions exports getProgressingConditions for testing.
func GetProgressingConditions(reason, message string) []metav1.Condition {
	return getProgressingConditions(reason, message)
}

// GetTerminatedConditions exports getTerminatedConditions for testing.
func GetTerminatedConditions(reason, message string) []metav1.Condition {
	return getTerminatedConditions(reason, message)
}

// Update exports update for testing.
func Update(env *v1alpha1.Environment, cachePath string) error {
	return update(env, cachePath)
}
