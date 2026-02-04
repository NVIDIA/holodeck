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

	"github.com/NVIDIA/holodeck/pkg/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Create creates an EC2 instance with proper Network configuration
// VPC, Subnet, Internet Gateway, Route Table, Security Group
// If the environment specifies a cluster configuration, it delegates to CreateCluster()
func (p *Provider) Create() error {
	// Check if this is a multinode cluster deployment
	if p.IsMultinode() {
		return p.CreateCluster()
	}

	// Single-node deployment
	cache := new(AWS)

	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Creating AWS resources"); err != nil {
		p.log.Warning("Failed to update progressing condition: %v", err)
	}

	if err := p.createVPC(cache); err != nil {
		if updateErr := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Error creating VPC"); updateErr != nil {
			p.log.Warning("Failed to update degraded condition: %v", updateErr)
		}
		return fmt.Errorf("error creating VPC: %w", err)
	}
	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "VPC created"); err != nil {
		p.log.Warning("Failed to update progressing condition: %v", err)
	}

	if err := p.createSubnet(cache); err != nil {
		if updateErr := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Error creating subnet"); updateErr != nil {
			p.log.Warning("Failed to update degraded condition: %v", updateErr)
		}
		return fmt.Errorf("error creating subnet: %w", err)
	}
	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Subnet created"); err != nil {
		p.log.Warning("Failed to update progressing condition: %v", err)
	}

	if err := p.createInternetGateway(cache); err != nil {
		if updateErr := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Error creating Internet Gateway"); updateErr != nil {
			p.log.Warning("Failed to update degraded condition: %v", updateErr)
		}
		return fmt.Errorf("error creating Internet Gateway: %w", err)
	}
	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Internet Gateway created"); err != nil {
		p.log.Warning("Failed to update progressing condition: %v", err)
	}

	if err := p.createRouteTable(cache); err != nil {
		if updateErr := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Error creating route table"); updateErr != nil {
			p.log.Warning("Failed to update degraded condition: %v", updateErr)
		}
		return fmt.Errorf("error creating route table: %w", err)
	}
	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Route Table created"); err != nil {
		p.log.Warning("Failed to update progressing condition: %v", err)
	}

	if err := p.createSecurityGroup(cache); err != nil {
		if updateErr := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Error creating security group"); updateErr != nil {
			p.log.Warning("Failed to update degraded condition: %v", updateErr)
		}
		return fmt.Errorf("error creating security group: %w", err)
	}
	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Security Group created"); err != nil {
		p.log.Warning("Failed to update progressing condition: %v", err)
	}

	if err := p.createEC2Instance(cache); err != nil {
		if updateErr := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Creating", "Error creating EC2 instance"); updateErr != nil {
			p.log.Warning("Failed to update degraded condition: %v", updateErr)
		}
		return fmt.Errorf("error creating EC2 instance: %w", err)
	}

	// Save objects ID's into a cache file
	if err := p.updateAvailableCondition(*p.Environment, cache); err != nil {
		return fmt.Errorf("error creating cache file: %w", err)
	}
	return nil
}

// createVPC creates a VPC with CIDR
func (p *Provider) createVPC(cache *AWS) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating VPC")

	vpcInput := &ec2.CreateVpcInput{
		CidrBlock:                   aws.String("10.0.0.0/16"),
		AmazonProvidedIpv6CidrBlock: &no,
		InstanceTenancy:             types.TenancyDefault,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags:         p.Tags,
			},
		},
	}

	vpcOutput, err := p.ec2.CreateVpc(context.TODO(), vpcInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating VPC: %w", err)
	}
	cache.Vpcid = *vpcOutput.Vpc.VpcId

	modVcp := &ec2.ModifyVpcAttributeInput{
		VpcId:              vpcOutput.Vpc.VpcId,
		EnableDnsHostnames: &types.AttributeBooleanValue{Value: &yes},
	}

	_, err = p.ec2.ModifyVpcAttribute(context.Background(), modVcp)
	if err != nil {
		p.fail()
		return fmt.Errorf("error modifying VPC attributes: %w", err)
	}
	p.done()

	return nil
}

// createSubnet creates a subnet for the VPC
func (p *Provider) createSubnet(cache *AWS) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating subnet")

	subnetInput := &ec2.CreateSubnetInput{
		VpcId:     aws.String(cache.Vpcid),
		CidrBlock: aws.String("10.0.0.0/24"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSubnet,
				Tags:         p.Tags,
			},
		},
	}
	subnetOutput, err := p.ec2.CreateSubnet(context.TODO(), subnetInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating subnet: %w", err)
	}
	cache.Subnetid = *subnetOutput.Subnet.SubnetId

	p.done()
	return nil
}

// createInternetGateway creates an Internet Gateway and attaches it to the VPC
func (p *Provider) createInternetGateway(cache *AWS) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating Internet Gateway")

	gwInput := &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInternetGateway,
				Tags:         p.Tags,
			},
		},
	}
	gwOutput, err := p.ec2.CreateInternetGateway(context.TODO(), gwInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating Internet Gateway: %w", err)
	}
	cache.InternetGwid = *gwOutput.InternetGateway.InternetGatewayId

	// Attach Internet Gateway to the VPC
	attachInput := &ec2.AttachInternetGatewayInput{
		VpcId:             aws.String(cache.Vpcid),
		InternetGatewayId: gwOutput.InternetGateway.InternetGatewayId,
	}
	_, err = p.ec2.AttachInternetGateway(context.TODO(), attachInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error attaching Internet Gateway: %w", err)
	}
	if len(gwOutput.InternetGateway.Attachments) > 0 {
		cache.InternetGatewayAttachment = *gwOutput.InternetGateway.Attachments[0].VpcId
	}

	p.done()
	return nil
}

// createRouteTable creates a route table and associates it with the subnet
func (p *Provider) createRouteTable(cache *AWS) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating route table")

	rtInput := &ec2.CreateRouteTableInput{
		VpcId: aws.String(cache.Vpcid),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeRouteTable,
				Tags:         p.Tags,
			},
		},
	}
	rtOutput, err := p.ec2.CreateRouteTable(context.TODO(), rtInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating route table: %w", err)
	}
	cache.RouteTable = *rtOutput.RouteTable.RouteTableId

	// Associate the route table with the subnet
	assocInput := &ec2.AssociateRouteTableInput{
		RouteTableId: rtOutput.RouteTable.RouteTableId,
		SubnetId:     aws.String(cache.Subnetid),
	}
	if _, err = p.ec2.AssociateRouteTable(context.Background(), assocInput); err != nil {
		p.fail()
		return fmt.Errorf("error associating route table: %w", err)
	}

	routeInput := &ec2.CreateRouteInput{
		RouteTableId:         rtOutput.RouteTable.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(cache.InternetGwid),
	}
	if _, err = p.ec2.CreateRoute(context.TODO(), routeInput); err != nil {
		return fmt.Errorf("error creating route: %w", err)
	}

	p.done()
	return nil
}

// createSecurityGroup creates a security group to allow external communication
// with K8S control plane and SSH
func (p *Provider) createSecurityGroup(cache *AWS) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating security group")

	sgInput := &ec2.CreateSecurityGroupInput{
		GroupName:   &p.ObjectMeta.Name,
		Description: &description,
		VpcId:       aws.String(cache.Vpcid),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags:         p.Tags,
			},
		},
	}
	sgOutput, err := p.ec2.CreateSecurityGroup(context.TODO(), sgInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating security group: %w", err)
	}
	cache.SecurityGroupid = *sgOutput.GroupId

	// Enter the Ingress rules for the security group
	ipRangeMap := make(map[string]bool)
	ipRanges := []types.IpRange{}

	// First lookup for the IP address of the user
	ip, err := utils.GetIPAddress()
	if err != nil {
		p.fail()
		return fmt.Errorf("error getting IP address: %w", err)
	}

	// Add the auto-detected IP to the map and list
	ipRangeMap[ip] = true
	ipRanges = append(ipRanges, types.IpRange{
		CidrIp: &ip,
	})

	// Then add the IP ranges from the spec, skipping duplicates
	for _, ip := range p.Spec.IngressIpRanges {
		if !ipRangeMap[ip] {
			ipRangeMap[ip] = true
			ipRanges = append(ipRanges, types.IpRange{
				CidrIp: &ip,
			})
		}
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

	if _, err = p.ec2.AuthorizeSecurityGroupIngress(context.TODO(), irInput); err != nil {
		p.fail()
		return fmt.Errorf("error authorizing security group ingress: %w", err)
	}

	p.done()
	return nil
}

// createEC2Instance creates an EC2 instance with proper Network configuration
func (p *Provider) createEC2Instance(cache *AWS) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating EC2 instance")

	// Check if the image is provided, if not get the latest image
	err := p.setAMI()
	if err != nil {
		p.fail()
		return fmt.Errorf("error getting AMI: %w", err)
	}

	// if the root volume size is not set, use the default size
	if p.Spec.RootVolumeSizeGB != nil {
		storageSizeGB = *p.Spec.RootVolumeSizeGB
	}

	instanceIn := &ec2.RunInstancesInput{
		ImageId:                           p.Spec.Image.ImageId,
		InstanceType:                      types.InstanceType(p.Spec.Type),
		MaxCount:                          &minMaxCount,
		MinCount:                          &minMaxCount,
		InstanceInitiatedShutdownBehavior: types.ShutdownBehaviorTerminate,
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
		KeyName: aws.String(p.Spec.KeyName),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags:         p.Tags,
			},
		},
	}
	instanceOut, err := p.ec2.RunInstances(context.Background(), instanceIn)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating instance: %w", err)
	}
	cache.Instanceid = *instanceOut.Instances[0].InstanceId

	waiterOptions := []func(*ec2.InstanceRunningWaiterOptions){
		func(o *ec2.InstanceRunningWaiterOptions) {
			o.MaxDelay = 1 * time.Minute
			o.MinDelay = 5 * time.Second
		},
	}
	waiter := ec2.NewInstanceRunningWaiter(p.ec2, waiterOptions...)

	if err = waiter.Wait(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{*instanceOut.Instances[0].InstanceId},
	}, 5*time.Minute, waiterOptions...); err != nil {
		p.fail()
		return fmt.Errorf("error waiting for instance to be in running state: %w", err)
	}

	// Describe instance now that is running
	instanceRunning, err := p.ec2.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{*instanceOut.Instances[0].InstanceId},
	})
	if err != nil {
		p.fail()
		return fmt.Errorf("error describing instances: %w", err)
	}
	cache.PublicDnsName = *instanceRunning.Reservations[0].Instances[0].PublicDnsName

	// tag network interface
	instance := instanceOut.Instances[0]
	networkInterfaceId := *instance.NetworkInterfaces[0].NetworkInterfaceId
	_, err = p.ec2.CreateTags(context.TODO(), &ec2.CreateTagsInput{
		Resources: []string{networkInterfaceId},
		Tags:      p.Tags,
	})
	if err != nil {
		p.fail()
		return fmt.Errorf("fail to tag network to instance: %w", err)
	}

	// Disable Source/Destination Check for Calico networking
	// This is required for Kubernetes CNI plugins (Calico, Flannel, etc.) to work correctly
	// See: https://github.com/NVIDIA/holodeck/issues/586
	_, err = p.ec2.ModifyNetworkInterfaceAttribute(context.TODO(),
		&ec2.ModifyNetworkInterfaceAttributeInput{
			NetworkInterfaceId: aws.String(networkInterfaceId),
			SourceDestCheck: &types.AttributeBooleanValue{
				Value: aws.Bool(false),
			},
		})
	if err != nil {
		p.fail()
		return fmt.Errorf("error disabling source/dest check: %w", err)
	}

	p.done()
	return nil
}
