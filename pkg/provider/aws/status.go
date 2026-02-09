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
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"

	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) Status() ([]metav1.Condition, error) {
	// Read the cache file
	data, err := os.ReadFile(p.cacheFile)
	if err != nil {
		return []metav1.Condition{}, err
	}

	// Unmarshal the data into a v1alpha1.Environment object
	var env v1alpha1.Environment
	err = yaml.Unmarshal(data, &env)
	if err != nil {
		return []metav1.Condition{}, err
	}

	if len(env.Status.Conditions) == 0 {
		return []metav1.Condition{}, nil
	}

	return env.Status.Conditions, nil
}

func (p *Provider) updateStatus(env v1alpha1.Environment, cache *AWS, condition []metav1.Condition) error {
	// The actual 'env' object should *not* be modified when trying to
	// check the object's status. This variable is a dummy variable used
	// to set temporary conditions.
	envCopy := env.DeepCopy()

	// If a set of conditions exists, then it should be added to the
	// 'env' Copy.
	if condition != nil {
		envCopy.Status.Conditions = condition
	}

	// Next step is to check if we need to update the status
	modified := false

	// Because there are only 4 possible conditions (degraded, available,
	// progressing and Terminated), it isn't necessary to check if old
	// conditions should be removed.
	for _, newCondition := range envCopy.Status.Conditions {
		oldCondition := meta.FindStatusCondition(env.Status.Conditions, newCondition.Type)
		if oldCondition == nil {
			modified = true
			break
		}
		// Ignore timestamps to avoid infinite reconcile loops
		if oldCondition.Status != newCondition.Status ||
			oldCondition.Reason != newCondition.Reason ||
			oldCondition.Message != newCondition.Message {
			modified = true
			break
		}
	}

	if len(envCopy.Status.Properties) == 0 {
		envCopy.Status.Properties = []v1alpha1.Properties{
			{Name: VpcID, Value: cache.Vpcid},
			{Name: SubnetID, Value: cache.Subnetid},
			{Name: InternetGwID, Value: cache.InternetGwid},
			{Name: InternetGatewayAttachment, Value: cache.InternetGatewayAttachment},
			{Name: RouteTable, Value: cache.RouteTable},
			{Name: SecurityGroupID, Value: cache.SecurityGroupid},
			{Name: InstanceID, Value: cache.Instanceid},
			{Name: PublicDnsName, Value: cache.PublicDnsName},
		}
		modified = true
	} else {
		for _, properties := range envCopy.Status.Properties {
			switch properties.Name {
			case VpcID:
				if properties.Value != cache.Vpcid {
					properties.Value = cache.Vpcid
					modified = true
				}
			case SubnetID:
				if properties.Value != cache.Subnetid {
					properties.Value = cache.Subnetid
					modified = true
				}
			case InternetGwID:
				if properties.Value != cache.InternetGwid {
					properties.Value = cache.InternetGwid
					modified = true
				}
			case InternetGatewayAttachment:
				if properties.Value != cache.InternetGatewayAttachment {
					properties.Value = cache.InternetGatewayAttachment
					modified = true
				}
			case RouteTable:
				if properties.Value != cache.RouteTable {
					properties.Value = cache.RouteTable
					modified = true
				}
			case SecurityGroupID:
				if properties.Value != cache.SecurityGroupid {
					properties.Value = cache.SecurityGroupid
					modified = true
				}
			case InstanceID:
				if properties.Value != cache.Instanceid {
					properties.Value = cache.Instanceid
					modified = true
				}
			case PublicDnsName:
				if properties.Value != cache.PublicDnsName {
					properties.Value = cache.PublicDnsName
					modified = true
				}
			default:
				// Ignore non AWS infra properties
				continue
			}
		}
	}

	// If nothing has been modified, then return nothing. Even if the list
	// of 'conditions' is not empty, it should not be counted as an update
	// if it was already counted as an update before.
	if !modified {
		return nil
	}

	return update(envCopy, p.cacheFile)
}

// update the status of the aws object into a cache file
func update(env *v1alpha1.Environment, cachePath string) error {
	data, err := yaml.Marshal(env)
	if err != nil {
		return err
	}

	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err != nil {
		return err
	}

	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		return err
	}
	// Enforce permissions even if the file already existed with broader perms
	return os.Chmod(cachePath, 0600)
}

// updateCondition is used to update the condition of a given resource.
func (p *Provider) updateCondition(env v1alpha1.Environment, cache *AWS, conditions []metav1.Condition) error {
	return p.updateStatus(env, cache, conditions)
}

// updateAvailableCondition is used to mark a given resource as "available".
func (p *Provider) updateAvailableCondition(env v1alpha1.Environment, cache *AWS) error {
	return p.updateCondition(env, cache, getAvailableConditions())
}

// updateTerminatedCondition is used to mark a given resource as "terminated".
func (p *Provider) updateTerminatedCondition(env v1alpha1.Environment, cache *AWS) error {
	return p.updateCondition(env, cache, getTerminatedConditions("v1alpha1.Terminated", "AWS resources have been terminated"))
}

// updateProgressingCondition is used to mark a given resource as "progressing".
func (p *Provider) updateProgressingCondition(env v1alpha1.Environment, cache *AWS, reason, message string) error {
	return p.updateCondition(env, cache, getProgressingConditions(reason, message))
}

// updateDegradedCondition is used to mark a given resource as "degraded".
func (p *Provider) updateDegradedCondition(env v1alpha1.Environment, cache *AWS, reason, message string) error {
	return p.updateCondition(env, cache, getDegradedConditions(reason, message))
}

// buildConditions creates a standard set of conditions with the specified type set to True.
func buildConditions(trueType string, reason, message string) []metav1.Condition {
	now := time.Now()
	types := []string{
		v1alpha1.ConditionAvailable,
		v1alpha1.ConditionProgressing,
		v1alpha1.ConditionDegraded,
		v1alpha1.ConditionTerminated,
	}
	conditions := make([]metav1.Condition, 0, len(types))
	for _, t := range types {
		c := metav1.Condition{
			Type:               t,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		}
		if t == trueType {
			c.Status = metav1.ConditionTrue
		}
		// Propagate reason/message to the true condition and to ConditionTerminated
		// (preserving original behavior where degraded/progressing set reason on Terminated)
		if t == trueType || (t == v1alpha1.ConditionTerminated && reason != "") {
			c.Reason = reason
			c.Message = message
		}
		conditions = append(conditions, c)
	}
	return conditions
}

func getAvailableConditions() []metav1.Condition {
	return buildConditions(v1alpha1.ConditionAvailable, "", "")
}

func getDegradedConditions(reason, message string) []metav1.Condition {
	return buildConditions(v1alpha1.ConditionDegraded, reason, message)
}

func getProgressingConditions(reason, message string) []metav1.Condition {
	return buildConditions(v1alpha1.ConditionProgressing, reason, message)
}

func getTerminatedConditions(reason, message string) []metav1.Condition {
	return buildConditions(v1alpha1.ConditionTerminated, reason, message)
}
