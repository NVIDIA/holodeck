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
	"net/url"
	"os"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
	"sigs.k8s.io/yaml"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
)

const (
	// Name of this builder provider
	Name            = "vsphere"
	vCenterURL      = "vCenterURL"
	vCenterUsername = "vCenterUsername"
	vCenterPassword = "vCenterPassword"
	Datacenter      = "Datacenter"
	DataStore       = "DataStore"
	Cluster         = "Cluster"
	Network         = "Network"
	VMFolder        = "VMFolder"
	TemplateImage   = "TemplateImage"
	VMName          = "VMName"
	ResourcePool    = "ResourcePool"
	IpAddress       = "IpAddress"
)

type Vsphere struct {
	VmName        string
	TemplateImage string
	Cluster       string
	DataStore     string
	Datacenter    string
	Network       string
	VMFolder      string
	ResourcePool  string
	IpAddress     string
}

type Provider struct {
	vsphereClient *govmomi.Client
	cacheFile     string

	*v1alpha1.Environment
	log *logger.FunLogger
}

func New(log *logger.FunLogger, env v1alpha1.Environment, cacheFile string) (*Provider, error) {
	// Set a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create an VSPHERE client
	provider := &Provider{
		cacheFile:   cacheFile,
		Environment: &env,
		log:         log,
	}

	soapURL, err := soap.ParseURL(env.Spec.VsphereVirtualMachine.VCenterURL)
	if err != nil {
		return nil, fmt.Errorf("URL parsing error: %v", err)
	}

	// Get the username and password from env vars
	user := os.Getenv("HOLODECK_VCENTER_USERNAME")
	if user == "" {
		return nil, fmt.Errorf("HOLODECK_VCENTER_USERNAME not set")
	}
	pass := os.Getenv("HOLODECK_VCENTER_PASSWORD")
	if pass == "" {
		return nil, fmt.Errorf("HOLODECK_VCENTER_PASSWORD not set")
	}

	soapURL.User = url.UserPassword(user, pass)

	// Retry connecting to vCenter 5 times before giving up
	var vclient *govmomi.Client
	for i := 0; i < 5; i++ {
		vclient, err = govmomi.NewClient(ctx, soapURL, true)
		if err != nil && i < 4 {
			fmt.Fprintf(os.Stderr, "Error connecting to vCenter: %v...Retrying\n", err)
			time.Sleep(10 * time.Second)
			continue
		} else if err != nil {
			return nil, fmt.Errorf("error connecting to vCenter: %v", err)
		}
	}

	provider.vsphereClient = vclient

	return provider, nil
}

// Name returns the name of the builder provisioner
func (p *Provider) Name() string { return Name }

// unmarsalCache unmarshals the cache file into the AWS struct
func (p *Provider) unmarsalCache() (*Vsphere, error) {
	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](p.cacheFile)
	if err != nil {
		return nil, err
	}

	vm := &Vsphere{}

	for _, properties := range env.Status.Properties {
		switch properties.Name {
		case VMName:
			vm.VmName = properties.Value
		case TemplateImage:
			vm.TemplateImage = properties.Value
		case Cluster:
			vm.Cluster = properties.Value
		case DataStore:
			vm.DataStore = properties.Value
		case Datacenter:
			vm.Datacenter = properties.Value
		case Network:
			vm.Network = properties.Value
		case VMFolder:
			vm.VMFolder = properties.Value
		case ResourcePool:
			vm.ResourcePool = properties.Value
		case IpAddress:
			vm.IpAddress = properties.Value
		default:
			// Ignore non AWS infra properties
			continue
		}
	}

	return vm, nil
}

// dump writes the AWS struct to the cache file
func (p *Provider) dumpCache(vm *Vsphere) error {
	env := p.Environment.DeepCopy()
	env.Status.Properties = []v1alpha1.Properties{
		{Name: VMName, Value: vm.VmName},
		{Name: TemplateImage, Value: vm.TemplateImage},
		{Name: Cluster, Value: vm.Cluster},
		{Name: Datacenter, Value: vm.Datacenter},
		{Name: DataStore, Value: vm.DataStore},
		{Name: Network, Value: vm.Network},
		{Name: VMFolder, Value: vm.VMFolder},
		{Name: IpAddress, Value: vm.IpAddress},
	}

	data, err := yaml.Marshal(env)
	if err != nil {
		return err
	}

	err = os.WriteFile(p.cacheFile, data, 0644)
	if err != nil {
		return err
	}

	// Update the environment object with the new properties
	p.Environment = env

	return nil
}

func (p *Provider) done() {
	p.log.Done <- struct{}{}
	p.log.Wg.Wait()
}

func (p *Provider) fail() {
	p.log.Fail <- struct{}{}
	p.log.Wg.Wait()
}
