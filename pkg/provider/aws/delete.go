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
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Delete deletes the EC2 instance and all associated resources
func (p *Provider) Delete() error {
	cache, err := p.unmarsalCache()
	if err != nil {
		return fmt.Errorf("error retrieving cache: %v", err)
	}
	p.updateProgressingCondition(*p.Environment.DeepCopy(), cache, "v1alpha1.Destroying", "Destroying AWS resources")

	if err := p.delete(cache); err != nil {
		return fmt.Errorf("error destroying AWS resources: %v", err)
	}

	return nil
}

func (p *Provider) delete(cache *AWS) error {
	var err error
	// Delete the EC2 instance
	if cache.Instanceid == "" {
		p.log.Warning("No instance found to delete")
	} else {
		terminateInstancesInput := &ec2.TerminateInstancesInput{
			InstanceIds: []string{cache.Instanceid},
		}
		_, err := p.ec2.TerminateInstances(context.Background(), terminateInstancesInput)
		if err != nil {
			return fmt.Errorf("error deleting instance: %v", err)
		}

		p.log.Wg.Add(1)
		go p.log.Loading("Waiting for instance %s to be terminated", cache.Instanceid)

		waiterOptions := []func(*ec2.InstanceTerminatedWaiterOptions){
			func(o *ec2.InstanceTerminatedWaiterOptions) {
				o.MaxDelay = 10 * time.Minute
				o.MinDelay = 5 * time.Second
			},
		}
		wait := ec2.NewInstanceTerminatedWaiter(p.ec2, waiterOptions...)
		if err := wait.Wait(context.Background(), &ec2.DescribeInstancesInput{
			InstanceIds: []string{cache.Instanceid},
		}, 10*time.Minute, waiterOptions...); err != nil {
			p.fail()
			return fmt.Errorf("error waiting for instance to be terminated: %v", err)
		}

		// Delete the security group
		deleteSecurityGroup := &ec2.DeleteSecurityGroupInput{
			GroupId: &cache.SecurityGroupid,
		}
		_, err = p.ec2.DeleteSecurityGroup(context.Background(), deleteSecurityGroup)
		if err != nil {
			p.fail()
			return fmt.Errorf("error deleting security group: %v", err)
		}

		p.done()
	}

	p.log.Wg.Add(1)
	go p.log.Loading("Deleting VPC resources")
	// Delete the subnet
	deleteSubnet := &ec2.DeleteSubnetInput{
		SubnetId: &cache.Subnetid,
	}
	_, err = p.ec2.DeleteSubnet(context.Background(), deleteSubnet)
	if err != nil {
		p.fail()
		return fmt.Errorf("error deleting subnet: %v", err)
	}

	// Delete the route tables
	deleteRouteTable := &ec2.DeleteRouteTableInput{
		RouteTableId: &cache.RouteTable,
	}
	_, err = p.ec2.DeleteRouteTable(context.Background(), deleteRouteTable)
	if err != nil {
		p.fail()
		return fmt.Errorf("error deleting route table: %v", err)
	}

	// Detach the Internet Gateway
	detachInternetGateway := &ec2.DetachInternetGatewayInput{
		InternetGatewayId: &cache.InternetGwid,
		VpcId:             &cache.Vpcid,
	}
	_, err = p.ec2.DetachInternetGateway(context.Background(), detachInternetGateway)
	if err != nil {
		p.fail()
		return fmt.Errorf("error detaching Internet Gateway: %v", err)
	}

	// Delete the Internet Gateway
	deleteInternetGatewayInput := &ec2.DeleteInternetGatewayInput{
		InternetGatewayId: &cache.InternetGwid,
	}
	_, err = p.ec2.DeleteInternetGateway(context.Background(), deleteInternetGatewayInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error deleting Internet Gateway: %v", err)
	}

	// Delete the VPC
	dVpc := &ec2.DeleteVpcInput{
		VpcId: &cache.Vpcid,
	}
	_, err = p.ec2.DeleteVpc(context.Background(), dVpc)
	if err != nil {
		p.fail()
		return fmt.Errorf("error deleting VPC: %v", err)
	}

	p.done()
	return p.updateTerminatedCondition(*p.Environment, cache)
}
