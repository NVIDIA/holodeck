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
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
)

// Create creates an EC2 instance with proper Network configuration
// VPC, Subnet, Internet Gateway, Route Table, Security Group
func (a *Client) Create() error {
	cache := new(AWS)
	defer a.dumpCache(cache)

	a.updateProgressingCondition(*a.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Creating AWS resources")

	if err := a.createVPC(cache); err != nil {
		a.updateDegradedCondition(*a.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error creating VPC")
		return fmt.Errorf("error creating VPC: %v", err)
	}

	if err := a.createSubnet(cache); err != nil {
		a.updateDegradedCondition(*a.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error creating subnet")
		return fmt.Errorf("error creating subnet: %v", err)
	}

	if err := a.createInternetGateway(cache); err != nil {
		a.updateDegradedCondition(*a.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error creating Internet Gateway")
		return fmt.Errorf("error creating Internet Gateway: %v", err)
	}

	if err := a.createRouteTable(cache); err != nil {
		a.updateDegradedCondition(*a.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error creating route table")
		return fmt.Errorf("error creating route table: %v", err)
	}

	if err := a.createSecurityGroup(cache); err != nil {
		a.updateDegradedCondition(*a.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error creating security group")
		return fmt.Errorf("error creating security group: %v", err)
	}

	if err := a.createEC2Instance(cache); err != nil {
		a.updateDegradedCondition(*a.Environment.DeepCopy(), cache, "v1alpha1.Creating", "Error creating EC2 instance")
		return fmt.Errorf("error creating EC2 instance: %v", err)
	}

	// Save objects ID's into a cache file
	if err := a.updateAvailableCondition(*a.Environment, cache); err != nil {
		return fmt.Errorf("error creating cache file: %v", err)
	}
	return nil
}

// createVPC creates a VPC with CIDR
func (a *Client) createVPC(cache *AWS) error {
	a.log.Wg.Add(1)
	go a.log.Loading("Creating VPC")

	vpcInput := &ec2.CreateVpcInput{
		CidrBlock:                   aws.String("10.0.0.0/16"),
		AmazonProvidedIpv6CidrBlock: &no,
		InstanceTenancy:             types.TenancyDefault,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags:         a.Tags,
			},
		},
	}

	vpcOutput, err := a.ec2.CreateVpc(context.TODO(), vpcInput)
	if err != nil {
		a.fail()
		return fmt.Errorf("error creating VPC: %v", err)
	}
	cache.Vpcid = *vpcOutput.Vpc.VpcId

	modVcp := &ec2.ModifyVpcAttributeInput{
		VpcId:              vpcOutput.Vpc.VpcId,
		EnableDnsHostnames: &types.AttributeBooleanValue{Value: &yes},
	}

	_, err = a.ec2.ModifyVpcAttribute(context.Background(), modVcp)
	if err != nil {
		a.fail()
		return fmt.Errorf("error modifying VPC attributes: %v", err)
	}
	a.done()

	return nil
}

// createSubnet creates a subnet for the VPC
func (a *Client) createSubnet(cache *AWS) error {
	a.log.Wg.Add(1)
	go a.log.Loading("Creating subnet")

	subnetInput := &ec2.CreateSubnetInput{
		VpcId:     aws.String(cache.Vpcid),
		CidrBlock: aws.String("10.0.0.0/24"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSubnet,
				Tags:         a.Tags,
			},
		},
	}
	subnetOutput, err := a.ec2.CreateSubnet(context.TODO(), subnetInput)
	if err != nil {
		a.fail()
		return fmt.Errorf("error creating subnet: %v", err)
	}
	cache.Subnetid = *subnetOutput.Subnet.SubnetId

	a.done()
	return nil
}

// createInternetGateway creates an Internet Gateway and attaches it to the VPC
func (a *Client) createInternetGateway(cache *AWS) error {
	a.log.Wg.Add(1)
	go a.log.Loading("Creating Internet Gateway")

	gwInput := &ec2.CreateInternetGatewayInput{}
	gwOutput, err := a.ec2.CreateInternetGateway(context.TODO(), gwInput)
	if err != nil {
		a.fail()
		return fmt.Errorf("error creating Internet Gateway: %v", err)
	}
	cache.InternetGwid = *gwOutput.InternetGateway.InternetGatewayId

	// Attach Internet Gateway to the VPC
	attachInput := &ec2.AttachInternetGatewayInput{
		VpcId:             aws.String(cache.Vpcid),
		InternetGatewayId: gwOutput.InternetGateway.InternetGatewayId,
	}
	_, err = a.ec2.AttachInternetGateway(context.TODO(), attachInput)
	if err != nil {
		a.fail()
		return fmt.Errorf("error attaching Internet Gateway: %v", err)
	}
	if len(gwOutput.InternetGateway.Attachments) > 0 {
		cache.InternetGatewayAttachment = *gwOutput.InternetGateway.Attachments[0].VpcId
	}

	a.done()
	return nil
}

// createRouteTable creates a route table and associates it with the subnet
func (a *Client) createRouteTable(cache *AWS) error {
	a.log.Wg.Add(1)
	go a.log.Loading("Creating route table")

	rtInput := &ec2.CreateRouteTableInput{
		VpcId: aws.String(cache.Vpcid),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeRouteTable,
				Tags:         a.Tags,
			},
		},
	}
	rtOutput, err := a.ec2.CreateRouteTable(context.TODO(), rtInput)
	if err != nil {
		a.fail()
		return fmt.Errorf("error creating route table: %v", err)
	}
	cache.RouteTable = *rtOutput.RouteTable.RouteTableId

	// Associate the route table with the subnet
	assocInput := &ec2.AssociateRouteTableInput{
		RouteTableId: rtOutput.RouteTable.RouteTableId,
		SubnetId:     aws.String(cache.Subnetid),
	}
	if _, err = a.ec2.AssociateRouteTable(context.Background(), assocInput); err != nil {
		a.fail()
		return fmt.Errorf("error associating route table: %v", err)
	}

	routeInput := &ec2.CreateRouteInput{
		RouteTableId:         rtOutput.RouteTable.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(cache.InternetGwid),
	}
	if _, err = a.ec2.CreateRoute(context.TODO(), routeInput); err != nil {
		return fmt.Errorf("error creating route: %v", err)
	}

	a.done()
	return nil
}

// createSecurityGroup creates a security group to allow external communication
// with K8S control plane and SSH
func (a *Client) createSecurityGroup(cache *AWS) error {
	a.log.Wg.Add(1)
	go a.log.Loading("Creating security group")

	sgInput := &ec2.CreateSecurityGroupInput{
		GroupName:   &a.ObjectMeta.Name,
		Description: &description,
		VpcId:       aws.String(cache.Vpcid),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags:         a.Tags,
			},
		},
	}
	sgOutput, err := a.ec2.CreateSecurityGroup(context.TODO(), sgInput)
	if err != nil {
		a.fail()
		return fmt.Errorf("error creating security group: %v", err)
	}
	cache.SecurityGroupid = *sgOutput.GroupId

	// Enter the Ingress rules for the security group
	ipRanges := []types.IpRange{}
	for _, ip := range a.Spec.IngresIpRanges {
		ipRanges = append(ipRanges, types.IpRange{
			CidrIp: &ip,
		})
	}

	irInput := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: sgOutput.GroupId,
		IpPermissions: []types.IpPermission{
			{
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpProtocol: &tcp,
				IpRanges:   ipRanges,
			},
			{
				FromPort:   &k8s443,
				ToPort:     &k8s443,
				IpProtocol: &tcp,
				IpRanges:   ipRanges,
			},
			{
				FromPort:   &k8s6443,
				ToPort:     &k8s6443,
				IpProtocol: &tcp,
				IpRanges:   ipRanges,
			},
		},
	}

	if _, err = a.ec2.AuthorizeSecurityGroupIngress(context.TODO(), irInput); err != nil {
		a.fail()
		return fmt.Errorf("error authorizing security group ingress: %v", err)
	}

	a.done()
	return nil
}

// createEC2Instance creates an EC2 instance with proper Network configuration
func (a *Client) createEC2Instance(cache *AWS) error {
	a.log.Wg.Add(1)
	go a.log.Loading("Creating EC2 instance")

	instanceIn := &ec2.RunInstancesInput{
		ImageId:      a.Spec.Image.ImageId,
		InstanceType: types.InstanceType(a.Spec.Instance.Type),
		MaxCount:     &minMaxCount,
		MinCount:     &minMaxCount,
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &types.EbsBlockDevice{
					VolumeSize: &storageSizeGB,
					VolumeType: types.VolumeTypeGp2,
				},
			},
		},
		NetworkInterfaces: []types.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: &yes,
				DeleteOnTermination:      &yes,
				DeviceIndex:              aws.Int32(0),
				Groups: []string{
					cache.SecurityGroupid,
				},
				SubnetId: aws.String(cache.Subnetid),
			},
		},
		KeyName: aws.String(a.Spec.Auth.KeyName),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags:         a.Tags,
			},
		},
	}
	instanceOut, err := a.ec2.RunInstances(context.Background(), instanceIn)
	if err != nil {
		a.fail()
		return fmt.Errorf("error creating instance: %v", err)
	}
	cache.Instanceid = *instanceOut.Instances[0].InstanceId

	waiterOptions := []func(*ec2.InstanceRunningWaiterOptions){
		func(o *ec2.InstanceRunningWaiterOptions) {
			o.MaxDelay = 1 * time.Minute
			o.MinDelay = 5 * time.Second
		},
	}
	waiter := ec2.NewInstanceRunningWaiter(a.ec2, waiterOptions...)

	if err = waiter.Wait(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{*instanceOut.Instances[0].InstanceId},
	}, 5*time.Minute, waiterOptions...); err != nil {
		a.fail()
		return fmt.Errorf("error waiting for instance to be in running state: %v", err)
	}

	// Describe instance now that is running
	instanceRunning, err := a.ec2.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{*instanceOut.Instances[0].InstanceId},
	})
	if err != nil {
		a.fail()
		return fmt.Errorf("error describing instances: %v", err)
	}
	cache.PublicDnsName = *instanceRunning.Reservations[0].Instances[0].PublicDnsName

	a.done()
	return nil
}
