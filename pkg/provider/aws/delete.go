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
func (a *Client) Delete() error {
	cache, err := a.unmarsalCache()
	if err != nil {
		return fmt.Errorf("error retrieving cache: %v", err)
	}
	a.updateProgressingCondition(*a.Environment.DeepCopy(), cache, "v1alpha1.Destroying", "Destroying AWS resources")

	if err := a.delete(cache); err != nil {
		return fmt.Errorf("error destroying AWS resources: %v", err)
	}

	return nil
}

func (a *Client) delete(cache *AWS) error {
	// Delete the EC2 instance
	terminateInstancesInput := &ec2.TerminateInstancesInput{
		InstanceIds: []string{cache.Instanceid},
	}
	_, err := a.ec2.TerminateInstances(context.Background(), terminateInstancesInput)
	if err != nil {
		return fmt.Errorf("error deleting instance: %v", err)
	}

	a.log.Wg.Add(1)
	go a.log.Loading("Waiting for instance %s to be terminated\n", cache.Instanceid)

	waiterOptions := []func(*ec2.InstanceTerminatedWaiterOptions){
		func(o *ec2.InstanceTerminatedWaiterOptions) {
			o.MaxDelay = 1 * time.Minute
			o.MinDelay = 5 * time.Second
		},
	}
	wait := ec2.NewInstanceTerminatedWaiter(a.ec2, waiterOptions...)
	if err := wait.Wait(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{cache.Instanceid},
	}, 5*time.Minute, waiterOptions...); err != nil {
		a.fail()
		return fmt.Errorf("error waiting for instance to be terminated: %v", err)
	}

	// Delete the security group
	deleteSecurityGroup := &ec2.DeleteSecurityGroupInput{
		GroupId: &cache.SecurityGroupid,
	}
	_, err = a.ec2.DeleteSecurityGroup(context.Background(), deleteSecurityGroup)
	if err != nil {
		a.fail()
		return fmt.Errorf("error deleting security group: %v", err)
	}

	// Delete the subnet
	deleteSubnet := &ec2.DeleteSubnetInput{
		SubnetId: &cache.Subnetid,
	}
	_, err = a.ec2.DeleteSubnet(context.Background(), deleteSubnet)
	if err != nil {
		a.fail()
		return fmt.Errorf("error deleting subnet: %v", err)
	}

	// Delete the route tables
	deleteRouteTable := &ec2.DeleteRouteTableInput{
		RouteTableId: &cache.RouteTable,
	}
	_, err = a.ec2.DeleteRouteTable(context.Background(), deleteRouteTable)
	if err != nil {
		a.fail()
		return fmt.Errorf("error deleting route table: %v", err)
	}

	// Detach the Internet Gateway
	detachInternetGateway := &ec2.DetachInternetGatewayInput{
		InternetGatewayId: &cache.InternetGwid,
		VpcId:             &cache.Vpcid,
	}
	_, err = a.ec2.DetachInternetGateway(context.Background(), detachInternetGateway)
	if err != nil {
		a.fail()
		return fmt.Errorf("error detaching Internet Gateway: %v", err)
	}

	// Delete the Internet Gateway
	deleteInternetGatewayInput := &ec2.DeleteInternetGatewayInput{
		InternetGatewayId: &cache.InternetGwid,
	}
	_, err = a.ec2.DeleteInternetGateway(context.Background(), deleteInternetGatewayInput)
	if err != nil {
		a.fail()
		return fmt.Errorf("error deleting Internet Gateway: %v", err)
	}

	// Delete the VPC
	dVpc := &ec2.DeleteVpcInput{
		VpcId: &cache.Vpcid,
	}
	_, err = a.ec2.DeleteVpc(context.Background(), dVpc)
	if err != nil {
		a.fail()
		return fmt.Errorf("error deleting VPC: %v", err)
	}

	a.done()
	return a.updateTerminatedCondition(*a.Environment, cache)
}
