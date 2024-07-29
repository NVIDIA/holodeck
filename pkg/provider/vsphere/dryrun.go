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
)

func (p *Provider) DryRun() error {
	// Create a context with default timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	var err error

	// Create a new finder object
	finder := find.NewFinder(p.vsphereClient.Client, false)

	// Find the datacenter

	p.log.Wg.Add(1)
	go p.log.Loading("Verifying datacenter\t-\t%s", p.Environment.Spec.VsphereVirtualMachine.Datacenter)
	dc, err := finder.Datacenter(ctx, p.Environment.Spec.VsphereVirtualMachine.Datacenter)
	if err != nil {
		p.fail()
		return fmt.Errorf("error finding datacenter: %v", err)
	}
	finder.SetDatacenter(dc)
	p.done()

	// Find the cluster
	if p.Environment.Spec.VsphereVirtualMachine.Cluster != "" {
		p.log.Wg.Add(1)
		go p.log.Loading("Verifying cluster\t-\t%s", p.Environment.Spec.VsphereVirtualMachine.Cluster)
		_, err = finder.Datastore(ctx, p.Environment.Spec.VsphereVirtualMachine.DataStore)
		if err != nil {
			p.fail()
			return fmt.Errorf("error finding datastore: %v", err)
		}
		p.done()
	}

	// Find the network
	if p.Environment.Spec.VsphereVirtualMachine.Network != "" {
		p.log.Wg.Add(1)
		go p.log.Loading("Verifying network\t-\t%s", p.Environment.Spec.VsphereVirtualMachine.Network)
		_, err = finder.Network(ctx, p.Environment.Spec.VsphereVirtualMachine.Network)
		if err != nil {
			p.fail()
			return fmt.Errorf("error finding network: %v", err)
		}
		p.done()
	}

	// Find the template
	if p.Environment.Spec.VsphereVirtualMachine.TemplateImage != "" {
		p.log.Wg.Add(1)
		go p.log.Loading("Verifying template\t-\t%s", p.Environment.Spec.VsphereVirtualMachine.TemplateImage)
		_, err = finder.VirtualMachine(ctx, p.Environment.Spec.VsphereVirtualMachine.TemplateImage)
		if err != nil {
			p.fail()
			return fmt.Errorf("error finding template: %v", err)
		}
		p.done()
	}

	// Find the folder
	if p.Environment.Spec.VsphereVirtualMachine.VMFolder != "" {
		p.log.Wg.Add(1)
		go p.log.Loading("Verifying folderv%s", p.Environment.Spec.VsphereVirtualMachine.VMFolder)
		_, err = finder.Folder(ctx, p.Environment.Spec.VsphereVirtualMachine.VMFolder)
		if err != nil {
			p.fail()
			return fmt.Errorf("error finding folder: %v", err)
		}
		p.done()
	}

	// Find the resource pool
	if p.Environment.Spec.VsphereVirtualMachine.ResoursePool != "" {
		p.log.Wg.Add(1)
		go p.log.Loading("Verifying resource pool\t-\t%s", p.Environment.Spec.VsphereVirtualMachine.ResoursePool)
		_, err = finder.ResourcePool(ctx, p.Environment.Spec.VsphereVirtualMachine.ResoursePool)
		if err != nil {
			p.fail()
			return fmt.Errorf("error finding resource pool: %v", err)
		}
		p.done()
	}

	return nil
}
