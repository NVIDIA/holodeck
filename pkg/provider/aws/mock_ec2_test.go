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

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	internalaws "github.com/NVIDIA/holodeck/internal/aws"
)

// MockEC2Client is a mock implementation of EC2Client for testing.
type MockEC2Client struct {
	// VPC
	CreateVpcFunc     func(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error)
	ModifyVpcAttrFunc func(ctx context.Context, params *ec2.ModifyVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error)
	DeleteVpcFunc     func(ctx context.Context, params *ec2.DeleteVpcInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error)
	DescribeVpcsFunc  func(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)

	// Subnet
	CreateSubnetFunc    func(ctx context.Context, params *ec2.CreateSubnetInput, optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error)
	DeleteSubnetFunc    func(ctx context.Context, params *ec2.DeleteSubnetInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)
	DescribeSubnetsFunc func(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)

	// Internet Gateway
	CreateIGWFunc    func(ctx context.Context, params *ec2.CreateInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error)
	AttachIGWFunc    func(ctx context.Context, params *ec2.AttachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error)
	DetachIGWFunc    func(ctx context.Context, params *ec2.DetachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error)
	DeleteIGWFunc    func(ctx context.Context, params *ec2.DeleteInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error)
	DescribeIGWsFunc func(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)

	// Route Table
	CreateRTFunc       func(ctx context.Context, params *ec2.CreateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error)
	AssociateRTFunc    func(ctx context.Context, params *ec2.AssociateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error)
	CreateRouteFunc    func(ctx context.Context, params *ec2.CreateRouteInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error)
	DeleteRTFunc       func(ctx context.Context, params *ec2.DeleteRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error)
	DescribeRTsFunc    func(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
	ReplaceRTAssocFunc func(ctx context.Context, params *ec2.ReplaceRouteTableAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceRouteTableAssociationOutput, error)

	// Security Group
	CreateSGFunc    func(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	AuthorizeSGFunc func(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	DeleteSGFunc    func(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DescribeSGsFunc func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)

	// Instance
	RunInstancesFunc      func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	TerminateInstsFunc    func(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	DescribeInstsFunc     func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstTypesFunc func(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)

	// Image
	DescribeImagesFunc func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)

	// Network Interface
	DescribeNIsFunc  func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	ModifyNIAttrFunc func(ctx context.Context, params *ec2.ModifyNetworkInterfaceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput, error)

	// Tags
	CreateTagsFunc   func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DescribeTagsFunc func(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error)
}

// VPC operations
func (m *MockEC2Client) CreateVpc(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	if m.CreateVpcFunc != nil {
		return m.CreateVpcFunc(ctx, params, optFns...)
	}
	return &ec2.CreateVpcOutput{
		Vpc: &types.Vpc{VpcId: aws.String("vpc-mock-12345")},
	}, nil
}

func (m *MockEC2Client) ModifyVpcAttribute(ctx context.Context, params *ec2.ModifyVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error) {
	if m.ModifyVpcAttrFunc != nil {
		return m.ModifyVpcAttrFunc(ctx, params, optFns...)
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
		Subnet: &types.Subnet{SubnetId: aws.String("subnet-mock-12345")},
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
	if m.CreateIGWFunc != nil {
		return m.CreateIGWFunc(ctx, params, optFns...)
	}
	return &ec2.CreateInternetGatewayOutput{
		InternetGateway: &types.InternetGateway{
			InternetGatewayId: aws.String("igw-mock-12345"),
		},
	}, nil
}

func (m *MockEC2Client) AttachInternetGateway(ctx context.Context, params *ec2.AttachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error) {
	if m.AttachIGWFunc != nil {
		return m.AttachIGWFunc(ctx, params, optFns...)
	}
	return &ec2.AttachInternetGatewayOutput{}, nil
}

func (m *MockEC2Client) DetachInternetGateway(ctx context.Context, params *ec2.DetachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
	if m.DetachIGWFunc != nil {
		return m.DetachIGWFunc(ctx, params, optFns...)
	}
	return &ec2.DetachInternetGatewayOutput{}, nil
}

func (m *MockEC2Client) DeleteInternetGateway(ctx context.Context, params *ec2.DeleteInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error) {
	if m.DeleteIGWFunc != nil {
		return m.DeleteIGWFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteInternetGatewayOutput{}, nil
}

func (m *MockEC2Client) DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	if m.DescribeIGWsFunc != nil {
		return m.DescribeIGWsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}

// Route Table operations
func (m *MockEC2Client) CreateRouteTable(ctx context.Context, params *ec2.CreateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
	if m.CreateRTFunc != nil {
		return m.CreateRTFunc(ctx, params, optFns...)
	}
	return &ec2.CreateRouteTableOutput{
		RouteTable: &types.RouteTable{RouteTableId: aws.String("rtb-mock-12345")},
	}, nil
}

func (m *MockEC2Client) AssociateRouteTable(ctx context.Context, params *ec2.AssociateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error) {
	if m.AssociateRTFunc != nil {
		return m.AssociateRTFunc(ctx, params, optFns...)
	}
	return &ec2.AssociateRouteTableOutput{}, nil
}

func (m *MockEC2Client) CreateRoute(ctx context.Context, params *ec2.CreateRouteInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
	if m.CreateRouteFunc != nil {
		return m.CreateRouteFunc(ctx, params, optFns...)
	}
	return &ec2.CreateRouteOutput{}, nil
}

func (m *MockEC2Client) DeleteRouteTable(ctx context.Context, params *ec2.DeleteRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error) {
	if m.DeleteRTFunc != nil {
		return m.DeleteRTFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteRouteTableOutput{}, nil
}

func (m *MockEC2Client) DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	if m.DescribeRTsFunc != nil {
		return m.DescribeRTsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeRouteTablesOutput{}, nil
}

func (m *MockEC2Client) ReplaceRouteTableAssociation(ctx context.Context, params *ec2.ReplaceRouteTableAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceRouteTableAssociationOutput, error) {
	if m.ReplaceRTAssocFunc != nil {
		return m.ReplaceRTAssocFunc(ctx, params, optFns...)
	}
	return &ec2.ReplaceRouteTableAssociationOutput{}, nil
}

// Security Group operations
func (m *MockEC2Client) CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	if m.CreateSGFunc != nil {
		return m.CreateSGFunc(ctx, params, optFns...)
	}
	return &ec2.CreateSecurityGroupOutput{
		GroupId: aws.String("sg-mock-12345"),
	}, nil
}

func (m *MockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	if m.AuthorizeSGFunc != nil {
		return m.AuthorizeSGFunc(ctx, params, optFns...)
	}
	return &ec2.AuthorizeSecurityGroupIngressOutput{}, nil
}

func (m *MockEC2Client) DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	if m.DeleteSGFunc != nil {
		return m.DeleteSGFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteSecurityGroupOutput{}, nil
}

func (m *MockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if m.DescribeSGsFunc != nil {
		return m.DescribeSGsFunc(ctx, params, optFns...)
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
				InstanceId: aws.String("i-mock-12345"),
				NetworkInterfaces: []types.InstanceNetworkInterface{
					{NetworkInterfaceId: aws.String("eni-mock-12345")},
				},
			},
		},
	}, nil
}

func (m *MockEC2Client) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	if m.TerminateInstsFunc != nil {
		return m.TerminateInstsFunc(ctx, params, optFns...)
	}
	return &ec2.TerminateInstancesOutput{}, nil
}

func (m *MockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.DescribeInstsFunc != nil {
		return m.DescribeInstsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:    aws.String("i-mock-12345"),
						PublicDnsName: aws.String("ec2-mock.compute.amazonaws.com"),
					},
				},
			},
		},
	}, nil
}

func (m *MockEC2Client) DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	if m.DescribeInstTypesFunc != nil {
		return m.DescribeInstTypesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []types.InstanceTypeInfo{
			{InstanceType: types.InstanceTypeT3Medium},
			{InstanceType: types.InstanceTypeT3Large},
		},
	}, nil
}

// Image operations
func (m *MockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	if m.DescribeImagesFunc != nil {
		return m.DescribeImagesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeImagesOutput{
		Images: []types.Image{
			{
				ImageId:      aws.String("ami-mock-12345"),
				CreationDate: aws.String("2024-01-01T00:00:00.000Z"),
				Name:         aws.String("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04"),
				Architecture: types.ArchitectureValuesX8664,
			},
		},
	}, nil
}

// Network Interface operations
func (m *MockEC2Client) DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	if m.DescribeNIsFunc != nil {
		return m.DescribeNIsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}

func (m *MockEC2Client) ModifyNetworkInterfaceAttribute(ctx context.Context, params *ec2.ModifyNetworkInterfaceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput, error) {
	if m.ModifyNIAttrFunc != nil {
		return m.ModifyNIAttrFunc(ctx, params, optFns...)
	}
	return &ec2.ModifyNetworkInterfaceAttributeOutput{}, nil
}

// Tag operations
func (m *MockEC2Client) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	if m.CreateTagsFunc != nil {
		return m.CreateTagsFunc(ctx, params, optFns...)
	}
	return &ec2.CreateTagsOutput{}, nil
}

func (m *MockEC2Client) DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	if m.DescribeTagsFunc != nil {
		return m.DescribeTagsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeTagsOutput{}, nil
}

// NewMockEC2Client creates a new MockEC2Client with default implementations.
func NewMockEC2Client() *MockEC2Client {
	return &MockEC2Client{}
}

// NewMockEC2ClientWithError creates a mock that returns errors for all operations.
func NewMockEC2ClientWithError(err error) *MockEC2Client {
	return &MockEC2Client{
		CreateVpcFunc: func(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
			return nil, err
		},
		CreateSubnetFunc: func(ctx context.Context, params *ec2.CreateSubnetInput, optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
			return nil, err
		},
		// Add more error-returning functions as needed
	}
}

// Ensure MockEC2Client implements internalaws.EC2Client
var _ internalaws.EC2Client = (*MockEC2Client)(nil)

// Helper functions for creating test data
func strPtr(s string) *string {
	return &s
}

// errMock is a helper for creating mock errors
type errMock string

func (e errMock) Error() string {
	return string(e)
}

// ErrMockVPCCreation is a mock error for VPC creation failures
var ErrMockVPCCreation = errMock("mock VPC creation error")

// ErrMockSubnetCreation is a mock error for subnet creation failures
var ErrMockSubnetCreation = errMock("mock subnet creation error")

// ErrMockInstanceNotFound is a mock error for instance not found
var ErrMockInstanceNotFound = errMock("mock instance not found")

// ErrMockDescribeImages is a mock error for DescribeImages failures
var ErrMockDescribeImages = fmt.Errorf("mock describe images error")
