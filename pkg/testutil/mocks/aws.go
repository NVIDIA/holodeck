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

// Package mocks provides mock implementations for external dependencies
// used in testing.
package mocks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// EC2Client defines the interface for EC2 operations used by the AWS provider.
// This interface allows for easy mocking in tests.
type EC2Client interface {
	CreateVpc(ctx context.Context, params *ec2.CreateVpcInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error)
	ModifyVpcAttribute(ctx context.Context, params *ec2.ModifyVpcAttributeInput,
		optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error)
	DeleteVpc(ctx context.Context, params *ec2.DeleteVpcInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error)
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)

	CreateSubnet(ctx context.Context, params *ec2.CreateSubnetInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error)
	DeleteSubnet(ctx context.Context, params *ec2.DeleteSubnetInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)

	CreateInternetGateway(ctx context.Context,
		params *ec2.CreateInternetGatewayInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error)
	AttachInternetGateway(ctx context.Context,
		params *ec2.AttachInternetGatewayInput,
		optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error)
	DetachInternetGateway(ctx context.Context,
		params *ec2.DetachInternetGatewayInput,
		optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error)
	DeleteInternetGateway(ctx context.Context,
		params *ec2.DeleteInternetGatewayInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error)

	CreateRouteTable(ctx context.Context, params *ec2.CreateRouteTableInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error)
	AssociateRouteTable(ctx context.Context,
		params *ec2.AssociateRouteTableInput,
		optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error)
	CreateRoute(ctx context.Context, params *ec2.CreateRouteInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error)
	DeleteRouteTable(ctx context.Context, params *ec2.DeleteRouteTableInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error)
	DescribeRouteTables(ctx context.Context,
		params *ec2.DescribeRouteTablesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)

	CreateSecurityGroup(ctx context.Context,
		params *ec2.CreateSecurityGroupInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	AuthorizeSecurityGroupIngress(ctx context.Context,
		params *ec2.AuthorizeSecurityGroupIngressInput,
		optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	DeleteSecurityGroup(ctx context.Context,
		params *ec2.DeleteSecurityGroupInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DescribeSecurityGroups(ctx context.Context,
		params *ec2.DescribeSecurityGroupsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)

	RunInstances(ctx context.Context, params *ec2.RunInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	TerminateInstances(ctx context.Context,
		params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	DescribeInstances(ctx context.Context,
		params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)

	CreateTags(ctx context.Context, params *ec2.CreateTagsInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DescribeNetworkInterfaces(ctx context.Context,
		params *ec2.DescribeNetworkInterfacesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
}

// MockEC2Client is a mock implementation of EC2Client for testing.
type MockEC2Client struct {
	// VPC operations
	CreateVpcFunc          func(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error)
	ModifyVpcAttributeFunc func(ctx context.Context, params *ec2.ModifyVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error)
	DeleteVpcFunc          func(ctx context.Context, params *ec2.DeleteVpcInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error)
	DescribeVpcsFunc       func(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)

	// Subnet operations
	CreateSubnetFunc    func(ctx context.Context, params *ec2.CreateSubnetInput, optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error)
	DeleteSubnetFunc    func(ctx context.Context, params *ec2.DeleteSubnetInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)
	DescribeSubnetsFunc func(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)

	// Internet Gateway operations
	CreateInternetGatewayFunc func(ctx context.Context, params *ec2.CreateInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error)
	AttachInternetGatewayFunc func(ctx context.Context, params *ec2.AttachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error)
	DetachInternetGatewayFunc func(ctx context.Context, params *ec2.DetachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error)
	DeleteInternetGatewayFunc func(ctx context.Context, params *ec2.DeleteInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error)

	// Route Table operations
	CreateRouteTableFunc    func(ctx context.Context, params *ec2.CreateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error)
	AssociateRouteTableFunc func(ctx context.Context, params *ec2.AssociateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error)
	CreateRouteFunc         func(ctx context.Context, params *ec2.CreateRouteInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error)
	DeleteRouteTableFunc    func(ctx context.Context, params *ec2.DeleteRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error)
	DescribeRouteTablesFunc func(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)

	// Security Group operations
	CreateSecurityGroupFunc           func(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	AuthorizeSecurityGroupIngressFunc func(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	DeleteSecurityGroupFunc           func(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DescribeSecurityGroupsFunc        func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)

	// Instance operations
	RunInstancesFunc       func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	TerminateInstancesFunc func(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	DescribeInstancesFunc  func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeImagesFunc     func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)

	// Tagging and Network operations
	CreateTagsFunc                      func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DescribeNetworkInterfacesFunc       func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	ModifyNetworkInterfaceAttributeFunc func(ctx context.Context, params *ec2.ModifyNetworkInterfaceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput, error)
	DescribeTagsFunc                    func(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error)

	// Additional operations required by internal/aws.EC2Client
	DescribeInternetGatewaysFunc     func(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)
	DescribeInstanceTypesFunc        func(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
	ReplaceRouteTableAssociationFunc func(ctx context.Context, params *ec2.ReplaceRouteTableAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceRouteTableAssociationOutput, error)
}

// VPC operations

func (m *MockEC2Client) CreateVpc(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	if m.CreateVpcFunc != nil {
		return m.CreateVpcFunc(ctx, params, optFns...)
	}
	return &ec2.CreateVpcOutput{
		Vpc: &types.Vpc{
			VpcId: strPtr("vpc-mock-12345"),
		},
	}, nil
}

func (m *MockEC2Client) ModifyVpcAttribute(ctx context.Context, params *ec2.ModifyVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error) {
	if m.ModifyVpcAttributeFunc != nil {
		return m.ModifyVpcAttributeFunc(ctx, params, optFns...)
	}
	return &ec2.ModifyVpcAttributeOutput{}, nil
}

func (m *MockEC2Client) DeleteVpc(ctx context.Context, params *ec2.DeleteVpcInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
	if m.DeleteVpcFunc != nil {
		return m.DeleteVpcFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteVpcOutput{}, nil
}

func (m *MockEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	if m.DescribeVpcsFunc != nil {
		return m.DescribeVpcsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVpcsOutput{}, nil
}

// Subnet operations

func (m *MockEC2Client) CreateSubnet(ctx context.Context, params *ec2.CreateSubnetInput, optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	if m.CreateSubnetFunc != nil {
		return m.CreateSubnetFunc(ctx, params, optFns...)
	}
	return &ec2.CreateSubnetOutput{
		Subnet: &types.Subnet{
			SubnetId: strPtr("subnet-mock-12345"),
		},
	}, nil
}

func (m *MockEC2Client) DeleteSubnet(ctx context.Context, params *ec2.DeleteSubnetInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
	if m.DeleteSubnetFunc != nil {
		return m.DeleteSubnetFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteSubnetOutput{}, nil
}

func (m *MockEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if m.DescribeSubnetsFunc != nil {
		return m.DescribeSubnetsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSubnetsOutput{}, nil
}

// Internet Gateway operations

func (m *MockEC2Client) CreateInternetGateway(ctx context.Context, params *ec2.CreateInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error) {
	if m.CreateInternetGatewayFunc != nil {
		return m.CreateInternetGatewayFunc(ctx, params, optFns...)
	}
	return &ec2.CreateInternetGatewayOutput{
		InternetGateway: &types.InternetGateway{
			InternetGatewayId: strPtr("igw-mock-12345"),
		},
	}, nil
}

func (m *MockEC2Client) AttachInternetGateway(ctx context.Context, params *ec2.AttachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error) {
	if m.AttachInternetGatewayFunc != nil {
		return m.AttachInternetGatewayFunc(ctx, params, optFns...)
	}
	return &ec2.AttachInternetGatewayOutput{}, nil
}

func (m *MockEC2Client) DetachInternetGateway(ctx context.Context, params *ec2.DetachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
	if m.DetachInternetGatewayFunc != nil {
		return m.DetachInternetGatewayFunc(ctx, params, optFns...)
	}
	return &ec2.DetachInternetGatewayOutput{}, nil
}

func (m *MockEC2Client) DeleteInternetGateway(ctx context.Context, params *ec2.DeleteInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error) {
	if m.DeleteInternetGatewayFunc != nil {
		return m.DeleteInternetGatewayFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteInternetGatewayOutput{}, nil
}

// Route Table operations

func (m *MockEC2Client) CreateRouteTable(ctx context.Context, params *ec2.CreateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
	if m.CreateRouteTableFunc != nil {
		return m.CreateRouteTableFunc(ctx, params, optFns...)
	}
	return &ec2.CreateRouteTableOutput{
		RouteTable: &types.RouteTable{
			RouteTableId: strPtr("rtb-mock-12345"),
		},
	}, nil
}

func (m *MockEC2Client) AssociateRouteTable(ctx context.Context, params *ec2.AssociateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error) {
	if m.AssociateRouteTableFunc != nil {
		return m.AssociateRouteTableFunc(ctx, params, optFns...)
	}
	return &ec2.AssociateRouteTableOutput{
		AssociationId: strPtr("rtbassoc-mock-12345"),
	}, nil
}

func (m *MockEC2Client) CreateRoute(ctx context.Context, params *ec2.CreateRouteInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
	if m.CreateRouteFunc != nil {
		return m.CreateRouteFunc(ctx, params, optFns...)
	}
	return &ec2.CreateRouteOutput{
		Return: boolPtr(true),
	}, nil
}

func (m *MockEC2Client) DeleteRouteTable(ctx context.Context, params *ec2.DeleteRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error) {
	if m.DeleteRouteTableFunc != nil {
		return m.DeleteRouteTableFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteRouteTableOutput{}, nil
}

func (m *MockEC2Client) DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	if m.DescribeRouteTablesFunc != nil {
		return m.DescribeRouteTablesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeRouteTablesOutput{}, nil
}

// Security Group operations

func (m *MockEC2Client) CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	if m.CreateSecurityGroupFunc != nil {
		return m.CreateSecurityGroupFunc(ctx, params, optFns...)
	}
	return &ec2.CreateSecurityGroupOutput{
		GroupId: strPtr("sg-mock-12345"),
	}, nil
}

func (m *MockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	if m.AuthorizeSecurityGroupIngressFunc != nil {
		return m.AuthorizeSecurityGroupIngressFunc(ctx, params, optFns...)
	}
	return &ec2.AuthorizeSecurityGroupIngressOutput{
		Return: boolPtr(true),
	}, nil
}

func (m *MockEC2Client) DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	if m.DeleteSecurityGroupFunc != nil {
		return m.DeleteSecurityGroupFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteSecurityGroupOutput{}, nil
}

func (m *MockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if m.DescribeSecurityGroupsFunc != nil {
		return m.DescribeSecurityGroupsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}

// Instance operations

func (m *MockEC2Client) RunInstances(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	if m.RunInstancesFunc != nil {
		return m.RunInstancesFunc(ctx, params, optFns...)
	}
	return &ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId: strPtr("i-mock-12345"),
				NetworkInterfaces: []types.InstanceNetworkInterface{
					{
						NetworkInterfaceId: strPtr("eni-mock-12345"),
					},
				},
			},
		},
	}, nil
}

func (m *MockEC2Client) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	if m.TerminateInstancesFunc != nil {
		return m.TerminateInstancesFunc(ctx, params, optFns...)
	}
	return &ec2.TerminateInstancesOutput{}, nil
}

func (m *MockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.DescribeInstancesFunc != nil {
		return m.DescribeInstancesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:    strPtr("i-mock-12345"),
						PublicDnsName: strPtr("ec2-1-2-3-4.compute.amazonaws.com"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
					},
				},
			},
		},
	}, nil
}

func (m *MockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	if m.DescribeImagesFunc != nil {
		return m.DescribeImagesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeImagesOutput{
		Images: []types.Image{
			{
				ImageId:      strPtr("ami-mock-12345"),
				CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
			},
		},
	}, nil
}

// Tagging and Network operations

func (m *MockEC2Client) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	if m.CreateTagsFunc != nil {
		return m.CreateTagsFunc(ctx, params, optFns...)
	}
	return &ec2.CreateTagsOutput{}, nil
}

func (m *MockEC2Client) DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	if m.DescribeNetworkInterfacesFunc != nil {
		return m.DescribeNetworkInterfacesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}

func (m *MockEC2Client) ModifyNetworkInterfaceAttribute(ctx context.Context, params *ec2.ModifyNetworkInterfaceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput, error) {
	if m.ModifyNetworkInterfaceAttributeFunc != nil {
		return m.ModifyNetworkInterfaceAttributeFunc(ctx, params, optFns...)
	}
	return &ec2.ModifyNetworkInterfaceAttributeOutput{}, nil
}

func (m *MockEC2Client) DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	if m.DescribeTagsFunc != nil {
		return m.DescribeTagsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeTagsOutput{}, nil
}

func (m *MockEC2Client) DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	if m.DescribeInternetGatewaysFunc != nil {
		return m.DescribeInternetGatewaysFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}

func (m *MockEC2Client) DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	if m.DescribeInstanceTypesFunc != nil {
		return m.DescribeInstanceTypesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstanceTypesOutput{}, nil
}

func (m *MockEC2Client) ReplaceRouteTableAssociation(ctx context.Context, params *ec2.ReplaceRouteTableAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceRouteTableAssociationOutput, error) {
	if m.ReplaceRouteTableAssociationFunc != nil {
		return m.ReplaceRouteTableAssociationFunc(ctx, params, optFns...)
	}
	return &ec2.ReplaceRouteTableAssociationOutput{}, nil
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
