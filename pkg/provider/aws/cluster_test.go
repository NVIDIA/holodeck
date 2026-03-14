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
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	internalaws "github.com/NVIDIA/holodeck/internal/aws"
)

// callTracker records the order of networking operations during CreateCluster.
type callTracker struct {
	mu    sync.Mutex
	calls []string
}

func (ct *callTracker) record(name string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.calls = append(ct.calls, name)
}

func (ct *callTracker) getCalls() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	cp := make([]string, len(ct.calls))
	copy(cp, ct.calls)
	return cp
}

// Compile-time check that mockELBv2Client satisfies internalaws.ELBv2Client.
var _ internalaws.ELBv2Client = (*mockELBv2Client)(nil)

// mockELBv2Client implements internalaws.ELBv2Client for testing.
type mockELBv2Client struct{}

func (m *mockELBv2Client) CreateLoadBalancer(_ context.Context, params *elasticloadbalancingv2.CreateLoadBalancerInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateLoadBalancerOutput, error) {
	return &elasticloadbalancingv2.CreateLoadBalancerOutput{
		LoadBalancers: []elbv2types.LoadBalancer{
			{
				LoadBalancerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/test-nlb/abc123"),
				DNSName:         aws.String("test-nlb-abc123.elb.us-east-1.amazonaws.com"),
			},
		},
	}, nil
}

func (m *mockELBv2Client) DescribeLoadBalancers(_ context.Context, _ *elasticloadbalancingv2.DescribeLoadBalancersInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	return &elasticloadbalancingv2.DescribeLoadBalancersOutput{}, nil
}

func (m *mockELBv2Client) DeleteLoadBalancer(_ context.Context, _ *elasticloadbalancingv2.DeleteLoadBalancerInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteLoadBalancerOutput, error) {
	return &elasticloadbalancingv2.DeleteLoadBalancerOutput{}, nil
}

func (m *mockELBv2Client) CreateTargetGroup(_ context.Context, _ *elasticloadbalancingv2.CreateTargetGroupInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateTargetGroupOutput, error) {
	return &elasticloadbalancingv2.CreateTargetGroupOutput{
		TargetGroups: []elbv2types.TargetGroup{
			{
				TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/test-tg/abc123"),
			},
		},
	}, nil
}

func (m *mockELBv2Client) DescribeTargetGroups(_ context.Context, _ *elasticloadbalancingv2.DescribeTargetGroupsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
	return &elasticloadbalancingv2.DescribeTargetGroupsOutput{}, nil
}

func (m *mockELBv2Client) DeleteTargetGroup(_ context.Context, _ *elasticloadbalancingv2.DeleteTargetGroupInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteTargetGroupOutput, error) {
	return &elasticloadbalancingv2.DeleteTargetGroupOutput{}, nil
}

func (m *mockELBv2Client) RegisterTargets(_ context.Context, _ *elasticloadbalancingv2.RegisterTargetsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.RegisterTargetsOutput, error) {
	return &elasticloadbalancingv2.RegisterTargetsOutput{}, nil
}

func (m *mockELBv2Client) DeregisterTargets(_ context.Context, _ *elasticloadbalancingv2.DeregisterTargetsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeregisterTargetsOutput, error) {
	return &elasticloadbalancingv2.DeregisterTargetsOutput{}, nil
}

func (m *mockELBv2Client) DescribeTargetHealth(_ context.Context, _ *elasticloadbalancingv2.DescribeTargetHealthInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
	return &elasticloadbalancingv2.DescribeTargetHealthOutput{}, nil
}

func (m *mockELBv2Client) CreateListener(_ context.Context, _ *elasticloadbalancingv2.CreateListenerInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateListenerOutput, error) {
	return &elasticloadbalancingv2.CreateListenerOutput{}, nil
}

func (m *mockELBv2Client) DescribeListeners(_ context.Context, _ *elasticloadbalancingv2.DescribeListenersInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
	return &elasticloadbalancingv2.DescribeListenersOutput{}, nil
}

func (m *mockELBv2Client) DeleteListener(_ context.Context, _ *elasticloadbalancingv2.DeleteListenerInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteListenerOutput, error) {
	return &elasticloadbalancingv2.DeleteListenerOutput{}, nil
}

func (m *mockELBv2Client) AddTags(_ context.Context, _ *elasticloadbalancingv2.AddTagsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.AddTagsOutput, error) {
	return &elasticloadbalancingv2.AddTagsOutput{}, nil
}

// TestCreateClusterNetworkingOrder verifies that CreateCluster calls networking
// functions in the correct order: public subnet -> public RT -> NAT GW -> private RT.
func TestCreateClusterNetworkingOrder(t *testing.T) {
	tracker := &callTracker{}
	subnetCallCount := 0

	mock := &MockEC2Client{
		// Track subnet creation calls (first is private, second is public)
		CreateSubnetFunc: func(_ context.Context, params *ec2.CreateSubnetInput, _ ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
			subnetCallCount++
			cidr := aws.ToString(params.CidrBlock)
			if cidr == "10.0.1.0/24" {
				tracker.record("createPublicSubnet")
				return &ec2.CreateSubnetOutput{
					Subnet: &types.Subnet{SubnetId: aws.String("subnet-public-12345")},
				}, nil
			}
			// Private subnet (10.0.0.0/24)
			return &ec2.CreateSubnetOutput{
				Subnet: &types.Subnet{SubnetId: aws.String("subnet-private-12345")},
			}, nil
		},
		// Track route table creation calls
		CreateRTFunc: func(_ context.Context, _ *ec2.CreateRouteTableInput, _ ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
			return &ec2.CreateRouteTableOutput{
				RouteTable: &types.RouteTable{RouteTableId: aws.String(fmt.Sprintf("rtb-mock-%d", subnetCallCount))},
			}, nil
		},
		// Track route creation to distinguish public vs private RT
		CreateRouteFunc: func(_ context.Context, params *ec2.CreateRouteInput, _ ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
			if params.NatGatewayId != nil {
				tracker.record("createPrivateRouteTable")
			} else if params.GatewayId != nil {
				// Only track after public subnet is created (first call is from createRouteTable)
				if subnetCallCount > 0 {
					tracker.record("createPublicRouteTable")
				}
			}
			return &ec2.CreateRouteOutput{}, nil
		},
		// Track NAT Gateway creation
		CreateNatGatewayFunc: func(_ context.Context, _ *ec2.CreateNatGatewayInput, _ ...func(*ec2.Options)) (*ec2.CreateNatGatewayOutput, error) {
			tracker.record("createNATGateway")
			return &ec2.CreateNatGatewayOutput{
				NatGateway: &types.NatGateway{NatGatewayId: aws.String("nat-mock-12345")},
			}, nil
		},
		// SG creation returns unique IDs
		CreateSGFunc: func() func(_ context.Context, params *ec2.CreateSecurityGroupInput, _ ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
			counter := 0
			return func(_ context.Context, params *ec2.CreateSecurityGroupInput, _ ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
				counter++
				return &ec2.CreateSecurityGroupOutput{
					GroupId: aws.String(fmt.Sprintf("sg-mock-%d", counter)),
				}, nil
			}
		}(),
		// RunInstances returns properly structured output
		RunInstancesFunc: func(_ context.Context, _ *ec2.RunInstancesInput, _ ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			return &ec2.RunInstancesOutput{
				Instances: []types.Instance{
					{
						InstanceId:       aws.String("i-mock-12345"),
						PrivateIpAddress: aws.String("10.0.0.10"),
						PublicIpAddress:   aws.String("54.0.0.1"),
						PublicDnsName:     aws.String("ec2-mock.compute.amazonaws.com"),
						NetworkInterfaces: []types.InstanceNetworkInterface{
							{NetworkInterfaceId: aws.String("eni-mock-12345")},
						},
					},
				},
			}, nil
		},
		DescribeInstsFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-mock-12345"),
								PrivateIpAddress: aws.String("10.0.0.10"),
								PublicIpAddress:   aws.String("54.0.0.1"),
								PublicDnsName:     aws.String("ec2-mock.compute.amazonaws.com"),
								NetworkInterfaces: []types.InstanceNetworkInterface{
									{NetworkInterfaceId: aws.String("eni-mock-12345")},
								},
							},
						},
					},
				},
			}, nil
		},
		DescribeImagesFunc: func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{
						ImageId:        aws.String("ami-mock-12345"),
						RootDeviceName: aws.String("/dev/sda1"),
						Architecture:   types.ArchitectureValuesX8664,
						CreationDate:   aws.String("2024-01-01T00:00:00.000Z"),
						Name:           aws.String("ubuntu-22.04"),
					},
				},
			}, nil
		},
	}

	p := newClusterTestProvider(mock, &mockELBv2Client{})

	err := p.CreateCluster()
	if err != nil {
		t.Fatalf("CreateCluster() returned error: %v", err)
	}

	calls := tracker.getCalls()

	// Verify the four networking operations were called
	expectedOrder := []string{
		"createPublicSubnet",
		"createPublicRouteTable",
		"createNATGateway",
		"createPrivateRouteTable",
	}

	if len(calls) < len(expectedOrder) {
		t.Fatalf("expected at least %d networking calls, got %d: %v", len(expectedOrder), len(calls), calls)
	}

	for i, expected := range expectedOrder {
		if i >= len(calls) || calls[i] != expected {
			t.Errorf("call[%d]: expected %q, got %q (full order: %v)", i, expected, calls[i], calls)
		}
	}
}

// TestCreateClusterNLBUsesPublicSubnet verifies the NLB is created in the public subnet.
func TestCreateClusterNLBUsesPublicSubnet(t *testing.T) {
	var capturedNLBSubnets []string

	mock := &MockEC2Client{
		CreateSubnetFunc: func(_ context.Context, params *ec2.CreateSubnetInput, _ ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
			cidr := aws.ToString(params.CidrBlock)
			if cidr == "10.0.1.0/24" {
				return &ec2.CreateSubnetOutput{
					Subnet: &types.Subnet{SubnetId: aws.String("subnet-public-99999")},
				}, nil
			}
			return &ec2.CreateSubnetOutput{
				Subnet: &types.Subnet{SubnetId: aws.String("subnet-private-99999")},
			}, nil
		},
		CreateSGFunc: func() func(_ context.Context, _ *ec2.CreateSecurityGroupInput, _ ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
			counter := 0
			return func(_ context.Context, _ *ec2.CreateSecurityGroupInput, _ ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
				counter++
				return &ec2.CreateSecurityGroupOutput{
					GroupId: aws.String(fmt.Sprintf("sg-mock-%d", counter)),
				}, nil
			}
		}(),
		RunInstancesFunc: func(_ context.Context, _ *ec2.RunInstancesInput, _ ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			return &ec2.RunInstancesOutput{
				Instances: []types.Instance{
					{
						InstanceId:       aws.String("i-mock-12345"),
						PrivateIpAddress: aws.String("10.0.0.10"),
						PublicIpAddress:   aws.String("54.0.0.1"),
						PublicDnsName:     aws.String("ec2-mock.compute.amazonaws.com"),
						NetworkInterfaces: []types.InstanceNetworkInterface{
							{NetworkInterfaceId: aws.String("eni-mock-12345")},
						},
					},
				},
			}, nil
		},
		DescribeInstsFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-mock-12345"),
								PrivateIpAddress: aws.String("10.0.0.10"),
								PublicIpAddress:   aws.String("54.0.0.1"),
								PublicDnsName:     aws.String("ec2-mock.compute.amazonaws.com"),
								NetworkInterfaces: []types.InstanceNetworkInterface{
									{NetworkInterfaceId: aws.String("eni-mock-12345")},
								},
							},
						},
					},
				},
			}, nil
		},
		DescribeImagesFunc: func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{
						ImageId:        aws.String("ami-mock-12345"),
						RootDeviceName: aws.String("/dev/sda1"),
						Architecture:   types.ArchitectureValuesX8664,
						CreationDate:   aws.String("2024-01-01T00:00:00.000Z"),
						Name:           aws.String("ubuntu-22.04"),
					},
				},
			}, nil
		},
	}

	elbMock := &captureNLBSubnetsMock{
		mockELBv2Client: mockELBv2Client{},
		captureFunc: func(subnets []string) {
			capturedNLBSubnets = subnets
		},
	}

	p := newClusterTestProvider(mock, elbMock)
	// Enable HA so NLB is created
	p.Spec.Cluster.HighAvailability = &v1alpha1.HighAvailabilityConfig{Enabled: true}
	p.Spec.Cluster.ControlPlane.Count = 3

	err := p.CreateCluster()
	if err != nil {
		t.Fatalf("CreateCluster() returned error: %v", err)
	}

	// Verify NLB was created with the public subnet
	if len(capturedNLBSubnets) == 0 {
		t.Fatal("NLB was not created or subnets were not captured")
	}

	found := false
	for _, s := range capturedNLBSubnets {
		if s == "subnet-public-99999" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("NLB should use public subnet (subnet-public-99999), got subnets: %v", capturedNLBSubnets)
	}
}

// TestCreateClusterInstancesNoPublicIP verifies instances in cluster mode
// do not get public IP addresses (they are in private subnet behind NAT GW).
func TestCreateClusterInstancesNoPublicIP(t *testing.T) {
	var capturedRunInputs []ec2.RunInstancesInput
	var mu sync.Mutex

	mock := &MockEC2Client{
		CreateSubnetFunc: func(_ context.Context, params *ec2.CreateSubnetInput, _ ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
			cidr := aws.ToString(params.CidrBlock)
			if cidr == "10.0.1.0/24" {
				return &ec2.CreateSubnetOutput{
					Subnet: &types.Subnet{SubnetId: aws.String("subnet-public-12345")},
				}, nil
			}
			return &ec2.CreateSubnetOutput{
				Subnet: &types.Subnet{SubnetId: aws.String("subnet-private-12345")},
			}, nil
		},
		CreateSGFunc: func() func(_ context.Context, _ *ec2.CreateSecurityGroupInput, _ ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
			counter := 0
			return func(_ context.Context, _ *ec2.CreateSecurityGroupInput, _ ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
				counter++
				return &ec2.CreateSecurityGroupOutput{
					GroupId: aws.String(fmt.Sprintf("sg-mock-%d", counter)),
				}, nil
			}
		}(),
		RunInstancesFunc: func(_ context.Context, params *ec2.RunInstancesInput, _ ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			mu.Lock()
			capturedRunInputs = append(capturedRunInputs, *params)
			mu.Unlock()
			return &ec2.RunInstancesOutput{
				Instances: []types.Instance{
					{
						InstanceId:       aws.String("i-mock-12345"),
						PrivateIpAddress: aws.String("10.0.0.10"),
						PublicIpAddress:   aws.String(""),
						PublicDnsName:     aws.String(""),
						NetworkInterfaces: []types.InstanceNetworkInterface{
							{NetworkInterfaceId: aws.String("eni-mock-12345")},
						},
					},
				},
			}, nil
		},
		DescribeInstsFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-mock-12345"),
								PrivateIpAddress: aws.String("10.0.0.10"),
								PublicIpAddress:   aws.String(""),
								PublicDnsName:     aws.String(""),
								NetworkInterfaces: []types.InstanceNetworkInterface{
									{NetworkInterfaceId: aws.String("eni-mock-12345")},
								},
							},
						},
					},
				},
			}, nil
		},
		DescribeImagesFunc: func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{
						ImageId:        aws.String("ami-mock-12345"),
						RootDeviceName: aws.String("/dev/sda1"),
						Architecture:   types.ArchitectureValuesX8664,
						CreationDate:   aws.String("2024-01-01T00:00:00.000Z"),
						Name:           aws.String("ubuntu-22.04"),
					},
				},
			}, nil
		},
	}

	p := newClusterTestProvider(mock, &mockELBv2Client{})

	err := p.CreateCluster()
	if err != nil {
		t.Fatalf("CreateCluster() returned error: %v", err)
	}

	if len(capturedRunInputs) == 0 {
		t.Fatal("no RunInstances calls captured")
	}

	for i, input := range capturedRunInputs {
		if len(input.NetworkInterfaces) == 0 {
			t.Errorf("RunInstances call %d has no NetworkInterfaces", i)
			continue
		}
		nic := input.NetworkInterfaces[0]
		if nic.AssociatePublicIpAddress == nil {
			t.Errorf("RunInstances call %d: AssociatePublicIpAddress is nil, expected false", i)
		} else if *nic.AssociatePublicIpAddress {
			t.Errorf("RunInstances call %d: AssociatePublicIpAddress is true, expected false for cluster mode instances in private subnet", i)
		}
	}
}

// captureNLBSubnetsMock wraps mockELBv2Client to capture the subnets passed to CreateLoadBalancer.
type captureNLBSubnetsMock struct {
	mockELBv2Client
	captureFunc func(subnets []string)
}

func (m *captureNLBSubnetsMock) CreateLoadBalancer(_ context.Context, params *elasticloadbalancingv2.CreateLoadBalancerInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateLoadBalancerOutput, error) {
	if m.captureFunc != nil {
		m.captureFunc(params.Subnets)
	}
	return &elasticloadbalancingv2.CreateLoadBalancerOutput{
		LoadBalancers: []elbv2types.LoadBalancer{
			{
				LoadBalancerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/test-nlb/abc123"),
				DNSName:         aws.String("test-nlb-abc123.elb.us-east-1.amazonaws.com"),
			},
		},
	}, nil
}

// newClusterTestProvider creates a provider configured for cluster mode testing.
func newClusterTestProvider(ec2Mock *MockEC2Client, elbMock internalaws.ELBv2Client) *Provider {
	env := v1alpha1.Environment{}
	env.Name = "test-cluster"
	env.Spec.PrivateKey = "test-key"
	env.Spec.Username = "ubuntu"
	env.Spec.KeyName = "test-key"
	env.Spec.Cluster = &v1alpha1.ClusterSpec{
		ControlPlane: v1alpha1.ControlPlaneSpec{
			Count:        1,
			InstanceType: "t3.medium",
			OS:           "ubuntu_22.04",
		},
	}

	p := &Provider{
		ec2:         ec2Mock,
		elbv2:       elbMock,
		Environment: &env,
		log:         mockLogger(),
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("test-cluster")},
		},
	}
	return p
}
