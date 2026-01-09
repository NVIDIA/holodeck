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

// Package aws provides internal AWS utilities and interfaces for the holodeck
// project. This package is internal and not intended for external consumption.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// EC2Client defines the interface for EC2 operations used throughout holodeck.
// This interface enables dependency injection and facilitates unit testing
// by allowing mock implementations to be substituted for the real EC2 client.
//
// The interface follows AWS SDK for Go v2 best practices for unit testing:
// https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/unit-testing.html
type EC2Client interface {
	// VPC operations
	CreateVpc(ctx context.Context, params *ec2.CreateVpcInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error)
	ModifyVpcAttribute(ctx context.Context, params *ec2.ModifyVpcAttributeInput,
		optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error)
	DeleteVpc(ctx context.Context, params *ec2.DeleteVpcInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error)
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)

	// Subnet operations
	CreateSubnet(ctx context.Context, params *ec2.CreateSubnetInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error)
	DeleteSubnet(ctx context.Context, params *ec2.DeleteSubnetInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)

	// Internet Gateway operations
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
	DescribeInternetGateways(ctx context.Context,
		params *ec2.DescribeInternetGatewaysInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)

	// Route Table operations
	CreateRouteTable(ctx context.Context, params *ec2.CreateRouteTableInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error)
	AssociateRouteTable(ctx context.Context, params *ec2.AssociateRouteTableInput,
		optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error)
	CreateRoute(ctx context.Context, params *ec2.CreateRouteInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error)
	DeleteRouteTable(ctx context.Context, params *ec2.DeleteRouteTableInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error)
	DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
	ReplaceRouteTableAssociation(ctx context.Context,
		params *ec2.ReplaceRouteTableAssociationInput,
		optFns ...func(*ec2.Options)) (*ec2.ReplaceRouteTableAssociationOutput,
		error)

	// Security Group operations
	CreateSecurityGroup(ctx context.Context,
		params *ec2.CreateSecurityGroupInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	AuthorizeSecurityGroupIngress(ctx context.Context,
		params *ec2.AuthorizeSecurityGroupIngressInput,
		optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput,
		error)
	DeleteSecurityGroup(ctx context.Context,
		params *ec2.DeleteSecurityGroupInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DescribeSecurityGroups(ctx context.Context,
		params *ec2.DescribeSecurityGroupsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)

	// Instance operations
	RunInstances(ctx context.Context, params *ec2.RunInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceTypes(ctx context.Context,
		params *ec2.DescribeInstanceTypesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)

	// Image operations
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)

	// Network Interface operations
	DescribeNetworkInterfaces(ctx context.Context,
		params *ec2.DescribeNetworkInterfacesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	ModifyNetworkInterfaceAttribute(ctx context.Context,
		params *ec2.ModifyNetworkInterfaceAttributeInput,
		optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput,
		error)

	// Tag operations
	CreateTags(ctx context.Context, params *ec2.CreateTagsInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error)
}

// Ensure *ec2.Client implements EC2Client at compile time.
// This provides a compile-time check that our interface matches the real
// client.
var _ EC2Client = (*ec2.Client)(nil)
