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

package vsphere

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

func (p *Provider) updateStatus(env v1alpha1.Environment, cache *Vsphere, condition []metav1.Condition) error {
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
			{Name: VMName, Value: cache.VmName},
			{Name: TemplateImage, Value: cache.TemplateImage},
			{Name: Cluster, Value: cache.Cluster},
			{Name: DataStore, Value: cache.DataStore},
			{Name: Datacenter, Value: cache.Datacenter},
			{Name: Network, Value: cache.Network},
			{Name: ResourcePool, Value: cache.ResourcePool},
			{Name: VMFolder, Value: cache.VMFolder},
			{Name: IpAddress, Value: cache.IpAddress},
		}
		modified = true
	} else {
		for _, p := range envCopy.Status.Properties {
			switch p.Name {
			case VMName:
				if p.Value != cache.VmName {
					p.Value = cache.VmName
					modified = true
				}
			case TemplateImage:
				if p.Value != cache.TemplateImage {
					p.Value = cache.TemplateImage
					modified = true
				}
			case Cluster:
				if p.Value != cache.Cluster {
					p.Value = cache.Cluster
					modified = true
				}
			case DataStore:
				if p.Value != cache.DataStore {
					p.Value = cache.DataStore
					modified = true
				}
			case Datacenter:
				if p.Value != cache.Datacenter {
					p.Value = cache.Datacenter
					modified = true
				}
			case Network:
				if p.Value != cache.Network {
					p.Value = cache.Network
					modified = true
				}
			case ResourcePool:
				if p.Value != cache.ResourcePool {
					p.Value = cache.ResourcePool
					modified = true
				}
			case VMFolder:
				if p.Value != cache.VMFolder {
					p.Value = cache.VMFolder
					modified = true
				}
			case IpAddress:
				if p.Value != cache.IpAddress {
					p.Value = cache.IpAddress
					modified = true
				}
			default:
				// Ignore non Vsphere infra properties
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

	// if the cache file does not exist, check if the directory exists
	// if the directory does not exist, create it
	// if the directory exists, create the cache file
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		dir := filepath.Dir(cachePath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				return err
			}
		}
		_, err := os.Create(cachePath)
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
func (p *Provider) updateAvailableCondition(env v1alpha1.Environment, cache *Vsphere) error {
	availableCondition := getAvailableConditions()
	if err := p.updateStatus(env, cache, availableCondition); err != nil {
		return err
	}

	return nil
}

// updateTerminatedCondition is used to mark a given resource as "terminated".
func (p *Provider) updateTerminatedCondition(env v1alpha1.Environment, cache *Vsphere) error {
	terminatedCondition := getTerminatedConditions("v1alpha1.Terminated", "Vsphere resources have been terminated")
	if err := p.updateStatus(env, cache, terminatedCondition); err != nil {
		return err
	}

	return nil
}

// updateProgressingCondition is used to mark a given resource as "progressing".
func (p *Provider) updateProgressingCondition(env v1alpha1.Environment, cache *Vsphere, reason, message string) error {
	progressingCondition := getProgressingConditions(reason, message)
	if err := p.updateStatus(env, cache, progressingCondition); err != nil {
		return err
	}
	return nil
}

// updateDegradedCondition is used to mark a given resource as "degraded".
func (p *Provider) updateDegradedCondition(env v1alpha1.Environment, cache *Vsphere, reason, message string) error {
	degradedCondition := getDegradedConditions(reason, message)
	if err := p.updateStatus(env, cache, degradedCondition); err != nil {
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
			Type:               v1alpha1.ConditionAvailable,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionProgressing,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionDegraded,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionTerminated,
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
			Type:               v1alpha1.ConditionAvailable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionProgressing,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionDegraded,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             reason,
			Message:            message,
		},
		{
			Type:               v1alpha1.ConditionTerminated,
			Status:             metav1.ConditionFalse,
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
			Type:               v1alpha1.ConditionAvailable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionProgressing,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             reason,
			Message:            message,
		},
		{
			Type:               v1alpha1.ConditionDegraded,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionTerminated,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             reason,
			Message:            message,
		},
	}
}

// getTerminatedConditions returns a list of conditions.Condition objects and marks
// every condition as FALSE except for conditions.ConditionTerminated so that the
// reconciler can determine that the resource is terminated.
func getTerminatedConditions(reason string, message string) []metav1.Condition {
	now := time.Now()
	return []metav1.Condition{
		{
			Type:               v1alpha1.ConditionAvailable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionProgressing,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionDegraded,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
		},
		{
			Type:               v1alpha1.ConditionTerminated,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             reason,
			Message:            message,
		},
	}
}
