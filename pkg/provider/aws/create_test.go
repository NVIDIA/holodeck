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
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	internalaws "github.com/NVIDIA/holodeck/internal/aws"
	"github.com/NVIDIA/holodeck/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Compile-time check that mockEC2Client implements internalaws.EC2Client.
var _ internalaws.EC2Client = (*mockEC2Client)(nil)

// mockEC2Client is a mock implementation of internalaws.EC2Client for testing.
type mockEC2Client struct {
	// Track calls for verification
	modifyNetworkInterfaceAttributeCalls []ec2.ModifyNetworkInterfaceAttributeInput
	createTagsCalls                      []ec2.CreateTagsInput
	runInstancesCalls                    []ec2.RunInstancesInput
	describeInstancesCalls               []ec2.DescribeInstancesInput

	// Configure responses
	runInstancesOutput      *ec2.RunInstancesOutput
	describeInstancesOutput *ec2.DescribeInstancesOutput

	// Configure errors
	modifyNetworkInterfaceAttributeErr error
	createTagsErr                      error
	runInstancesErr                    error
	describeInstancesErr               error
}

func (m *mockEC2Client) CreateVpc(ctx context.Context, params *ec2.CreateVpcInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	return &ec2.CreateVpcOutput{
		Vpc: &types.Vpc{VpcId: aws.String("vpc-123")},
	}, nil
}

func (m *mockEC2Client) ModifyVpcAttribute(ctx context.Context, params *ec2.ModifyVpcAttributeInput,
	optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error) {
	return &ec2.ModifyVpcAttributeOutput{}, nil
}

func (m *mockEC2Client) DeleteVpc(ctx context.Context, params *ec2.DeleteVpcInput,
	optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
	return &ec2.DeleteVpcOutput{}, nil
}

func (m *mockEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{}, nil
}

func (m *mockEC2Client) CreateSubnet(ctx context.Context, params *ec2.CreateSubnetInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	return &ec2.CreateSubnetOutput{
		Subnet: &types.Subnet{SubnetId: aws.String("subnet-123")},
	}, nil
}

func (m *mockEC2Client) DeleteSubnet(ctx context.Context, params *ec2.DeleteSubnetInput,
	optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
	return &ec2.DeleteSubnetOutput{}, nil
}

func (m *mockEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{}, nil
}

func (m *mockEC2Client) CreateInternetGateway(ctx context.Context,
	params *ec2.CreateInternetGatewayInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error) {
	return &ec2.CreateInternetGatewayOutput{
		InternetGateway: &types.InternetGateway{InternetGatewayId: aws.String("igw-123")},
	}, nil
}

func (m *mockEC2Client) AttachInternetGateway(ctx context.Context,
	params *ec2.AttachInternetGatewayInput,
	optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error) {
	return &ec2.AttachInternetGatewayOutput{}, nil
}

func (m *mockEC2Client) DetachInternetGateway(ctx context.Context,
	params *ec2.DetachInternetGatewayInput,
	optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
	return &ec2.DetachInternetGatewayOutput{}, nil
}

func (m *mockEC2Client) DeleteInternetGateway(ctx context.Context,
	params *ec2.DeleteInternetGatewayInput,
	optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error) {
	return &ec2.DeleteInternetGatewayOutput{}, nil
}

func (m *mockEC2Client) DescribeInternetGateways(ctx context.Context,
	params *ec2.DescribeInternetGatewaysInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}

func (m *mockEC2Client) CreateRouteTable(ctx context.Context, params *ec2.CreateRouteTableInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
	return &ec2.CreateRouteTableOutput{
		RouteTable: &types.RouteTable{RouteTableId: aws.String("rtb-123")},
	}, nil
}

func (m *mockEC2Client) AssociateRouteTable(ctx context.Context, params *ec2.AssociateRouteTableInput,
	optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error) {
	return &ec2.AssociateRouteTableOutput{}, nil
}

func (m *mockEC2Client) CreateRoute(ctx context.Context, params *ec2.CreateRouteInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
	return &ec2.CreateRouteOutput{}, nil
}

func (m *mockEC2Client) DeleteRouteTable(ctx context.Context, params *ec2.DeleteRouteTableInput,
	optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error) {
	return &ec2.DeleteRouteTableOutput{}, nil
}

func (m *mockEC2Client) DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{}, nil
}

func (m *mockEC2Client) ReplaceRouteTableAssociation(ctx context.Context,
	params *ec2.ReplaceRouteTableAssociationInput,
	optFns ...func(*ec2.Options)) (*ec2.ReplaceRouteTableAssociationOutput, error) {
	return &ec2.ReplaceRouteTableAssociationOutput{}, nil
}

func (m *mockEC2Client) CreateSecurityGroup(ctx context.Context,
	params *ec2.CreateSecurityGroupInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	return &ec2.CreateSecurityGroupOutput{GroupId: aws.String("sg-123")}, nil
}

func (m *mockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context,
	params *ec2.AuthorizeSecurityGroupIngressInput,
	optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	return &ec2.AuthorizeSecurityGroupIngressOutput{}, nil
}

func (m *mockEC2Client) DeleteSecurityGroup(ctx context.Context,
	params *ec2.DeleteSecurityGroupInput,
	optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	return &ec2.DeleteSecurityGroupOutput{}, nil
}

func (m *mockEC2Client) DescribeSecurityGroups(ctx context.Context,
	params *ec2.DescribeSecurityGroupsInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}

func (m *mockEC2Client) RunInstances(ctx context.Context, params *ec2.RunInstancesInput,
	optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	m.runInstancesCalls = append(m.runInstancesCalls, *params)
	if m.runInstancesErr != nil {
		return nil, m.runInstancesErr
	}
	if m.runInstancesOutput != nil {
		return m.runInstancesOutput, nil
	}
	return &ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId: aws.String("i-123"),
				NetworkInterfaces: []types.InstanceNetworkInterface{
					{NetworkInterfaceId: aws.String("eni-123")},
				},
			},
		},
	}, nil
}

func (m *mockEC2Client) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput,
	optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	return &ec2.TerminateInstancesOutput{}, nil
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	m.describeInstancesCalls = append(m.describeInstancesCalls, *params)
	if m.describeInstancesErr != nil {
		return nil, m.describeInstancesErr
	}
	if m.describeInstancesOutput != nil {
		return m.describeInstancesOutput, nil
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-123"),
						PublicDnsName:   aws.String("ec2-1-2-3-4.compute.amazonaws.com"),
						PublicIpAddress: aws.String("1.2.3.4"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						NetworkInterfaces: []types.InstanceNetworkInterface{
							{NetworkInterfaceId: aws.String("eni-123")},
						},
					},
				},
			},
		},
	}, nil
}

func (m *mockEC2Client) DescribeInstanceTypes(ctx context.Context,
	params *ec2.DescribeInstanceTypesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	return &ec2.DescribeInstanceTypesOutput{}, nil
}

func (m *mockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{
		Images: []types.Image{
			{ImageId: aws.String("ami-123"), CreationDate: aws.String("2024-01-01T00:00:00Z")},
		},
	}, nil
}

func (m *mockEC2Client) DescribeNetworkInterfaces(ctx context.Context,
	params *ec2.DescribeNetworkInterfacesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}

func (m *mockEC2Client) ModifyNetworkInterfaceAttribute(ctx context.Context,
	params *ec2.ModifyNetworkInterfaceAttributeInput,
	optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput, error) {
	m.modifyNetworkInterfaceAttributeCalls = append(m.modifyNetworkInterfaceAttributeCalls, *params)
	if m.modifyNetworkInterfaceAttributeErr != nil {
		return nil, m.modifyNetworkInterfaceAttributeErr
	}
	return &ec2.ModifyNetworkInterfaceAttributeOutput{}, nil
}

func (m *mockEC2Client) CreateTags(ctx context.Context, params *ec2.CreateTagsInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	m.createTagsCalls = append(m.createTagsCalls, *params)
	if m.createTagsErr != nil {
		return nil, m.createTagsErr
	}
	return &ec2.CreateTagsOutput{}, nil
}

func (m *mockEC2Client) DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	return &ec2.DescribeTagsOutput{}, nil
}

// mockLogger creates a logger for testing that doesn't block on channels.
func mockLogger() *logger.FunLogger {
	log := &logger.FunLogger{
		Out:  io.Discard,
		Done: make(chan struct{}, 100),
		Fail: make(chan struct{}, 100),
		Wg:   &sync.WaitGroup{},
		IsCI: true, // Prevents interactive terminal behavior
	}
	return log
}

func TestCreateEC2Instance_DisablesSourceDestCheck(t *testing.T) {
	mock := &mockEC2Client{}
	log := mockLogger()

	provider := &Provider{
		ec2: mock,
		log: log,
		Environment: &v1alpha1.Environment{
			Spec: v1alpha1.EnvironmentSpec{
				Auth: v1alpha1.Auth{
					KeyName: "test-key",
				},
				Instance: v1alpha1.Instance{
					Image: v1alpha1.Image{
						ImageId: aws.String("ami-123"),
					},
					Type: "t3.medium",
				},
			},
		},
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("test")},
		},
	}

	cache := &AWS{
		SecurityGroupid: "sg-123",
		Subnetid:        "subnet-123",
	}

	err := provider.createEC2Instance(cache)
	if err != nil {
		t.Fatalf("createEC2Instance failed: %v", err)
	}

	// Verify ModifyNetworkInterfaceAttribute was called
	if len(mock.modifyNetworkInterfaceAttributeCalls) == 0 {
		t.Fatal("ModifyNetworkInterfaceAttribute was not called - Source/Dest Check not disabled")
	}

	call := mock.modifyNetworkInterfaceAttributeCalls[0]

	// Verify the network interface ID
	if call.NetworkInterfaceId == nil || *call.NetworkInterfaceId != "eni-123" {
		t.Errorf("Expected NetworkInterfaceId 'eni-123', got %v", call.NetworkInterfaceId)
	}

	// Verify SourceDestCheck is set to false
	if call.SourceDestCheck == nil {
		t.Fatal("SourceDestCheck was not set in the call")
	}
	if call.SourceDestCheck.Value == nil || *call.SourceDestCheck.Value != false {
		t.Errorf("Expected SourceDestCheck.Value to be false, got %v", call.SourceDestCheck.Value)
	}
}

func TestCreateEC2Instance_SourceDestCheckError(t *testing.T) {
	expectedErr := errors.New("modify network interface failed")
	mock := &mockEC2Client{
		modifyNetworkInterfaceAttributeErr: expectedErr,
	}
	log := mockLogger()

	provider := &Provider{
		ec2: mock,
		log: log,
		Environment: &v1alpha1.Environment{
			Spec: v1alpha1.EnvironmentSpec{
				Auth: v1alpha1.Auth{
					KeyName: "test-key",
				},
				Instance: v1alpha1.Instance{
					Image: v1alpha1.Image{
						ImageId: aws.String("ami-123"),
					},
					Type: "t3.medium",
				},
			},
		},
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("test")},
		},
	}

	cache := &AWS{
		SecurityGroupid: "sg-123",
		Subnetid:        "subnet-123",
	}

	err := provider.createEC2Instance(cache)
	if err == nil {
		t.Fatal("Expected error when ModifyNetworkInterfaceAttribute fails")
	}

	expectedErrMsg := "error disabling source/dest check: " + expectedErr.Error()
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error %q, got: %v", expectedErrMsg, err)
	}
}
