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
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

// Create creates a VM on VSphere
func (p *Provider) Create() error {
	cache := new(Vsphere)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Creating Vsphere resources")

	vmName := p.Environment.Name
	// cache the input values
	cache.VmName = vmName
	cache.Cluster = p.Environment.Spec.VsphereVirtualMachine.Cluster
	cache.ResourcePool = p.Environment.Spec.VsphereVirtualMachine.ResoursePool

	// Create a new finder object
	finder := find.NewFinder(p.vsphereClient.Client, false)

	// Find the datacenter
	dc, err := finder.Datacenter(ctx, p.Environment.Spec.VsphereVirtualMachine.Datacenter)
	if err != nil {
		p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Finding Datacenter")
		return fmt.Errorf("error finding datacenter: %v", err)
	}
	finder.SetDatacenter(dc)
	cache.Datacenter = dc.Name()
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Datacenter Found")

	// Find the cluster
	datastore, err := finder.Datastore(ctx, p.Environment.Spec.VsphereVirtualMachine.DataStore)
	if err != nil {
		p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Finding Datastore")
		return fmt.Errorf("error finding datastore: %v", err)
	}
	cache.DataStore = datastore.Name()
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Datastore Found")

	// Find the network
	_, err = finder.Network(ctx, p.Environment.Spec.VsphereVirtualMachine.Network)
	if err != nil {
		p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Finding Network")
		return fmt.Errorf("error finding network: %v", err)
	}
	cache.Network = p.Environment.Spec.VsphereVirtualMachine.Network
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Network Found")

	// Find the template
	template, err := finder.VirtualMachine(ctx, p.Environment.Spec.VsphereVirtualMachine.TemplateImage)
	if err != nil {
		p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Finding Template")
		return fmt.Errorf("error finding template: %v", err)
	}
	cache.TemplateImage = p.Environment.Spec.VsphereVirtualMachine.TemplateImage
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Template Found")

	// Find the folder
	var folder *object.Folder
	if p.Environment.Spec.VsphereVirtualMachine.VMFolder != "" {
		folder, err = finder.Folder(ctx, p.Environment.Spec.VsphereVirtualMachine.VMFolder)
		if err != nil {
			p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Finding Folder")
			return fmt.Errorf("error finding folder: %v", err)
		}
	} else {
		folder, err = finder.DefaultFolder(ctx)
		if err != nil {
			p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error creating VPC")
			return fmt.Errorf("error finding default folder: %v", err)
		}
	}
	cache.VMFolder = folder.Name()
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Folder Found")

	// Find the resource pool
	var pool *object.ResourcePool
	if p.Environment.Spec.VsphereVirtualMachine.ResoursePool != "" {
		pool, err = finder.ResourcePool(ctx, p.Environment.Spec.VsphereVirtualMachine.ResoursePool)
		if err != nil {
			p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Finding Resource Pool")
			return fmt.Errorf("error finding resource pool: %v", err)
		}
	} else {
		// Find the resource pool
		pool, err = finder.ResourcePoolOrDefault(ctx, "")
		if err != nil {
			p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Finding Resource Pool")
			return fmt.Errorf("error finding default resource pool: %v", err)
		}
	}
	cache.ResourcePool = pool.Name()
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Resource Pool Found")

	datastoreRefrence := datastore.Reference()
	folderReference := folder.Reference()
	poolReference := pool.Reference()

	relocateSpec := types.VirtualMachineRelocateSpec{
		Pool:      &poolReference,
		Folder:    &folderReference,
		Datastore: &datastoreRefrence,
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location: relocateSpec,
		PowerOn:  true,
		Template: false,
	}

	task, err := template.Clone(ctx, folder, vmName, cloneSpec)
	if err != nil {
		p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Cloning VM")
		return fmt.Errorf("error cloning VM: %v", err)
	}

	p.log.Wg.Add(1)
	go p.log.Loading("Cloning VM")
	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Waiting for VM to get cloned into node")
		p.fail()
		return fmt.Errorf("error waiting for task: %v", err)
	}
	p.done()
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "VM Cloned")

	newVm := object.NewVirtualMachine(p.vsphereClient.Client, info.Result.(types.ManagedObjectReference))

	// Wait for the VM to get an IP address
	p.log.Wg.Add(1)
	go p.log.Loading("Waiting for VM IP address")
	ip, err := waitForIP(ctx, newVm)
	if err != nil {
		p.updateDegradedCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error Waiting for VM IP address")
		p.fail()
		return fmt.Errorf("error waiting for VM IP address: %v", err)
	}
	cache.IpAddress = ip
	p.done()

	// Save objects ID's into a cache file
	if err := p.updateAvailableCondition(*p.Environment, cache); err != nil {
		return fmt.Errorf("error creating cache file: %v", err)
	}
	return nil
}

func waitForIP(ctx context.Context, vm *object.VirtualMachine) (string, error) {
	for {
		ip, err := vm.WaitForIP(ctx)
		if err == nil {
			return ip, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			// Retry after a short delay
		}
	}
}
