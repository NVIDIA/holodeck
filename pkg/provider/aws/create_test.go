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
	"strings"
	"sync"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	internalaws "github.com/NVIDIA/holodeck/internal/aws"
	"github.com/NVIDIA/holodeck/internal/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

// Extended mockEC2Client with call tracking and error injection
type extendedMockEC2Client struct {
	mockEC2Client

	// Track calls
	createVpcCalls              []ec2.CreateVpcInput
	modifyVpcAttributeCalls     []ec2.ModifyVpcAttributeInput
	createSubnetCalls           []ec2.CreateSubnetInput
	createInternetGatewayCalls  []ec2.CreateInternetGatewayInput
	attachInternetGatewayCalls  []ec2.AttachInternetGatewayInput
	createRouteTableCalls       []ec2.CreateRouteTableInput
	associateRouteTableCalls    []ec2.AssociateRouteTableInput
	createRouteCalls            []ec2.CreateRouteInput
	createSecurityGroupCalls    []ec2.CreateSecurityGroupInput
	authorizeSecurityGroupCalls []ec2.AuthorizeSecurityGroupIngressInput

	// Error injection
	createVpcErr              error
	modifyVpcAttributeErr     error
	createSubnetErr           error
	createInternetGatewayErr  error
	attachInternetGatewayErr  error
	createRouteTableErr       error
	associateRouteTableErr    error
	createRouteErr            error
	createSecurityGroupErr    error
	authorizeSecurityGroupErr error
}

func (m *extendedMockEC2Client) CreateVpc(ctx context.Context, params *ec2.CreateVpcInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	m.createVpcCalls = append(m.createVpcCalls, *params)
	if m.createVpcErr != nil {
		return nil, m.createVpcErr
	}
	return &ec2.CreateVpcOutput{
		Vpc: &types.Vpc{VpcId: aws.String("vpc-test-123")},
	}, nil
}

func (m *extendedMockEC2Client) ModifyVpcAttribute(ctx context.Context, params *ec2.ModifyVpcAttributeInput,
	optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error) {
	m.modifyVpcAttributeCalls = append(m.modifyVpcAttributeCalls, *params)
	if m.modifyVpcAttributeErr != nil {
		return nil, m.modifyVpcAttributeErr
	}
	return &ec2.ModifyVpcAttributeOutput{}, nil
}

func (m *extendedMockEC2Client) CreateSubnet(ctx context.Context, params *ec2.CreateSubnetInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	m.createSubnetCalls = append(m.createSubnetCalls, *params)
	if m.createSubnetErr != nil {
		return nil, m.createSubnetErr
	}
	return &ec2.CreateSubnetOutput{
		Subnet: &types.Subnet{SubnetId: aws.String("subnet-test-123")},
	}, nil
}

func (m *extendedMockEC2Client) CreateInternetGateway(ctx context.Context,
	params *ec2.CreateInternetGatewayInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error) {
	m.createInternetGatewayCalls = append(m.createInternetGatewayCalls, *params)
	if m.createInternetGatewayErr != nil {
		return nil, m.createInternetGatewayErr
	}
	return &ec2.CreateInternetGatewayOutput{
		InternetGateway: &types.InternetGateway{
			InternetGatewayId: aws.String("igw-test-123"),
			Attachments: []types.InternetGatewayAttachment{
				{VpcId: aws.String("vpc-test-123")},
			},
		},
	}, nil
}

func (m *extendedMockEC2Client) AttachInternetGateway(ctx context.Context,
	params *ec2.AttachInternetGatewayInput,
	optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error) {
	m.attachInternetGatewayCalls = append(m.attachInternetGatewayCalls, *params)
	if m.attachInternetGatewayErr != nil {
		return nil, m.attachInternetGatewayErr
	}
	return &ec2.AttachInternetGatewayOutput{}, nil
}

func (m *extendedMockEC2Client) CreateRouteTable(ctx context.Context, params *ec2.CreateRouteTableInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
	m.createRouteTableCalls = append(m.createRouteTableCalls, *params)
	if m.createRouteTableErr != nil {
		return nil, m.createRouteTableErr
	}
	return &ec2.CreateRouteTableOutput{
		RouteTable: &types.RouteTable{RouteTableId: aws.String("rtb-test-123")},
	}, nil
}

func (m *extendedMockEC2Client) AssociateRouteTable(ctx context.Context, params *ec2.AssociateRouteTableInput,
	optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error) {
	m.associateRouteTableCalls = append(m.associateRouteTableCalls, *params)
	if m.associateRouteTableErr != nil {
		return nil, m.associateRouteTableErr
	}
	return &ec2.AssociateRouteTableOutput{}, nil
}

func (m *extendedMockEC2Client) CreateRoute(ctx context.Context, params *ec2.CreateRouteInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
	m.createRouteCalls = append(m.createRouteCalls, *params)
	if m.createRouteErr != nil {
		return nil, m.createRouteErr
	}
	return &ec2.CreateRouteOutput{}, nil
}

func (m *extendedMockEC2Client) CreateSecurityGroup(ctx context.Context,
	params *ec2.CreateSecurityGroupInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	m.createSecurityGroupCalls = append(m.createSecurityGroupCalls, *params)
	if m.createSecurityGroupErr != nil {
		return nil, m.createSecurityGroupErr
	}
	return &ec2.CreateSecurityGroupOutput{GroupId: aws.String("sg-test-123")}, nil
}

func (m *extendedMockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context,
	params *ec2.AuthorizeSecurityGroupIngressInput,
	optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	m.authorizeSecurityGroupCalls = append(m.authorizeSecurityGroupCalls, *params)
	if m.authorizeSecurityGroupErr != nil {
		return nil, m.authorizeSecurityGroupErr
	}
	return &ec2.AuthorizeSecurityGroupIngressOutput{}, nil
}

// Helper to create a test provider
func createTestProvider(mock internalaws.EC2Client) *Provider {
	return &Provider{
		ec2: mock,
		log: mockLogger(),
		Environment: &v1alpha1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-env",
			},
			Spec: v1alpha1.EnvironmentSpec{
				Instance: v1alpha1.Instance{
					IngressIpRanges: []string{"1.2.3.4/32"},
				},
			},
		},
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("test")},
		},
	}
}

func TestCreateVPC_Success(t *testing.T) {
	mock := &extendedMockEC2Client{}
	provider := createTestProvider(mock)
	cache := &AWS{}

	err := provider.createVPC(cache)
	if err != nil {
		t.Fatalf("createVPC failed: %v", err)
	}

	// Verify VPC was created
	if len(mock.createVpcCalls) == 0 {
		t.Fatal("CreateVpc was not called")
	}

	// Verify VPC ID was set in cache
	if cache.Vpcid != "vpc-test-123" {
		t.Errorf("Expected Vpcid 'vpc-test-123', got %q", cache.Vpcid)
	}

	// Verify ModifyVpcAttribute was called
	if len(mock.modifyVpcAttributeCalls) == 0 {
		t.Fatal("ModifyVpcAttribute was not called")
	}

	modCall := mock.modifyVpcAttributeCalls[0]
	if modCall.VpcId == nil || *modCall.VpcId != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", modCall.VpcId)
	}
	if modCall.EnableDnsHostnames == nil || modCall.EnableDnsHostnames.Value == nil || !*modCall.EnableDnsHostnames.Value {
		t.Error("Expected EnableDnsHostnames to be true")
	}
}

func TestCreateVPC_CreateVpcError(t *testing.T) {
	expectedErr := errors.New("VPC creation failed")
	mock := &extendedMockEC2Client{
		createVpcErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{}

	err := provider.createVPC(cache)
	if err == nil {
		t.Fatal("Expected error when CreateVpc fails")
	}

	if !contains(err.Error(), "error creating VPC") {
		t.Errorf("Expected error to contain 'error creating VPC', got: %v", err)
	}
}

func TestCreateVPC_ModifyVpcAttributeError(t *testing.T) {
	expectedErr := errors.New("modify VPC attribute failed")
	mock := &extendedMockEC2Client{
		modifyVpcAttributeErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{}

	err := provider.createVPC(cache)
	if err == nil {
		t.Fatal("Expected error when ModifyVpcAttribute fails")
	}

	if !contains(err.Error(), "error modifying VPC attributes") {
		t.Errorf("Expected error to contain 'error modifying VPC attributes', got: %v", err)
	}
}

func TestCreateSubnet_Success(t *testing.T) {
	mock := &extendedMockEC2Client{}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createSubnet(cache)
	if err != nil {
		t.Fatalf("createSubnet failed: %v", err)
	}

	// Verify subnet was created
	if len(mock.createSubnetCalls) == 0 {
		t.Fatal("CreateSubnet was not called")
	}

	call := mock.createSubnetCalls[0]
	if call.VpcId == nil || *call.VpcId != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", call.VpcId)
	}

	// Verify subnet ID was set in cache
	if cache.Subnetid != "subnet-test-123" {
		t.Errorf("Expected Subnetid 'subnet-test-123', got %q", cache.Subnetid)
	}
}

func TestCreateSubnet_Error(t *testing.T) {
	expectedErr := errors.New("subnet creation failed")
	mock := &extendedMockEC2Client{
		createSubnetErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createSubnet(cache)
	if err == nil {
		t.Fatal("Expected error when CreateSubnet fails")
	}

	if !contains(err.Error(), "error creating subnet") {
		t.Errorf("Expected error to contain 'error creating subnet', got: %v", err)
	}
}

func TestCreateInternetGateway_Success(t *testing.T) {
	mock := &extendedMockEC2Client{}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createInternetGateway(cache)
	if err != nil {
		t.Fatalf("createInternetGateway failed: %v", err)
	}

	// Verify Internet Gateway was created
	if len(mock.createInternetGatewayCalls) == 0 {
		t.Fatal("CreateInternetGateway was not called")
	}

	// Verify Internet Gateway was attached
	if len(mock.attachInternetGatewayCalls) == 0 {
		t.Fatal("AttachInternetGateway was not called")
	}

	attachCall := mock.attachInternetGatewayCalls[0]
	if attachCall.VpcId == nil || *attachCall.VpcId != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", attachCall.VpcId)
	}
	if attachCall.InternetGatewayId == nil || *attachCall.InternetGatewayId != "igw-test-123" {
		t.Errorf("Expected InternetGatewayId 'igw-test-123', got %v", attachCall.InternetGatewayId)
	}

	// Verify Internet Gateway ID was set in cache
	if cache.InternetGwid != "igw-test-123" {
		t.Errorf("Expected InternetGwid 'igw-test-123', got %q", cache.InternetGwid)
	}
}

func TestCreateInternetGateway_CreateError(t *testing.T) {
	expectedErr := errors.New("Internet Gateway creation failed")
	mock := &extendedMockEC2Client{
		createInternetGatewayErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createInternetGateway(cache)
	if err == nil {
		t.Fatal("Expected error when CreateInternetGateway fails")
	}

	if !contains(err.Error(), "error creating Internet Gateway") {
		t.Errorf("Expected error to contain 'error creating Internet Gateway', got: %v", err)
	}
}

func TestCreateInternetGateway_AttachError(t *testing.T) {
	expectedErr := errors.New("attach Internet Gateway failed")
	mock := &extendedMockEC2Client{
		attachInternetGatewayErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createInternetGateway(cache)
	if err == nil {
		t.Fatal("Expected error when AttachInternetGateway fails")
	}

	if !contains(err.Error(), "error attaching Internet Gateway") {
		t.Errorf("Expected error to contain 'error attaching Internet Gateway', got: %v", err)
	}
}

func TestCreateRouteTable_Success(t *testing.T) {
	mock := &extendedMockEC2Client{}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid:        "vpc-test-123",
		Subnetid:     "subnet-test-123",
		InternetGwid: "igw-test-123",
	}

	err := provider.createRouteTable(cache)
	if err != nil {
		t.Fatalf("createRouteTable failed: %v", err)
	}

	// Verify Route Table was created
	if len(mock.createRouteTableCalls) == 0 {
		t.Fatal("CreateRouteTable was not called")
	}

	createCall := mock.createRouteTableCalls[0]
	if createCall.VpcId == nil || *createCall.VpcId != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", createCall.VpcId)
	}

	// Verify Route Table was associated with subnet
	if len(mock.associateRouteTableCalls) == 0 {
		t.Fatal("AssociateRouteTable was not called")
	}

	assocCall := mock.associateRouteTableCalls[0]
	if assocCall.SubnetId == nil || *assocCall.SubnetId != "subnet-test-123" {
		t.Errorf("Expected SubnetId 'subnet-test-123', got %v", assocCall.SubnetId)
	}

	// Verify route was created
	if len(mock.createRouteCalls) == 0 {
		t.Fatal("CreateRoute was not called")
	}

	routeCall := mock.createRouteCalls[0]
	if routeCall.DestinationCidrBlock == nil || *routeCall.DestinationCidrBlock != "0.0.0.0/0" {
		t.Errorf("Expected DestinationCidrBlock '0.0.0.0/0', got %v", routeCall.DestinationCidrBlock)
	}
	if routeCall.GatewayId == nil || *routeCall.GatewayId != "igw-test-123" {
		t.Errorf("Expected GatewayId 'igw-test-123', got %v", routeCall.GatewayId)
	}

	// Verify Route Table ID was set in cache
	if cache.RouteTable != "rtb-test-123" {
		t.Errorf("Expected RouteTable 'rtb-test-123', got %q", cache.RouteTable)
	}
}

func TestCreateRouteTable_CreateError(t *testing.T) {
	expectedErr := errors.New("route table creation failed")
	mock := &extendedMockEC2Client{
		createRouteTableErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid:        "vpc-test-123",
		Subnetid:     "subnet-test-123",
		InternetGwid: "igw-test-123",
	}

	err := provider.createRouteTable(cache)
	if err == nil {
		t.Fatal("Expected error when CreateRouteTable fails")
	}

	if !contains(err.Error(), "error creating route table") {
		t.Errorf("Expected error to contain 'error creating route table', got: %v", err)
	}
}

func TestCreateRouteTable_AssociateError(t *testing.T) {
	expectedErr := errors.New("associate route table failed")
	mock := &extendedMockEC2Client{
		associateRouteTableErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid:        "vpc-test-123",
		Subnetid:     "subnet-test-123",
		InternetGwid: "igw-test-123",
	}

	err := provider.createRouteTable(cache)
	if err == nil {
		t.Fatal("Expected error when AssociateRouteTable fails")
	}

	if !contains(err.Error(), "error associating route table") {
		t.Errorf("Expected error to contain 'error associating route table', got: %v", err)
	}
}

func TestCreateRouteTable_CreateRouteError(t *testing.T) {
	expectedErr := errors.New("create route failed")
	mock := &extendedMockEC2Client{
		createRouteErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid:        "vpc-test-123",
		Subnetid:     "subnet-test-123",
		InternetGwid: "igw-test-123",
	}

	err := provider.createRouteTable(cache)
	if err == nil {
		t.Fatal("Expected error when CreateRoute fails")
	}

	if !contains(err.Error(), "error creating route") {
		t.Errorf("Expected error to contain 'error creating route', got: %v", err)
	}
}

func TestCreateSecurityGroup_Success(t *testing.T) {
	t.Skip("Skipping: requires network access for GetIPAddress()")
	mock := &extendedMockEC2Client{}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createSecurityGroup(cache)
	if err != nil {
		t.Fatalf("createSecurityGroup failed: %v", err)
	}

	// Verify Security Group was created
	if len(mock.createSecurityGroupCalls) == 0 {
		t.Fatal("CreateSecurityGroup was not called")
	}

	createCall := mock.createSecurityGroupCalls[0]
	if createCall.VpcId == nil || *createCall.VpcId != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", createCall.VpcId)
	}
	if createCall.GroupName == nil || *createCall.GroupName != "test-env" {
		t.Errorf("Expected GroupName 'test-env', got %v", createCall.GroupName)
	}

	// Verify Security Group ingress was authorized
	if len(mock.authorizeSecurityGroupCalls) == 0 {
		t.Fatal("AuthorizeSecurityGroupIngress was not called")
	}

	authCall := mock.authorizeSecurityGroupCalls[0]
	if authCall.GroupId == nil || *authCall.GroupId != "sg-test-123" {
		t.Errorf("Expected GroupId 'sg-test-123', got %v", authCall.GroupId)
	}

	// Verify Security Group ID was set in cache
	if cache.SecurityGroupid != "sg-test-123" {
		t.Errorf("Expected SecurityGroupid 'sg-test-123', got %q", cache.SecurityGroupid)
	}
}

func TestCreateSecurityGroup_CreateError(t *testing.T) {
	t.Skip("Skipping: requires network access for GetIPAddress()")
	expectedErr := errors.New("security group creation failed")
	mock := &extendedMockEC2Client{
		createSecurityGroupErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createSecurityGroup(cache)
	if err == nil {
		t.Fatal("Expected error when CreateSecurityGroup fails")
	}

	if !contains(err.Error(), "error creating security group") {
		t.Errorf("Expected error to contain 'error creating security group', got: %v", err)
	}
}

func TestCreateSecurityGroup_AuthorizeError(t *testing.T) {
	t.Skip("Skipping: requires network access for GetIPAddress()")
	expectedErr := errors.New("authorize security group failed")
	mock := &extendedMockEC2Client{
		authorizeSecurityGroupErr: expectedErr,
	}
	provider := createTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createSecurityGroup(cache)
	if err == nil {
		t.Fatal("Expected error when AuthorizeSecurityGroupIngress fails")
	}

	if !contains(err.Error(), "error authorizing security group ingress") {
		t.Errorf("Expected error to contain 'error authorizing security group ingress', got: %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
