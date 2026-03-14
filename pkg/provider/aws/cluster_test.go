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
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

// TestCreateInstancesSetsNoPublicIP verifies that createInstances sets
// AssociatePublicIpAddress=false in the RunInstancesInput for cluster mode.
func TestCreateInstancesSetsNoPublicIP(t *testing.T) {
	var mu sync.Mutex
	var captured []*ec2.RunInstancesInput

	mock := NewMockEC2Client()

	// Capture RunInstances calls
	mock.RunInstancesFunc = func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
		mu.Lock()
		captured = append(captured, params)
		mu.Unlock()
		return &ec2.RunInstancesOutput{
			Instances: []types.Instance{
				{
					InstanceId:       aws.String("i-test-12345"),
					PrivateIpAddress: aws.String("10.0.0.10"),
					NetworkInterfaces: []types.InstanceNetworkInterface{
						{NetworkInterfaceId: aws.String("eni-test-12345")},
					},
				},
			},
		}, nil
	}

	// Mock DescribeInstances for the waiter — return instance in "running" state
	mock.DescribeInstsFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
		return &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId:       aws.String("i-test-12345"),
							State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
							PublicDnsName:    aws.String(""),
							PublicIpAddress:  nil,
							PrivateIpAddress: aws.String("10.0.0.10"),
							NetworkInterfaces: []types.InstanceNetworkInterface{
								{NetworkInterfaceId: aws.String("eni-test-12345")},
							},
						},
					},
				},
			},
		}, nil
	}

	// Mock DescribeImages for resolveImageForNode and describeImageRootDevice
	mock.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		return &ec2.DescribeImagesOutput{
			Images: []types.Image{
				{
					ImageId:        aws.String("ami-test-123"),
					Architecture:   types.ArchitectureValuesX8664,
					RootDeviceName: aws.String("/dev/sda1"),
				},
			},
		}, nil
	}

	provider := newTestProvider(mock)
	cache := &ClusterCache{
		AWS: AWS{
			Subnetid:              "subnet-private",
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

	// Verify no public IP
	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 1 {
		t.Fatalf("expected 1 RunInstances call, got %d", len(captured))
	}

	nis := captured[0].NetworkInterfaces
	if len(nis) == 0 {
		t.Fatal("expected NetworkInterfaces in RunInstancesInput")
	}
	if nis[0].AssociatePublicIpAddress == nil || *nis[0].AssociatePublicIpAddress != false {
		t.Error("AssociatePublicIpAddress should be false for cluster instances")
	}

	// Verify instance uses private subnet
	if aws.ToString(nis[0].SubnetId) != "subnet-private" {
		t.Errorf("SubnetId = %q, want %q", aws.ToString(nis[0].SubnetId), "subnet-private")
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
			var mu sync.Mutex
			var captured *ec2.RunInstancesInput

			mock := NewMockEC2Client()
			mock.RunInstancesFunc = func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
				mu.Lock()
				captured = params
				mu.Unlock()
				return &ec2.RunInstancesOutput{
					Instances: []types.Instance{
						{
							InstanceId:       aws.String("i-test"),
							PrivateIpAddress: aws.String("10.0.0.10"),
							NetworkInterfaces: []types.InstanceNetworkInterface{
								{NetworkInterfaceId: aws.String("eni-test")},
							},
						},
					},
				}, nil
			}
			mock.DescribeInstsFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{{
						Instances: []types.Instance{{
							InstanceId:       aws.String("i-test"),
							State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
							PrivateIpAddress: aws.String("10.0.0.10"),
							NetworkInterfaces: []types.InstanceNetworkInterface{
								{NetworkInterfaceId: aws.String("eni-test")},
							},
						}},
					}},
				}, nil
			}
			mock.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
				return &ec2.DescribeImagesOutput{
					Images: []types.Image{{
						ImageId:        aws.String("ami-test"),
						Architecture:   types.ArchitectureValuesX8664,
						RootDeviceName: aws.String("/dev/sda1"),
					}},
				}, nil
			}

			provider := newTestProvider(mock)
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

			mu.Lock()
			defer mu.Unlock()
			if captured == nil {
				t.Fatal("RunInstances was not called")
			}
			gotSG := captured.NetworkInterfaces[0].Groups[0]
			if gotSG != tt.wantSG {
				t.Errorf("SecurityGroup = %q, want %q", gotSG, tt.wantSG)
			}
		})
	}
}

// TestPrivateRouteTableRoutesToNATGW verifies that createPrivateRouteTable
// routes 0.0.0.0/0 to the NAT Gateway (not the Internet Gateway).
func TestPrivateRouteTableRoutesToNATGW(t *testing.T) {
	var capturedRoute *ec2.CreateRouteInput

	mock := NewMockEC2Client()
	mock.CreateRouteFunc = func(ctx context.Context, params *ec2.CreateRouteInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
		capturedRoute = params
		return &ec2.CreateRouteOutput{}, nil
	}

	provider := newTestProvider(mock)
	cache := &AWS{
		Vpcid:        "vpc-test",
		Subnetid:     "subnet-private",
		NatGatewayid: "nat-test-123",
		InternetGwid: "igw-test-456",
	}

	if err := provider.createPrivateRouteTable(cache); err != nil {
		t.Fatalf("createPrivateRouteTable failed: %v", err)
	}

	if capturedRoute == nil {
		t.Fatal("CreateRoute was not called")
	}

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
	var capturedRoute *ec2.CreateRouteInput
	var capturedAssoc *ec2.AssociateRouteTableInput

	mock := NewMockEC2Client()
	mock.CreateRouteFunc = func(ctx context.Context, params *ec2.CreateRouteInput, optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
		capturedRoute = params
		return &ec2.CreateRouteOutput{}, nil
	}
	mock.AssociateRTFunc = func(ctx context.Context, params *ec2.AssociateRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error) {
		capturedAssoc = params
		return &ec2.AssociateRouteTableOutput{}, nil
	}

	provider := newTestProvider(mock)
	cache := &AWS{
		Vpcid:          "vpc-test",
		PublicSubnetid: "subnet-public",
		InternetGwid:   "igw-test-456",
	}

	if err := provider.createPublicRouteTable(cache); err != nil {
		t.Fatalf("createPublicRouteTable failed: %v", err)
	}

	// Verify route targets IGW
	if capturedRoute == nil {
		t.Fatal("CreateRoute was not called")
	}
	if capturedRoute.GatewayId == nil || *capturedRoute.GatewayId != "igw-test-456" {
		t.Errorf("Route should target IGW igw-test-456, got GatewayId=%v", aws.ToString(capturedRoute.GatewayId))
	}

	// Verify association with public subnet (not private)
	if capturedAssoc == nil {
		t.Fatal("AssociateRouteTable was not called")
	}
	if aws.ToString(capturedAssoc.SubnetId) != "subnet-public" {
		t.Errorf("Route table associated with %q, want public subnet %q",
			aws.ToString(capturedAssoc.SubnetId), "subnet-public")
	}
}

// TestPublicSubnetCreatedInCorrectCIDR verifies that createPublicSubnet
// creates a subnet in the 10.0.1.0/24 CIDR and stores it in PublicSubnetid.
func TestPublicSubnetCreatedInCorrectCIDR(t *testing.T) {
	var capturedSubnet *ec2.CreateSubnetInput

	mock := NewMockEC2Client()
	mock.CreateSubnetFunc = func(ctx context.Context, params *ec2.CreateSubnetInput, optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
		capturedSubnet = params
		return &ec2.CreateSubnetOutput{
			Subnet: &types.Subnet{SubnetId: aws.String("subnet-public-123")},
		}, nil
	}

	provider := newTestProvider(mock)
	cache := &AWS{
		Vpcid: "vpc-test",
	}

	if err := provider.createPublicSubnet(cache); err != nil {
		t.Fatalf("createPublicSubnet failed: %v", err)
	}

	if capturedSubnet == nil {
		t.Fatal("CreateSubnet was not called")
	}
	if aws.ToString(capturedSubnet.CidrBlock) != "10.0.1.0/24" {
		t.Errorf("Public subnet CIDR = %q, want %q", aws.ToString(capturedSubnet.CidrBlock), "10.0.1.0/24")
	}

	// Verify stored in PublicSubnetid (not Subnetid)
	if cache.PublicSubnetid != "subnet-public-123" {
		t.Errorf("cache.PublicSubnetid = %q, want %q", cache.PublicSubnetid, "subnet-public-123")
	}
}

// TestNATGatewayCreatedInPublicSubnet verifies that createNATGateway
// places the NAT gateway in the public subnet.
func TestNATGatewayCreatedInPublicSubnet(t *testing.T) {
	var capturedNAT *ec2.CreateNatGatewayInput

	mock := NewMockEC2Client()
	mock.CreateNatGatewayFunc = func(ctx context.Context, params *ec2.CreateNatGatewayInput, optFns ...func(*ec2.Options)) (*ec2.CreateNatGatewayOutput, error) {
		capturedNAT = params
		return &ec2.CreateNatGatewayOutput{
			NatGateway: &types.NatGateway{
				NatGatewayId: aws.String("nat-test-123"),
				State:        types.NatGatewayStateAvailable,
			},
		}, nil
	}
	// Mock DescribeNatGateways for the wait loop
	mock.DescribeNatGatewaysFunc = func(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
		return &ec2.DescribeNatGatewaysOutput{
			NatGateways: []types.NatGateway{
				{
					NatGatewayId: aws.String("nat-test-123"),
					State:        types.NatGatewayStateAvailable,
				},
			},
		}, nil
	}

	provider := newTestProvider(mock)
	cache := &AWS{
		Vpcid:          "vpc-test",
		PublicSubnetid: "subnet-public",
		Subnetid:       "subnet-private",
	}

	if err := provider.createNATGateway(cache); err != nil {
		t.Fatalf("createNATGateway failed: %v", err)
	}

	if capturedNAT == nil {
		t.Fatal("CreateNatGateway was not called")
	}
	if aws.ToString(capturedNAT.SubnetId) != "subnet-public" {
		t.Errorf("NAT GW placed in %q, want public subnet %q",
			aws.ToString(capturedNAT.SubnetId), "subnet-public")
	}
}
