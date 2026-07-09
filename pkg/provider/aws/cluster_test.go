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
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/aws/awsfake"
)

// seedTestImage seeds an available x86_64 image so resolveImageForNode and
// describeImageRootDevice resolve for an explicit spec.image.imageId.
func seedTestImage(f *awsfake.Fake, id string) {
	f.Store.SeedImage(types.Image{
		ImageId:        aws.String(id),
		Architecture:   types.ArchitectureValuesX8664,
		RootDeviceName: aws.String("/dev/sda1"),
		State:          types.ImageStateAvailable,
	})
}

// TestCreateInstancesSetsPublicIP verifies that createInstances sets
// AssociatePublicIpAddress=true in the RunInstancesInput for cluster mode.
func TestCreateInstancesSetsPublicIP(t *testing.T) {
	f := awsfake.New()
	seedTestImage(f, "ami-test-123")

	provider := newTestProvider(f.EC2)
	cache := &ClusterCache{
		AWS: AWS{
			Subnetid:              "subnet-private",
			PublicSubnetid:        "subnet-public",
			CPSecurityGroupid:     "sg-cp",
			WorkerSecurityGroupid: "sg-worker",
		},
	}

	instances, err := provider.createInstances(
		cache,
		1,
		NodeRoleControlPlane,
		"t3.medium",
		nil,
		"",
		&v1alpha1.Image{ImageId: aws.String("ami-test-123")},
	)
	if err != nil {
		t.Fatalf("createInstances failed: %v", err)
	}

	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	// Inspect the RunInstances arguments.
	runCalls := f.Store.Inputs("RunInstances")
	if len(runCalls) != 1 {
		t.Fatalf("expected 1 RunInstances call, got %d", len(runCalls))
	}
	nis := runCalls[0].(*ec2.RunInstancesInput).NetworkInterfaces
	if len(nis) == 0 {
		t.Fatal("expected NetworkInterfaces in RunInstancesInput")
	}
	if nis[0].AssociatePublicIpAddress == nil || !*nis[0].AssociatePublicIpAddress {
		t.Error("AssociatePublicIpAddress should be true for cluster instances")
	}

	// Verify instance uses public subnet
	if aws.ToString(nis[0].SubnetId) != "subnet-public" {
		t.Errorf("SubnetId = %q, want %q", aws.ToString(nis[0].SubnetId), "subnet-public")
	}
}

// TestCreateInstancesUsesRoleSecurityGroup verifies that createInstances selects
// the correct security group based on the node role.
func TestCreateInstancesUsesRoleSecurityGroup(t *testing.T) {
	tests := []struct {
		name   string
		role   NodeRole
		wantSG string
	}{
		{"control-plane uses CP SG", NodeRoleControlPlane, "sg-cp"},
		{"worker uses Worker SG", NodeRoleWorker, "sg-worker"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := awsfake.New()
			seedTestImage(f, "ami-test")

			provider := newTestProvider(f.EC2)
			cache := &ClusterCache{
				AWS: AWS{
					Subnetid:              "subnet-private",
					CPSecurityGroupid:     "sg-cp",
					WorkerSecurityGroupid: "sg-worker",
				},
			}

			_, err := provider.createInstances(cache, 1, tt.role, "t3.medium", nil, "", &v1alpha1.Image{ImageId: aws.String("ami-test")})
			if err != nil {
				t.Fatalf("createInstances failed: %v", err)
			}

			runCalls := f.Store.Inputs("RunInstances")
			if len(runCalls) != 1 {
				t.Fatalf("RunInstances was called %d times, want 1", len(runCalls))
			}
			gotSG := runCalls[0].(*ec2.RunInstancesInput).NetworkInterfaces[0].Groups[0]
			if gotSG != tt.wantSG {
				t.Errorf("SecurityGroup = %q, want %q", gotSG, tt.wantSG)
			}
		})
	}
}

// TestPrivateRouteTableRoutesToNATGW verifies that createPrivateRouteTable
// routes 0.0.0.0/0 to the NAT Gateway (not the Internet Gateway).
func TestPrivateRouteTableRoutesToNATGW(t *testing.T) {
	f := awsfake.New()

	provider := newTestProvider(f.EC2)
	cache := &AWS{
		Vpcid:        "vpc-test",
		Subnetid:     "subnet-private",
		NatGatewayid: "nat-test-123",
		InternetGwid: "igw-test-456",
	}

	if err := provider.createPrivateRouteTable(cache); err != nil {
		t.Fatalf("createPrivateRouteTable failed: %v", err)
	}

	routeCalls := f.Store.Inputs("CreateRoute")
	if len(routeCalls) == 0 {
		t.Fatal("CreateRoute was not called")
	}
	capturedRoute := routeCalls[0].(*ec2.CreateRouteInput)

	// Must route to NAT GW, not IGW
	if capturedRoute.NatGatewayId == nil || *capturedRoute.NatGatewayId != "nat-test-123" {
		t.Errorf("Route should target NAT GW nat-test-123, got NatGatewayId=%v GatewayId=%v",
			aws.ToString(capturedRoute.NatGatewayId), aws.ToString(capturedRoute.GatewayId))
	}
	if capturedRoute.GatewayId != nil {
		t.Error("Private route table should NOT route to IGW (GatewayId should be nil)")
	}
}

// TestPublicRouteTableRoutesToIGW verifies that createPublicRouteTable
// routes 0.0.0.0/0 to the Internet Gateway and associates with the public subnet.
func TestPublicRouteTableRoutesToIGW(t *testing.T) {
	f := awsfake.New()

	provider := newTestProvider(f.EC2)
	cache := &AWS{
		Vpcid:          "vpc-test",
		PublicSubnetid: "subnet-public",
		InternetGwid:   "igw-test-456",
	}

	if err := provider.createPublicRouteTable(cache); err != nil {
		t.Fatalf("createPublicRouteTable failed: %v", err)
	}

	// Verify route targets IGW
	routeCalls := f.Store.Inputs("CreateRoute")
	if len(routeCalls) == 0 {
		t.Fatal("CreateRoute was not called")
	}
	capturedRoute := routeCalls[0].(*ec2.CreateRouteInput)
	if capturedRoute.GatewayId == nil || *capturedRoute.GatewayId != "igw-test-456" {
		t.Errorf("Route should target IGW igw-test-456, got GatewayId=%v", aws.ToString(capturedRoute.GatewayId))
	}

	// Verify association with public subnet (not private)
	assocCalls := f.Store.Inputs("AssociateRouteTable")
	if len(assocCalls) == 0 {
		t.Fatal("AssociateRouteTable was not called")
	}
	capturedAssoc := assocCalls[0].(*ec2.AssociateRouteTableInput)
	if aws.ToString(capturedAssoc.SubnetId) != "subnet-public" {
		t.Errorf("Route table associated with %q, want public subnet %q",
			aws.ToString(capturedAssoc.SubnetId), "subnet-public")
	}
}

// TestPublicSubnetCreatedInCorrectCIDR verifies that createPublicSubnet
// creates a subnet in the 10.0.1.0/24 CIDR and stores it in PublicSubnetid.
func TestPublicSubnetCreatedInCorrectCIDR(t *testing.T) {
	f := awsfake.New()

	provider := newTestProvider(f.EC2)
	cache := &AWS{
		Vpcid: "vpc-test",
	}

	if err := provider.createPublicSubnet(cache); err != nil {
		t.Fatalf("createPublicSubnet failed: %v", err)
	}

	subnetCalls := f.Store.Inputs("CreateSubnet")
	if len(subnetCalls) == 0 {
		t.Fatal("CreateSubnet was not called")
	}
	if got := aws.ToString(subnetCalls[0].(*ec2.CreateSubnetInput).CidrBlock); got != "10.0.1.0/24" {
		t.Errorf("Public subnet CIDR = %q, want %q", got, "10.0.1.0/24")
	}

	// Verify the created subnet's ID was stored in PublicSubnetid (not Subnetid).
	if len(f.Store.Subnets) != 1 {
		t.Fatalf("expected exactly 1 subnet in store, got %d", len(f.Store.Subnets))
	}
	var createdID string
	for id := range f.Store.Subnets {
		createdID = id
	}
	if cache.PublicSubnetid != createdID {
		t.Errorf("cache.PublicSubnetid = %q, want created subnet %q", cache.PublicSubnetid, createdID)
	}
	if cache.Subnetid != "" {
		t.Errorf("cache.Subnetid = %q, want empty (public subnet must not overwrite it)", cache.Subnetid)
	}
}

// TestNATGatewayCreatedInPublicSubnet verifies that createNATGateway
// places the NAT gateway in the public subnet.
func TestNATGatewayCreatedInPublicSubnet(t *testing.T) {
	f := awsfake.New()

	provider := newTestProvider(f.EC2)
	cache := &AWS{
		Vpcid:          "vpc-test",
		PublicSubnetid: "subnet-public",
		Subnetid:       "subnet-private",
	}

	if err := provider.createNATGateway(cache); err != nil {
		t.Fatalf("createNATGateway failed: %v", err)
	}

	natCalls := f.Store.Inputs("CreateNatGateway")
	if len(natCalls) == 0 {
		t.Fatal("CreateNatGateway was not called")
	}
	if got := aws.ToString(natCalls[0].(*ec2.CreateNatGatewayInput).SubnetId); got != "subnet-public" {
		t.Errorf("NAT GW placed in %q, want public subnet %q", got, "subnet-public")
	}
}

// TestNATGatewayWaitsForAvailable verifies that createNATGateway polls
// DescribeNatGateways until the NAT GW transitions from pending to available.
func TestNATGatewayWaitsForAvailable(t *testing.T) {
	f := awsfake.New()
	// Report pending on the first observation, available on the second.
	f.Store.SeedNextNatGatewayState(1, types.NatGatewayStateAvailable)

	provider := newTestProvider(f.EC2)
	cache := &AWS{
		Vpcid:          "vpc-test",
		PublicSubnetid: "subnet-public",
		Subnetid:       "subnet-private",
	}

	if err := provider.createNATGateway(cache); err != nil {
		t.Fatalf("createNATGateway failed: %v", err)
	}

	if got := f.Store.CallsTo("DescribeNatGateways"); got < 2 {
		t.Errorf("Expected at least 2 DescribeNatGateways calls for polling, got %d", got)
	}
	// The provider records the created NAT gateway's ID in the cache.
	if len(f.Store.NatGateways) != 1 {
		t.Fatalf("expected exactly 1 NAT gateway in store, got %d", len(f.Store.NatGateways))
	}
	var createdID string
	for id := range f.Store.NatGateways {
		createdID = id
	}
	if cache.NatGatewayid != createdID {
		t.Errorf("cache.NatGatewayid = %q, want created NAT %q", cache.NatGatewayid, createdID)
	}
}

// TestNATGatewayFailedState verifies that createNATGateway returns an error
// if the NAT GW transitions to the failed state.
func TestNATGatewayFailedState(t *testing.T) {
	f := awsfake.New()
	// The NAT gateway reports failed immediately.
	f.Store.SeedNextNatGatewayState(0, types.NatGatewayStateFailed)

	provider := newTestProvider(f.EC2)
	cache := &AWS{
		Vpcid:          "vpc-test",
		PublicSubnetid: "subnet-public",
		Subnetid:       "subnet-private",
	}

	err := provider.createNATGateway(cache)
	if err == nil {
		t.Fatal("Expected error when NAT Gateway reaches failed state")
	}
	if !strings.Contains(err.Error(), "failed state") {
		t.Errorf("Error should mention failed state, got: %v", err)
	}
}
