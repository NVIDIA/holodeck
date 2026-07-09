/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package awsfake

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	internalaws "github.com/NVIDIA/holodeck/internal/aws"
)

// FakeEC2 is a stateful in-memory implementation of internalaws.EC2Client.
type FakeEC2 struct {
	store *Store
}

var _ internalaws.EC2Client = (*FakeEC2)(nil)

// ---- VPC ----

func (f *FakeEC2) CreateVpc(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateVpc", params)
	if err := f.store.failure("CreateVpc"); err != nil {
		return nil, err
	}
	id := f.store.nextID("vpc")
	v := ec2types.Vpc{
		VpcId:     aws.String(id),
		CidrBlock: params.CidrBlock,
		State:     ec2types.VpcStateAvailable,
		Tags:      tagsFromSpecs(params.TagSpecifications),
	}
	f.store.Vpcs[id] = &v
	return &ec2.CreateVpcOutput{Vpc: &v}, nil
}

func (f *FakeEC2) ModifyVpcAttribute(ctx context.Context, params *ec2.ModifyVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("ModifyVpcAttribute", params)
	if err := f.store.failure("ModifyVpcAttribute"); err != nil {
		return nil, err
	}
	return &ec2.ModifyVpcAttributeOutput{}, nil
}

func (f *FakeEC2) DeleteVpc(ctx context.Context, params *ec2.DeleteVpcInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteVpc", params)
	if err := f.store.failure("DeleteVpc"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.VpcId)
	if _, ok := f.store.Vpcs[id]; !ok {
		return nil, notFound("InvalidVpcID.NotFound", id)
	}
	delete(f.store.Vpcs, id)
	return &ec2.DeleteVpcOutput{}, nil
}

func (f *FakeEC2) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeVpcs", params)
	if err := f.store.failure("DescribeVpcs"); err != nil {
		return nil, err
	}
	var out []ec2types.Vpc
	if len(params.VpcIds) > 0 {
		for _, id := range params.VpcIds {
			v, ok := f.store.Vpcs[id]
			if !ok {
				return nil, notFound("InvalidVpcID.NotFound", id)
			}
			out = append(out, *v)
		}
	} else {
		for _, v := range f.store.Vpcs {
			out = append(out, *v)
		}
	}
	return &ec2.DescribeVpcsOutput{Vpcs: out}, nil
}

// ---- Subnet ----

func (f *FakeEC2) CreateSubnet(ctx context.Context, params *ec2.CreateSubnetInput, optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateSubnet", params)
	if err := f.store.failure("CreateSubnet"); err != nil {
		return nil, err
	}
	id := f.store.nextID("subnet")
	sn := ec2types.Subnet{
		SubnetId:  aws.String(id),
		VpcId:     params.VpcId,
		CidrBlock: params.CidrBlock,
		State:     ec2types.SubnetStateAvailable,
		Tags:      tagsFromSpecs(params.TagSpecifications),
	}
	f.store.Subnets[id] = &sn
	return &ec2.CreateSubnetOutput{Subnet: &sn}, nil
}

func (f *FakeEC2) DeleteSubnet(ctx context.Context, params *ec2.DeleteSubnetInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteSubnet", params)
	if err := f.store.failure("DeleteSubnet"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.SubnetId)
	if _, ok := f.store.Subnets[id]; !ok {
		return nil, notFound("InvalidSubnetID.NotFound", id)
	}
	delete(f.store.Subnets, id)
	return &ec2.DeleteSubnetOutput{}, nil
}

func (f *FakeEC2) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeSubnets", params)
	if err := f.store.failure("DescribeSubnets"); err != nil {
		return nil, err
	}
	var out []ec2types.Subnet
	switch {
	case len(params.SubnetIds) > 0:
		for _, id := range params.SubnetIds {
			sn, ok := f.store.Subnets[id]
			if !ok {
				return nil, notFound("InvalidSubnetID.NotFound", id)
			}
			out = append(out, *sn)
		}
	case len(params.Filters) > 0:
		vpcID := filterValue(params.Filters, "vpc-id")
		for _, sn := range f.store.Subnets {
			if vpcID == "" || aws.ToString(sn.VpcId) == vpcID {
				out = append(out, *sn)
			}
		}
	default:
		for _, sn := range f.store.Subnets {
			out = append(out, *sn)
		}
	}
	return &ec2.DescribeSubnetsOutput{Subnets: out}, nil
}

// ---- Internet Gateway ----

func (f *FakeEC2) CreateInternetGateway(ctx context.Context, params *ec2.CreateInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateInternetGateway", params)
	if err := f.store.failure("CreateInternetGateway"); err != nil {
		return nil, err
	}
	id := f.store.nextID("igw")
	igw := ec2types.InternetGateway{
		InternetGatewayId: aws.String(id),
		Tags:              tagsFromSpecs(params.TagSpecifications),
	}
	f.store.InternetGateways[id] = &igw
	return &ec2.CreateInternetGatewayOutput{InternetGateway: &igw}, nil
}

func (f *FakeEC2) AttachInternetGateway(ctx context.Context, params *ec2.AttachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("AttachInternetGateway", params)
	if err := f.store.failure("AttachInternetGateway"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.InternetGatewayId)
	if igw, ok := f.store.InternetGateways[id]; ok {
		igw.Attachments = append(igw.Attachments, ec2types.InternetGatewayAttachment{
			State: ec2types.AttachmentStatusAttached,
			VpcId: params.VpcId,
		})
	}
	return &ec2.AttachInternetGatewayOutput{}, nil
}

func (f *FakeEC2) DetachInternetGateway(ctx context.Context, params *ec2.DetachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DetachInternetGateway", params)
	if err := f.store.failure("DetachInternetGateway"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.InternetGatewayId)
	igw, ok := f.store.InternetGateways[id]
	if !ok {
		return nil, notFound("InvalidInternetGatewayID.NotFound", id)
	}
	if len(igw.Attachments) == 0 {
		return nil, notFound("Gateway.NotAttached", id)
	}
	igw.Attachments = nil
	return &ec2.DetachInternetGatewayOutput{}, nil
}

func (f *FakeEC2) DeleteInternetGateway(ctx context.Context, params *ec2.DeleteInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteInternetGateway", params)
	if err := f.store.failure("DeleteInternetGateway"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.InternetGatewayId)
	if _, ok := f.store.InternetGateways[id]; !ok {
		return nil, notFound("InvalidInternetGatewayID.NotFound", id)
	}
	delete(f.store.InternetGateways, id)
	return &ec2.DeleteInternetGatewayOutput{}, nil
}

func (f *FakeEC2) DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeInternetGateways", params)
	if err := f.store.failure("DescribeInternetGateways"); err != nil {
		return nil, err
	}
	var out []ec2types.InternetGateway
	if len(params.InternetGatewayIds) > 0 {
		for _, id := range params.InternetGatewayIds {
			igw, ok := f.store.InternetGateways[id]
			if !ok {
				return nil, notFound("InvalidInternetGatewayID.NotFound", id)
			}
			out = append(out, *igw)
		}
	} else {
		for _, igw := range f.store.InternetGateways {
			out = append(out, *igw)
		}
	}
	return &ec2.DescribeInternetGatewaysOutput{InternetGateways: out}, nil
}

// ---- Route Table ----

func (f *FakeEC2) CreateRouteTable(ctx context.Context, params *ec2.CreateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateRouteTable", params)
	if err := f.store.failure("CreateRouteTable"); err != nil {
		return nil, err
	}
	id := f.store.nextID("rtb")
	rt := ec2types.RouteTable{
		RouteTableId: aws.String(id),
		VpcId:        params.VpcId,
		Tags:         tagsFromSpecs(params.TagSpecifications),
	}
	f.store.RouteTables[id] = &rt
	return &ec2.CreateRouteTableOutput{RouteTable: &rt}, nil
}

func (f *FakeEC2) AssociateRouteTable(ctx context.Context, params *ec2.AssociateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("AssociateRouteTable", params)
	if err := f.store.failure("AssociateRouteTable"); err != nil {
		return nil, err
	}
	assocID := f.store.nextID("rtbassoc")
	if rt, ok := f.store.RouteTables[aws.ToString(params.RouteTableId)]; ok {
		rt.Associations = append(rt.Associations, ec2types.RouteTableAssociation{
			RouteTableAssociationId: aws.String(assocID),
			RouteTableId:            params.RouteTableId,
			SubnetId:                params.SubnetId,
			Main:                    aws.Bool(false),
		})
	}
	return &ec2.AssociateRouteTableOutput{AssociationId: aws.String(assocID)}, nil
}

func (f *FakeEC2) CreateRoute(ctx context.Context, params *ec2.CreateRouteInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateRoute", params)
	if err := f.store.failure("CreateRoute"); err != nil {
		return nil, err
	}
	return &ec2.CreateRouteOutput{Return: aws.Bool(true)}, nil
}

func (f *FakeEC2) DeleteRouteTable(ctx context.Context, params *ec2.DeleteRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteRouteTable", params)
	if err := f.store.failure("DeleteRouteTable"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.RouteTableId)
	if _, ok := f.store.RouteTables[id]; !ok {
		return nil, notFound("InvalidRouteTableID.NotFound", id)
	}
	delete(f.store.RouteTables, id)
	return &ec2.DeleteRouteTableOutput{}, nil
}

func (f *FakeEC2) DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeRouteTables", params)
	if err := f.store.failure("DescribeRouteTables"); err != nil {
		return nil, err
	}
	var out []ec2types.RouteTable
	if len(params.RouteTableIds) > 0 {
		for _, id := range params.RouteTableIds {
			rt, ok := f.store.RouteTables[id]
			if !ok {
				return nil, notFound("InvalidRouteTableID.NotFound", id)
			}
			out = append(out, *rt)
		}
	} else {
		for _, rt := range f.store.RouteTables {
			out = append(out, *rt)
		}
	}
	return &ec2.DescribeRouteTablesOutput{RouteTables: out}, nil
}

func (f *FakeEC2) ReplaceRouteTableAssociation(ctx context.Context, params *ec2.ReplaceRouteTableAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceRouteTableAssociationOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("ReplaceRouteTableAssociation", params)
	if err := f.store.failure("ReplaceRouteTableAssociation"); err != nil {
		return nil, err
	}
	return &ec2.ReplaceRouteTableAssociationOutput{NewAssociationId: aws.String(f.store.nextID("rtbassoc"))}, nil
}

// ---- Security Group ----

func (f *FakeEC2) CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateSecurityGroup", params)
	if err := f.store.failure("CreateSecurityGroup"); err != nil {
		return nil, err
	}
	id := f.store.nextID("sg")
	sg := ec2types.SecurityGroup{
		GroupId:     aws.String(id),
		GroupName:   params.GroupName,
		Description: params.Description,
		VpcId:       params.VpcId,
		Tags:        tagsFromSpecs(params.TagSpecifications),
	}
	f.store.SecurityGroups[id] = &sg
	return &ec2.CreateSecurityGroupOutput{GroupId: aws.String(id)}, nil
}

func (f *FakeEC2) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("AuthorizeSecurityGroupIngress", params)
	if err := f.store.failure("AuthorizeSecurityGroupIngress"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.GroupId)
	if sg, ok := f.store.SecurityGroups[id]; ok {
		sg.IpPermissions = append(sg.IpPermissions, params.IpPermissions...)
	}
	return &ec2.AuthorizeSecurityGroupIngressOutput{Return: aws.Bool(true)}, nil
}

func (f *FakeEC2) DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteSecurityGroup", params)
	if err := f.store.failure("DeleteSecurityGroup"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.GroupId)
	if _, ok := f.store.SecurityGroups[id]; !ok {
		return nil, notFound("InvalidGroup.NotFound", id)
	}
	delete(f.store.SecurityGroups, id)
	return &ec2.DeleteSecurityGroupOutput{}, nil
}

func (f *FakeEC2) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeSecurityGroups", params)
	if err := f.store.failure("DescribeSecurityGroups"); err != nil {
		return nil, err
	}
	var out []ec2types.SecurityGroup
	switch {
	case len(params.GroupIds) > 0:
		for _, id := range params.GroupIds {
			sg, ok := f.store.SecurityGroups[id]
			if !ok {
				return nil, notFound("InvalidGroup.NotFound", id)
			}
			out = append(out, *sg)
		}
	case len(params.Filters) > 0:
		vpcID := filterValue(params.Filters, "vpc-id")
		for _, sg := range f.store.SecurityGroups {
			if vpcID == "" || aws.ToString(sg.VpcId) == vpcID {
				out = append(out, *sg)
			}
		}
	default:
		for _, sg := range f.store.SecurityGroups {
			out = append(out, *sg)
		}
	}
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: out}, nil
}

func (f *FakeEC2) RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("RevokeSecurityGroupIngress", params)
	if err := f.store.failure("RevokeSecurityGroupIngress"); err != nil {
		return nil, err
	}
	if sg, ok := f.store.SecurityGroups[aws.ToString(params.GroupId)]; ok {
		sg.IpPermissions = nil
	}
	return &ec2.RevokeSecurityGroupIngressOutput{Return: aws.Bool(true)}, nil
}

func (f *FakeEC2) RevokeSecurityGroupEgress(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("RevokeSecurityGroupEgress", params)
	if err := f.store.failure("RevokeSecurityGroupEgress"); err != nil {
		return nil, err
	}
	return &ec2.RevokeSecurityGroupEgressOutput{Return: aws.Bool(true)}, nil
}

// ---- Instances ----

func (f *FakeEC2) RunInstances(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("RunInstances", params)
	if err := f.store.failure("RunInstances"); err != nil {
		return nil, err
	}

	count := int32(1)
	if params.MinCount != nil && *params.MinCount > 0 {
		count = *params.MinCount
	}

	subnetID := ""
	if len(params.NetworkInterfaces) > 0 {
		subnetID = aws.ToString(params.NetworkInterfaces[0].SubnetId)
	} else if params.SubnetId != nil {
		subnetID = aws.ToString(params.SubnetId)
	}
	vpcID := ""
	if sn, ok := f.store.Subnets[subnetID]; ok {
		vpcID = aws.ToString(sn.VpcId)
	}
	tags := tagsFromSpecs(params.TagSpecifications)

	var instances []ec2types.Instance
	for i := int32(0); i < count; i++ {
		instID := f.store.nextID("i")
		eniID := f.store.nextID("eni")

		f.store.NetworkInterfaces[eniID] = &ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(eniID),
			VpcId:              aws.String(vpcID),
			SubnetId:           aws.String(subnetID),
			Status:             ec2types.NetworkInterfaceStatusInUse,
		}

		inst := ec2types.Instance{
			InstanceId:       aws.String(instID),
			ImageId:          params.ImageId,
			InstanceType:     params.InstanceType,
			State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning, Code: aws.Int32(16)},
			PublicDnsName:    aws.String(fmt.Sprintf("ec2-%s.compute.amazonaws.com", instID)),
			PublicIpAddress:  aws.String(fmt.Sprintf("203.0.113.%d", f.store.counter%254+1)),
			PrivateIpAddress: aws.String(fmt.Sprintf("10.0.0.%d", f.store.counter%254+1)),
			SubnetId:         aws.String(subnetID),
			VpcId:            aws.String(vpcID),
			NetworkInterfaces: []ec2types.InstanceNetworkInterface{
				{NetworkInterfaceId: aws.String(eniID), SubnetId: aws.String(subnetID)},
			},
			Tags: tags,
		}
		f.store.Instances[instID] = &inst
		instances = append(instances, inst)
	}
	return &ec2.RunInstancesOutput{Instances: instances}, nil
}

func (f *FakeEC2) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("TerminateInstances", params)
	if err := f.store.failure("TerminateInstances"); err != nil {
		return nil, err
	}
	var changes []ec2types.InstanceStateChange
	for _, id := range params.InstanceIds {
		inst, ok := f.store.Instances[id]
		if !ok {
			return nil, notFound("InvalidInstanceID.NotFound", id)
		}
		// Detach and remove the instance's ENIs so waitForENIsDrained's vpc-id
		// query reports zero in-use interfaces on the first poll.
		for _, ni := range inst.NetworkInterfaces {
			delete(f.store.NetworkInterfaces, aws.ToString(ni.NetworkInterfaceId))
		}
		// Keep the instance in the map in terminated state so the
		// terminated-waiter / isInstanceTerminated observe it.
		inst.State = &ec2types.InstanceState{Name: ec2types.InstanceStateNameTerminated, Code: aws.Int32(48)}
		changes = append(changes, ec2types.InstanceStateChange{
			InstanceId:    aws.String(id),
			CurrentState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameTerminated, Code: aws.Int32(48)},
			PreviousState: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning, Code: aws.Int32(16)},
		})
	}
	return &ec2.TerminateInstancesOutput{TerminatingInstances: changes}, nil
}

func (f *FakeEC2) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeInstances", params)
	if err := f.store.failure("DescribeInstances"); err != nil {
		return nil, err
	}
	var insts []ec2types.Instance
	if len(params.InstanceIds) > 0 {
		for _, id := range params.InstanceIds {
			inst, ok := f.store.Instances[id]
			if !ok {
				return nil, notFound("InvalidInstanceID.NotFound", id)
			}
			insts = append(insts, *inst)
		}
	} else {
		for _, inst := range f.store.Instances {
			insts = append(insts, *inst)
		}
	}
	// Group into a single reservation (matches the provider's single-page reads).
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{ReservationId: aws.String("r-fake"), Instances: insts},
		},
	}, nil
}

func (f *FakeEC2) DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeInstanceTypes", params)
	if err := f.store.failure("DescribeInstanceTypes"); err != nil {
		return nil, err
	}
	var infos []ec2types.InstanceTypeInfo
	if len(params.InstanceTypes) > 0 {
		// Permissive echo by default: every requested type "exists". Types
		// marked absent are omitted (region-unavailable); types with seeded
		// explicit architectures report those instead of the prefix heuristic.
		for _, it := range params.InstanceTypes {
			name := string(it)
			if f.store.absentInstanceTypes[name] {
				continue
			}
			if archs, ok := f.store.instanceTypeArchs[name]; ok {
				infos = append(infos, ec2types.InstanceTypeInfo{
					InstanceType:  ec2types.InstanceType(name),
					ProcessorInfo: &ec2types.ProcessorInfo{SupportedArchitectures: archs},
				})
				continue
			}
			infos = append(infos, instanceTypeInfo(name))
		}
	} else {
		// Full catalog (checkInstanceTypes lists all types with no filter).
		for name := range f.store.InstanceTypes {
			infos = append(infos, instanceTypeInfo(name))
		}
	}
	return &ec2.DescribeInstanceTypesOutput{InstanceTypes: infos, NextToken: nil}, nil
}

func instanceTypeInfo(name string) ec2types.InstanceTypeInfo {
	return ec2types.InstanceTypeInfo{
		InstanceType:  ec2types.InstanceType(name),
		ProcessorInfo: &ec2types.ProcessorInfo{SupportedArchitectures: archsFor(name)},
	}
}

// ---- Images ----

func (f *FakeEC2) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeImages", params)
	if err := f.store.failure("DescribeImages"); err != nil {
		return nil, err
	}
	var out []ec2types.Image
	if len(params.ImageIds) > 0 {
		want := map[string]bool{}
		for _, id := range params.ImageIds {
			want[id] = true
		}
		for _, img := range f.store.Images {
			if want[aws.ToString(img.ImageId)] {
				out = append(out, img)
			}
		}
	} else {
		// Permissive: return the full seeded catalog for any filter set.
		out = append(out, f.store.Images...)
	}
	return &ec2.DescribeImagesOutput{Images: out, NextToken: nil}, nil
}

// ---- Network Interfaces ----

func (f *FakeEC2) DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeNetworkInterfaces", params)
	if err := f.store.failure("DescribeNetworkInterfaces"); err != nil {
		return nil, err
	}
	var out []ec2types.NetworkInterface
	switch {
	case len(params.NetworkInterfaceIds) > 0:
		for _, id := range params.NetworkInterfaceIds {
			if ni, ok := f.store.NetworkInterfaces[id]; ok {
				out = append(out, *ni)
			}
		}
	case len(params.Filters) > 0:
		vpcID := filterValue(params.Filters, "vpc-id")
		for id, ni := range f.store.NetworkInterfaces {
			if vpcID == "" || aws.ToString(ni.VpcId) == vpcID {
				out = append(out, *ni)
				f.advanceENIDrain(id)
			}
		}
	default:
		for _, ni := range f.store.NetworkInterfaces {
			out = append(out, *ni)
		}
	}
	return &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: out}, nil
}

// advanceENIDrain decrements a scripted-draining interface's remaining blocking
// observations, removing it once exhausted so a later poll reports it drained.
// Interfaces without a script are untouched. Callers must hold mu.
func (f *FakeEC2) advanceENIDrain(id string) {
	rem, ok := f.store.eniDraining[id]
	if !ok {
		return
	}
	rem--
	if rem <= 0 {
		delete(f.store.NetworkInterfaces, id)
		delete(f.store.eniDraining, id)
		return
	}
	f.store.eniDraining[id] = rem
}

func (f *FakeEC2) ModifyNetworkInterfaceAttribute(ctx context.Context, params *ec2.ModifyNetworkInterfaceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("ModifyNetworkInterfaceAttribute", params)
	if err := f.store.failure("ModifyNetworkInterfaceAttribute"); err != nil {
		return nil, err
	}
	return &ec2.ModifyNetworkInterfaceAttributeOutput{}, nil
}

// ---- Tags ----

func (f *FakeEC2) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateTags", params)
	if err := f.store.failure("CreateTags"); err != nil {
		return nil, err
	}
	for _, id := range params.Resources {
		if inst, ok := f.store.Instances[id]; ok {
			inst.Tags = append(inst.Tags, params.Tags...)
			continue
		}
		if ni, ok := f.store.NetworkInterfaces[id]; ok {
			ni.TagSet = append(ni.TagSet, params.Tags...)
		}
	}
	return &ec2.CreateTagsOutput{}, nil
}

func (f *FakeEC2) DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeTags", params)
	if err := f.store.failure("DescribeTags"); err != nil {
		return nil, err
	}
	var descs []ec2types.TagDescription
	for id, inst := range f.store.Instances {
		for _, tag := range inst.Tags {
			descs = append(descs, ec2types.TagDescription{
				ResourceId: aws.String(id),
				Key:        tag.Key,
				Value:      tag.Value,
			})
		}
	}
	return &ec2.DescribeTagsOutput{Tags: descs}, nil
}

// ---- Elastic IP ----

func (f *FakeEC2) AllocateAddress(ctx context.Context, params *ec2.AllocateAddressInput, optFns ...func(*ec2.Options)) (*ec2.AllocateAddressOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("AllocateAddress", params)
	if err := f.store.failure("AllocateAddress"); err != nil {
		return nil, err
	}
	id := f.store.nextID("eipalloc")
	ip := fmt.Sprintf("203.0.113.%d", f.store.counter%254+1)
	f.store.Addresses[id] = &ec2types.Address{
		AllocationId: aws.String(id),
		PublicIp:     aws.String(ip),
		Domain:       ec2types.DomainTypeVpc,
	}
	return &ec2.AllocateAddressOutput{AllocationId: aws.String(id), PublicIp: aws.String(ip)}, nil
}

func (f *FakeEC2) ReleaseAddress(ctx context.Context, params *ec2.ReleaseAddressInput, optFns ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("ReleaseAddress", params)
	if err := f.store.failure("ReleaseAddress"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.AllocationId)
	if _, ok := f.store.Addresses[id]; !ok {
		return nil, notFound("InvalidAllocationID.NotFound", id)
	}
	delete(f.store.Addresses, id)
	return &ec2.ReleaseAddressOutput{}, nil
}

// ---- NAT Gateway ----

func (f *FakeEC2) CreateNatGateway(ctx context.Context, params *ec2.CreateNatGatewayInput, optFns ...func(*ec2.Options)) (*ec2.CreateNatGatewayOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateNatGateway", params)
	if err := f.store.failure("CreateNatGateway"); err != nil {
		return nil, err
	}
	id := f.store.nextID("nat")
	// Default: gateways are created available. A one-shot SeedNextNatGatewayState
	// directive scripts a pending phase (or a non-available terminal state) so
	// createNATGateway's poll loop can be exercised.
	state := ec2types.NatGatewayStateAvailable
	if script := f.store.natScript; script != nil {
		if script.pendingPolls > 0 {
			state = ec2types.NatGatewayStatePending
			f.store.natPending[id] = script.pendingPolls
			f.store.natFinal[id] = script.final
		} else {
			state = script.final
		}
		f.store.natScript = nil
	}
	nat := ec2types.NatGateway{
		NatGatewayId: aws.String(id),
		SubnetId:     params.SubnetId,
		State:        state,
		Tags:         tagsFromSpecs(params.TagSpecifications),
	}
	f.store.NatGateways[id] = &nat
	return &ec2.CreateNatGatewayOutput{NatGateway: &nat}, nil
}

func (f *FakeEC2) DeleteNatGateway(ctx context.Context, params *ec2.DeleteNatGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNatGatewayOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteNatGateway", params)
	if err := f.store.failure("DeleteNatGateway"); err != nil {
		return nil, err
	}
	id := aws.ToString(params.NatGatewayId)
	nat, ok := f.store.NatGateways[id]
	if !ok {
		return nil, notFound("NatGatewayNotFound", id)
	}
	// Default: remove immediately. A one-shot SeedNextNatGatewayDeleteState
	// directive keeps the gateway in "deleting" for a scripted number of
	// describe observations so deleteNATGateway's wait-for-deleted loop runs.
	if f.store.natDeleteScript != nil {
		nat.State = ec2types.NatGatewayStateDeleting
		f.store.natDeleting[id] = *f.store.natDeleteScript
		f.store.natDeleteScript = nil
		return &ec2.DeleteNatGatewayOutput{NatGatewayId: aws.String(id)}, nil
	}
	delete(f.store.NatGateways, id)
	return &ec2.DeleteNatGatewayOutput{NatGatewayId: aws.String(id)}, nil
}

func (f *FakeEC2) DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeNatGateways", params)
	if err := f.store.failure("DescribeNatGateways"); err != nil {
		return nil, err
	}
	var out []ec2types.NatGateway
	if len(params.NatGatewayIds) > 0 {
		for _, id := range params.NatGatewayIds {
			if nat, ok := f.store.NatGateways[id]; ok {
				out = append(out, f.advanceNatState(id, nat))
			}
		}
	} else {
		for id, nat := range f.store.NatGateways {
			out = append(out, f.advanceNatState(id, nat))
		}
	}
	return &ec2.DescribeNatGatewaysOutput{NatGateways: out}, nil
}

// advanceNatState reports a scripted gateway transition for this observation:
// a create-time pending gateway reports pending until its count is exhausted
// (then flips to its final state), and a delete-time deleting gateway reports
// deleting until its count is exhausted (then is removed from the store).
// Gateways without a script are returned unchanged. Callers must hold mu.
func (f *FakeEC2) advanceNatState(id string, nat *ec2types.NatGateway) ec2types.NatGateway {
	if rem, ok := f.store.natDeleting[id]; ok {
		deleting := *nat
		deleting.State = ec2types.NatGatewayStateDeleting
		rem--
		if rem <= 0 {
			delete(f.store.NatGateways, id)
			delete(f.store.natDeleting, id)
		} else {
			f.store.natDeleting[id] = rem
		}
		return deleting
	}
	rem, scripted := f.store.natPending[id]
	if !scripted || rem == 0 {
		return *nat
	}
	pending := *nat
	pending.State = ec2types.NatGatewayStatePending
	rem--
	f.store.natPending[id] = rem
	if rem == 0 {
		nat.State = f.store.natFinal[id]
	}
	return pending
}

// ---- Subnet attribute ----

func (f *FakeEC2) ModifySubnetAttribute(ctx context.Context, params *ec2.ModifySubnetAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifySubnetAttributeOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("ModifySubnetAttribute", params)
	if err := f.store.failure("ModifySubnetAttribute"); err != nil {
		return nil, err
	}
	return &ec2.ModifySubnetAttributeOutput{}, nil
}

// filterValue returns the first value of the named filter, or "" if absent.
func filterValue(filters []ec2types.Filter, name string) string {
	for _, filter := range filters {
		if aws.ToString(filter.Name) == name && len(filter.Values) > 0 {
			return filter.Values[0]
		}
	}
	return ""
}
