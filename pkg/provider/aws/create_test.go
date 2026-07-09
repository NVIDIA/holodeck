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
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	internalaws "github.com/NVIDIA/holodeck/internal/aws"
	"github.com/NVIDIA/holodeck/internal/aws/awsfake"
	"github.com/NVIDIA/holodeck/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockLogger creates a logger for testing.
func mockLogger() *logger.FunLogger {
	log := &logger.FunLogger{
		Out:  io.Discard,
		Wg:   &sync.WaitGroup{},
		IsCI: true, // Prevents interactive terminal behavior
	}
	return log
}

// onlyID returns the sole key of a one-element store map, failing the test
// otherwise. It cross-checks that the provider stored the fake-generated ID of
// the single resource it created (IDs are dynamic, unlike the old canned mock).
func onlyID[V any](t *testing.T, m map[string]V, kind string) string {
	t.Helper()
	if len(m) != 1 {
		t.Fatalf("expected exactly 1 %s in store, got %d", kind, len(m))
	}
	for id := range m {
		return id
	}
	return ""
}

func TestCreateEC2Instance_DisablesSourceDestCheck(t *testing.T) {
	f := awsfake.New()
	seedTestImage(f, "ami-123")

	provider := &Provider{
		ec2:   f.EC2,
		log:   mockLogger(),
		sleep: noopSleep,
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
	modCalls := f.Store.Inputs("ModifyNetworkInterfaceAttribute")
	if len(modCalls) == 0 {
		t.Fatal("ModifyNetworkInterfaceAttribute was not called - Source/Dest Check not disabled")
	}

	call := modCalls[0].(*ec2.ModifyNetworkInterfaceAttributeInput)

	// Verify it targets the instance's network interface (the one the fake created)
	eniID := onlyID(t, f.Store.NetworkInterfaces, "network interface")
	if call.NetworkInterfaceId == nil || *call.NetworkInterfaceId != eniID {
		t.Errorf("Expected NetworkInterfaceId %q, got %v", eniID, aws.ToString(call.NetworkInterfaceId))
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
	f := awsfake.New()
	seedTestImage(f, "ami-123")
	f.Store.FailNext("ModifyNetworkInterfaceAttribute", expectedErr)

	provider := &Provider{
		ec2:   f.EC2,
		log:   mockLogger(),
		sleep: noopSleep,
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

// Helper to create a test provider
func createTestProvider(client internalaws.EC2Client) *Provider {
	return &Provider{
		ec2:   client,
		log:   mockLogger(),
		sleep: noopSleep,
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
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{}

	err := provider.createVPC(cache)
	if err != nil {
		t.Fatalf("createVPC failed: %v", err)
	}

	// Verify VPC was created and its ID stored in the cache.
	if f.Store.CallsTo("CreateVpc") == 0 {
		t.Fatal("CreateVpc was not called")
	}
	vpcID := onlyID(t, f.Store.Vpcs, "VPC")
	if cache.Vpcid != vpcID {
		t.Errorf("Expected Vpcid %q, got %q", vpcID, cache.Vpcid)
	}

	// Verify ModifyVpcAttribute was called to enable DNS hostnames on the VPC.
	modCalls := f.Store.Inputs("ModifyVpcAttribute")
	if len(modCalls) == 0 {
		t.Fatal("ModifyVpcAttribute was not called")
	}
	modCall := modCalls[0].(*ec2.ModifyVpcAttributeInput)
	if aws.ToString(modCall.VpcId) != vpcID {
		t.Errorf("Expected VpcId %q, got %v", vpcID, modCall.VpcId)
	}
	if modCall.EnableDnsHostnames == nil || modCall.EnableDnsHostnames.Value == nil || !*modCall.EnableDnsHostnames.Value {
		t.Error("Expected EnableDnsHostnames to be true")
	}
}

func TestCreateVPC_CreateVpcError(t *testing.T) {
	expectedErr := errors.New("VPC creation failed")
	f := awsfake.New()
	f.Store.FailNext("CreateVpc", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	f.Store.FailNext("ModifyVpcAttribute", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createSubnet(cache)
	if err != nil {
		t.Fatalf("createSubnet failed: %v", err)
	}

	// Verify subnet was created with the given VPC ID.
	subnetCalls := f.Store.Inputs("CreateSubnet")
	if len(subnetCalls) == 0 {
		t.Fatal("CreateSubnet was not called")
	}
	call := subnetCalls[0].(*ec2.CreateSubnetInput)
	if aws.ToString(call.VpcId) != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", call.VpcId)
	}

	// Verify subnet ID was set in cache
	subnetID := onlyID(t, f.Store.Subnets, "subnet")
	if cache.Subnetid != subnetID {
		t.Errorf("Expected Subnetid %q, got %q", subnetID, cache.Subnetid)
	}
}

func TestCreateSubnet_Error(t *testing.T) {
	expectedErr := errors.New("subnet creation failed")
	f := awsfake.New()
	f.Store.FailNext("CreateSubnet", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createInternetGateway(cache)
	if err != nil {
		t.Fatalf("createInternetGateway failed: %v", err)
	}

	// Verify Internet Gateway was created and its ID stored in the cache.
	if f.Store.CallsTo("CreateInternetGateway") == 0 {
		t.Fatal("CreateInternetGateway was not called")
	}
	igwID := onlyID(t, f.Store.InternetGateways, "internet gateway")
	if cache.InternetGwid != igwID {
		t.Errorf("Expected InternetGwid %q, got %q", igwID, cache.InternetGwid)
	}

	// Verify Internet Gateway was attached to the VPC using its created ID.
	attachCalls := f.Store.Inputs("AttachInternetGateway")
	if len(attachCalls) == 0 {
		t.Fatal("AttachInternetGateway was not called")
	}
	attachCall := attachCalls[0].(*ec2.AttachInternetGatewayInput)
	if aws.ToString(attachCall.VpcId) != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", attachCall.VpcId)
	}
	if aws.ToString(attachCall.InternetGatewayId) != igwID {
		t.Errorf("Expected InternetGatewayId %q, got %v", igwID, attachCall.InternetGatewayId)
	}
}

func TestCreateInternetGateway_CreateError(t *testing.T) {
	expectedErr := errors.New("Internet Gateway creation failed")
	f := awsfake.New()
	f.Store.FailNext("CreateInternetGateway", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	f.Store.FailNext("AttachInternetGateway", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		Vpcid:        "vpc-test-123",
		Subnetid:     "subnet-test-123",
		InternetGwid: "igw-test-123",
	}

	err := provider.createRouteTable(cache)
	if err != nil {
		t.Fatalf("createRouteTable failed: %v", err)
	}

	// Verify Route Table was created with the given VPC ID.
	createCalls := f.Store.Inputs("CreateRouteTable")
	if len(createCalls) == 0 {
		t.Fatal("CreateRouteTable was not called")
	}
	createCall := createCalls[0].(*ec2.CreateRouteTableInput)
	if aws.ToString(createCall.VpcId) != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", createCall.VpcId)
	}

	// Verify Route Table was associated with subnet
	assocCalls := f.Store.Inputs("AssociateRouteTable")
	if len(assocCalls) == 0 {
		t.Fatal("AssociateRouteTable was not called")
	}
	assocCall := assocCalls[0].(*ec2.AssociateRouteTableInput)
	if aws.ToString(assocCall.SubnetId) != "subnet-test-123" {
		t.Errorf("Expected SubnetId 'subnet-test-123', got %v", assocCall.SubnetId)
	}

	// Verify route was created
	routeCalls := f.Store.Inputs("CreateRoute")
	if len(routeCalls) == 0 {
		t.Fatal("CreateRoute was not called")
	}
	routeCall := routeCalls[0].(*ec2.CreateRouteInput)
	if aws.ToString(routeCall.DestinationCidrBlock) != "0.0.0.0/0" {
		t.Errorf("Expected DestinationCidrBlock '0.0.0.0/0', got %v", routeCall.DestinationCidrBlock)
	}
	if aws.ToString(routeCall.GatewayId) != "igw-test-123" {
		t.Errorf("Expected GatewayId 'igw-test-123', got %v", routeCall.GatewayId)
	}

	// Verify Route Table ID was set in cache
	rtID := onlyID(t, f.Store.RouteTables, "route table")
	if cache.RouteTable != rtID {
		t.Errorf("Expected RouteTable %q, got %q", rtID, cache.RouteTable)
	}
}

func TestCreateRouteTable_CreateError(t *testing.T) {
	expectedErr := errors.New("route table creation failed")
	f := awsfake.New()
	f.Store.FailNext("CreateRouteTable", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	f.Store.FailNext("AssociateRouteTable", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	f.Store.FailNext("CreateRoute", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createSecurityGroup(cache)
	if err != nil {
		t.Fatalf("createSecurityGroup failed: %v", err)
	}

	// Verify Security Group was created
	createCalls := f.Store.Inputs("CreateSecurityGroup")
	if len(createCalls) == 0 {
		t.Fatal("CreateSecurityGroup was not called")
	}
	createCall := createCalls[0].(*ec2.CreateSecurityGroupInput)
	if aws.ToString(createCall.VpcId) != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", createCall.VpcId)
	}
	if aws.ToString(createCall.GroupName) != "test-env" {
		t.Errorf("Expected GroupName 'test-env', got %v", createCall.GroupName)
	}

	// Verify Security Group ingress was authorized against the created SG.
	sgID := onlyID(t, f.Store.SecurityGroups, "security group")
	authCalls := f.Store.Inputs("AuthorizeSecurityGroupIngress")
	if len(authCalls) == 0 {
		t.Fatal("AuthorizeSecurityGroupIngress was not called")
	}
	authCall := authCalls[0].(*ec2.AuthorizeSecurityGroupIngressInput)
	if aws.ToString(authCall.GroupId) != sgID {
		t.Errorf("Expected GroupId %q, got %v", sgID, authCall.GroupId)
	}

	// Verify Security Group ID was set in cache
	if cache.SecurityGroupid != sgID {
		t.Errorf("Expected SecurityGroupid %q, got %q", sgID, cache.SecurityGroupid)
	}
}

func TestCreateSecurityGroup_CreateError(t *testing.T) {
	t.Skip("Skipping: requires network access for GetIPAddress()")
	expectedErr := errors.New("security group creation failed")
	f := awsfake.New()
	f.Store.FailNext("CreateSecurityGroup", expectedErr)
	provider := createTestProvider(f.EC2)
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
	f := awsfake.New()
	f.Store.FailNext("AuthorizeSecurityGroupIngress", expectedErr)
	provider := createTestProvider(f.EC2)
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

func TestCreate_RejectsUnsupportedInstanceType(t *testing.T) {
	f := awsfake.New()
	provider := &Provider{
		ec2:   f.EC2,
		log:   mockLogger(),
		sleep: noopSleep,
		Environment: &v1alpha1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-env",
			},
			Spec: v1alpha1.EnvironmentSpec{
				Auth: v1alpha1.Auth{
					KeyName: "test-key",
				},
				Instance: v1alpha1.Instance{
					// A type absent from the region's instance-type catalog
					// (the fake seeds only the types the test configs use), so
					// checkInstanceTypes must reject it before any resource is created.
					Type:   "p4d.24xlarge",
					Region: "us-west-1",
					Image: v1alpha1.Image{
						ImageId: aws.String("ami-123"),
					},
				},
			},
		},
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("test")},
		},
	}

	err := provider.Create()

	// Must fail before creating any resources
	if err == nil {
		t.Fatal("Expected Create() to fail for unsupported instance type")
	}
	if !contains(err.Error(), "instance type") {
		t.Errorf("Expected error to mention 'instance type', got: %v", err)
	}
	if !contains(err.Error(), "not supported") {
		t.Errorf("Expected error to mention 'not supported', got: %v", err)
	}

	// Verify no VPC was created (fail-fast, no leaked resources)
	if f.Store.CallsTo("CreateVpc") != 0 {
		t.Error("VPC was created despite unsupported instance type — resources leaked")
	}
}

func TestCreatePublicSubnet_Success(t *testing.T) {
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createPublicSubnet(cache)
	if err != nil {
		t.Fatalf("createPublicSubnet failed: %v", err)
	}

	// Verify subnet was created with correct CIDR
	subnetCalls := f.Store.Inputs("CreateSubnet")
	if len(subnetCalls) == 0 {
		t.Fatal("CreateSubnet was not called")
	}
	call := subnetCalls[0].(*ec2.CreateSubnetInput)
	if aws.ToString(call.CidrBlock) != "10.0.1.0/24" {
		t.Errorf("Expected CidrBlock '10.0.1.0/24', got %v", call.CidrBlock)
	}
	if aws.ToString(call.VpcId) != "vpc-test-123" {
		t.Errorf("Expected VpcId 'vpc-test-123', got %v", call.VpcId)
	}

	// Verify public subnet ID was set in cache
	subnetID := onlyID(t, f.Store.Subnets, "subnet")
	if cache.PublicSubnetid != subnetID {
		t.Errorf("Expected PublicSubnetid %q, got %q", subnetID, cache.PublicSubnetid)
	}

	// ModifySubnetAttribute should NOT be called — NAT GW gets its public IP from the EIP,
	// not from MapPublicIpOnLaunch. No instances are launched in the public subnet.
	if f.Store.CallsTo("ModifySubnetAttribute") != 0 {
		t.Error("ModifySubnetAttribute should not be called on the public subnet")
	}
}

func TestCreatePublicSubnet_Error(t *testing.T) {
	expectedErr := errors.New("public subnet creation failed")
	f := awsfake.New()
	f.Store.FailNext("CreateSubnet", expectedErr)
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		Vpcid: "vpc-test-123",
	}

	err := provider.createPublicSubnet(cache)
	if err == nil {
		t.Fatal("Expected error when CreateSubnet fails")
	}

	if !contains(err.Error(), "error creating public subnet") {
		t.Errorf("Expected error to contain 'error creating public subnet', got: %v", err)
	}
}

func TestCreateNATGateway_Success(t *testing.T) {
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		PublicSubnetid: "subnet-pub-123",
	}

	err := provider.createNATGateway(cache)
	if err != nil {
		t.Fatalf("createNATGateway failed: %v", err)
	}

	// Verify EIP was allocated
	if f.Store.CallsTo("AllocateAddress") == 0 {
		t.Fatal("AllocateAddress was not called")
	}
	eipID := onlyID(t, f.Store.Addresses, "elastic IP")

	// Verify NAT Gateway was created in the public subnet using the allocated EIP.
	natCalls := f.Store.Inputs("CreateNatGateway")
	if len(natCalls) == 0 {
		t.Fatal("CreateNatGateway was not called")
	}
	natCall := natCalls[0].(*ec2.CreateNatGatewayInput)
	if aws.ToString(natCall.SubnetId) != "subnet-pub-123" {
		t.Errorf("Expected SubnetId 'subnet-pub-123', got %v", natCall.SubnetId)
	}
	if aws.ToString(natCall.AllocationId) != eipID {
		t.Errorf("Expected AllocationId %q, got %v", eipID, natCall.AllocationId)
	}

	// Verify cache was updated
	natID := onlyID(t, f.Store.NatGateways, "NAT gateway")
	if cache.NatGatewayid != natID {
		t.Errorf("Expected NatGatewayid %q, got %q", natID, cache.NatGatewayid)
	}
	if cache.EIPAllocationid != eipID {
		t.Errorf("Expected EIPAllocationid %q, got %q", eipID, cache.EIPAllocationid)
	}
}

func TestCreateNATGateway_AllocateAddressError(t *testing.T) {
	expectedErr := errors.New("allocate address failed")
	f := awsfake.New()
	f.Store.FailNext("AllocateAddress", expectedErr)
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		PublicSubnetid: "subnet-pub-123",
	}

	err := provider.createNATGateway(cache)
	if err == nil {
		t.Fatal("Expected error when AllocateAddress fails")
	}

	if !contains(err.Error(), "error allocating EIP") {
		t.Errorf("Expected error to contain 'error allocating EIP', got: %v", err)
	}
}

func TestCreateNATGateway_CreateNatGatewayError_CleansUpEIP(t *testing.T) {
	expectedErr := errors.New("NAT gateway creation failed")
	f := awsfake.New()
	f.Store.FailNext("CreateNatGateway", expectedErr)
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		PublicSubnetid: "subnet-pub-123",
	}

	err := provider.createNATGateway(cache)
	if err == nil {
		t.Fatal("Expected error when CreateNatGateway fails")
	}

	if !contains(err.Error(), "error creating NAT gateway") {
		t.Errorf("Expected error to contain 'error creating NAT gateway', got: %v", err)
	}

	// D4: the EIP allocated for the NAT gateway must be released on failure, so
	// no address is left leaked in the store.
	if f.Store.CallsTo("ReleaseAddress") == 0 {
		t.Fatal("ReleaseAddress was not called after NAT gateway creation failure (D4 violated)")
	}
	if len(f.Store.Addresses) != 0 {
		t.Errorf("EIP was not released after NAT gateway creation failure, %d address(es) leaked", len(f.Store.Addresses))
	}
}

func TestCreatePublicRouteTable_Success(t *testing.T) {
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		Vpcid:          "vpc-test-123",
		PublicSubnetid: "subnet-pub-123",
		InternetGwid:   "igw-test-123",
	}

	err := provider.createPublicRouteTable(cache)
	if err != nil {
		t.Fatalf("createPublicRouteTable failed: %v", err)
	}

	// Verify Route Table was created
	if f.Store.CallsTo("CreateRouteTable") == 0 {
		t.Fatal("CreateRouteTable was not called")
	}

	// Verify association with public subnet
	assocCalls := f.Store.Inputs("AssociateRouteTable")
	if len(assocCalls) == 0 {
		t.Fatal("AssociateRouteTable was not called")
	}
	assocCall := assocCalls[0].(*ec2.AssociateRouteTableInput)
	if aws.ToString(assocCall.SubnetId) != "subnet-pub-123" {
		t.Errorf("Expected SubnetId 'subnet-pub-123', got %v", assocCall.SubnetId)
	}

	// Verify route points to IGW
	routeCalls := f.Store.Inputs("CreateRoute")
	if len(routeCalls) == 0 {
		t.Fatal("CreateRoute was not called")
	}
	routeCall := routeCalls[0].(*ec2.CreateRouteInput)
	if aws.ToString(routeCall.GatewayId) != "igw-test-123" {
		t.Errorf("Expected GatewayId 'igw-test-123', got %v", routeCall.GatewayId)
	}

	// Verify cache was updated
	rtID := onlyID(t, f.Store.RouteTables, "route table")
	if cache.PublicRouteTable != rtID {
		t.Errorf("Expected PublicRouteTable %q, got %q", rtID, cache.PublicRouteTable)
	}
}

func TestCreatePrivateRouteTable_Success(t *testing.T) {
	f := awsfake.New()
	provider := createTestProvider(f.EC2)
	cache := &AWS{
		Vpcid:        "vpc-test-123",
		Subnetid:     "subnet-priv-123",
		NatGatewayid: "nat-test-123",
	}

	err := provider.createPrivateRouteTable(cache)
	if err != nil {
		t.Fatalf("createPrivateRouteTable failed: %v", err)
	}

	// Verify Route Table was created
	if f.Store.CallsTo("CreateRouteTable") == 0 {
		t.Fatal("CreateRouteTable was not called")
	}

	// Verify association with private subnet
	assocCalls := f.Store.Inputs("AssociateRouteTable")
	if len(assocCalls) == 0 {
		t.Fatal("AssociateRouteTable was not called")
	}
	assocCall := assocCalls[0].(*ec2.AssociateRouteTableInput)
	if aws.ToString(assocCall.SubnetId) != "subnet-priv-123" {
		t.Errorf("Expected SubnetId 'subnet-priv-123', got %v", assocCall.SubnetId)
	}

	// Verify route points to NAT GW (not IGW)
	routeCalls := f.Store.Inputs("CreateRoute")
	if len(routeCalls) == 0 {
		t.Fatal("CreateRoute was not called")
	}
	routeCall := routeCalls[0].(*ec2.CreateRouteInput)
	if aws.ToString(routeCall.NatGatewayId) != "nat-test-123" {
		t.Errorf("Expected NatGatewayId 'nat-test-123', got %v", routeCall.NatGatewayId)
	}
	if routeCall.GatewayId != nil {
		t.Errorf("Expected GatewayId to be nil for private route, got %v", routeCall.GatewayId)
	}

	// Verify cache was updated
	rtID := onlyID(t, f.Store.RouteTables, "route table")
	if cache.RouteTable != rtID {
		t.Errorf("Expected RouteTable %q, got %q", rtID, cache.RouteTable)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
