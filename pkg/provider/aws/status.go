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
	"time"

	"sigs.k8s.io/yaml"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"

	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionProgressing string = "Progressing"
	ConditionDegraded    string = "Degraded"
	ConditionAvailable   string = "Available"
	ConditionTerminated  string = "Terminated"
)

func (a *Client) updateStatus(env v1alpha1.Environment, cache *AWS, condition []metav1.Condition) error {
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

	// Because there are only 3 possible conditions (degraded, available,
	// and progressing), it isn't necessary to check if old
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
		for _, p := range envCopy.Status.Properties {
			switch p.Name {
			case VpcID:
				if p.Value != cache.Vpcid {
					p.Value = cache.Vpcid
					modified = true
				}
			case SubnetID:
				if p.Value != cache.Subnetid {
					p.Value = cache.Subnetid
					modified = true
				}
			case InternetGwID:
				if p.Value != cache.InternetGwid {
					p.Value = cache.InternetGwid
					modified = true
				}
			case InternetGatewayAttachment:
				if p.Value != cache.InternetGatewayAttachment {
					p.Value = cache.InternetGatewayAttachment
					modified = true
				}
			case RouteTable:
				if p.Value != cache.RouteTable {
					p.Value = cache.RouteTable
					modified = true
				}
			case SecurityGroupID:
				if p.Value != cache.SecurityGroupid {
					p.Value = cache.SecurityGroupid
					modified = true
				}
			case InstanceID:
				if p.Value != cache.Instanceid {
					p.Value = cache.Instanceid
					modified = true
				}
			case PublicDnsName:
				if p.Value != cache.PublicDnsName {
					p.Value = cache.PublicDnsName
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

	return update(envCopy, a.cacheFile)
}

// update the status of the aws object into a cache file
func update(env *v1alpha1.Environment, cachePath string) error {
	data, err := yaml.Marshal(env)
	if err != nil {
		return err
	}

	// if the cache directory doesn't exist, create it
	_, err = os.Stat(cachePath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(cachePath, 0755)
		if err != nil {
			return err
		}
	}

	// write to file
	err = os.WriteFile(cachePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

// updateAvailableCondition is used to mark a given resource as "available".
func (a *Client) updateAvailableCondition(env v1alpha1.Environment, cache *AWS) error {
	availableCondition := getAvailableConditions()
	if err := a.updateStatus(env, cache, availableCondition); err != nil {
		return err
	}

	return nil
}

// updateTerminatedCondition is used to mark a given resource as "terminated".
func (a *Client) updateTerminatedCondition(env v1alpha1.Environment, cache *AWS) error {
	terminatedCondition := getDegradedConditions("v1alpha1.Terminated", "AWS resources have been terminated")
	if err := a.updateStatus(env, cache, terminatedCondition); err != nil {
		return err
	}

	return nil
}

// updateProgressingCondition is used to mark a given resource as "progressing".
func (a *Client) updateProgressingCondition(env v1alpha1.Environment, cache *AWS, reason, message string) error {
	progressingCondition := getProgressingConditions(reason, message)
	if err := a.updateStatus(env, cache, progressingCondition); err != nil {
		return err
	}
	return nil
}

// updateDegradedCondition is used to mark a given resource as "degraded".
func (a *Client) updateDegradedCondition(env v1alpha1.Environment, cache *AWS, reason, message string) error {
	degradedCondition := getDegradedConditions(reason, message)
	if err := a.updateStatus(env, cache, degradedCondition); err != nil {
		return err
	}

	return nil
}

// getAvailableConditions returns a list of Condition objects and marks
// every condition as FALSE except for ConditionAvailable so that the
// reconciler can determine that the resource is available.
func getAvailableConditions() []metav1.Condition {
	now := time.Now()
	return []metav1.Condition{
		{
			Type:               ConditionAvailable,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               ConditionProgressing,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               ConditionDegraded,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
	}
}

// getDegradedConditions returns a list of conditions.Condition objects and marks
// every condition as FALSE except for conditions.ConditionDegraded so that the
// reconciler can determine that the resource is degraded.
func getDegradedConditions(reason string, message string) []metav1.Condition {
	now := time.Now()
	return []metav1.Condition{
		{
			Type:               ConditionAvailable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               ConditionProgressing,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               ConditionDegraded,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             reason,
			Message:            message,
		},
	}
}

// getProgressingConditions returns a list of Condition objects and marks
// every condition as FALSE except for ConditionProgressing so that the
// reconciler can determine that the resource is progressing.
func getProgressingConditions(reason string, message string) []metav1.Condition {
	now := time.Now()
	return []metav1.Condition{
		{
			Type:               ConditionAvailable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               ConditionProgressing,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             reason,
			Message:            message,
		},
		{
			Type:               ConditionDegraded,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
	}
}
