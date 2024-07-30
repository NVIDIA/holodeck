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

	"github.com/vmware/govmomi/find"
)

// Delete deletes the Vsphere VM
func (p *Provider) Delete() error {
	cache, err := p.unmarsalCache()
	if err != nil {
		return fmt.Errorf("error retrieving cache: %v", err)
	}
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Destroying", "Destroying Vsphere resources")

	if err := p.delete(cache); err != nil {
		return fmt.Errorf("error destroying Vsphere resources: %v", err)
	}

	return nil
}

func (p *Provider) delete(cache *Vsphere) error {
	var err error

	// Delete the VM
	finder := find.NewFinder(p.vsphereClient.Client, false)

	dc, err := finder.Datacenter(context.TODO(), cache.Datacenter)
	if err != nil {
		return fmt.Errorf("error finding datacenter: %v", err)
	}
	finder.SetDatacenter(dc)

	vm, err := finder.VirtualMachine(context.TODO(), cache.VmName)
	if err != nil {
		return fmt.Errorf("error finding VM: %v", err)
	}

	task, err := vm.PowerOff(context.TODO())
	if err != nil {
		return fmt.Errorf("error powering off VM: %v", err)
	}
	err = task.Wait(context.TODO())
	if err != nil {
		return fmt.Errorf("error waiting for VM power off task: %v", err)
	}

	task, err = vm.Destroy(context.TODO())
	if err != nil {
		return fmt.Errorf("error destroying VM: %v", err)
	}

	p.log.Wg.Add(1)
	go p.log.Loading("Deleting Vsphere VM")
	err = task.Wait(context.TODO())
	if err != nil {
		p.fail()
		return fmt.Errorf("error waiting for VM destroy task: %v", err)
	}
	p.done()

	return p.updateTerminatedCondition(*p.Environment, cache)
}
