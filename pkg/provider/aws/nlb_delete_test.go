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
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockELBv2Client implements internalaws.ELBv2Client for testing.
type MockELBv2Client struct {
	CreateLBFunc         func(ctx context.Context, params *elasticloadbalancingv2.CreateLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateLoadBalancerOutput, error)
	DescribeLBsFunc      func(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error)
	DeleteLBFunc         func(ctx context.Context, params *elasticloadbalancingv2.DeleteLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteLoadBalancerOutput, error)
	CreateTGFunc         func(ctx context.Context, params *elasticloadbalancingv2.CreateTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateTargetGroupOutput, error)
	DescribeTGsFunc      func(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error)
	DescribeTHFunc       func(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error)
	DeleteTGFunc         func(ctx context.Context, params *elasticloadbalancingv2.DeleteTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteTargetGroupOutput, error)
	RegisterFunc         func(ctx context.Context, params *elasticloadbalancingv2.RegisterTargetsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.RegisterTargetsOutput, error)
	DeregisterFunc       func(ctx context.Context, params *elasticloadbalancingv2.DeregisterTargetsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeregisterTargetsOutput, error)
	CreateListenerFunc   func(ctx context.Context, params *elasticloadbalancingv2.CreateListenerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateListenerOutput, error)
	DescribeListenerFunc func(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error)
	DeleteListenerFunc   func(ctx context.Context, params *elasticloadbalancingv2.DeleteListenerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteListenerOutput, error)
	AddTagsFunc          func(ctx context.Context, params *elasticloadbalancingv2.AddTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.AddTagsOutput, error)
}

func (m *MockELBv2Client) CreateLoadBalancer(ctx context.Context, params *elasticloadbalancingv2.CreateLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateLoadBalancerOutput, error) {
	if m.CreateLBFunc != nil {
		return m.CreateLBFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.CreateLoadBalancerOutput{}, nil
}

func (m *MockELBv2Client) DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	if m.DescribeLBsFunc != nil {
		return m.DescribeLBsFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.DescribeLoadBalancersOutput{}, nil
}

func (m *MockELBv2Client) DeleteLoadBalancer(ctx context.Context, params *elasticloadbalancingv2.DeleteLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteLoadBalancerOutput, error) {
	if m.DeleteLBFunc != nil {
		return m.DeleteLBFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.DeleteLoadBalancerOutput{}, nil
}

func (m *MockELBv2Client) CreateTargetGroup(ctx context.Context, params *elasticloadbalancingv2.CreateTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateTargetGroupOutput, error) {
	if m.CreateTGFunc != nil {
		return m.CreateTGFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.CreateTargetGroupOutput{}, nil
}

func (m *MockELBv2Client) DescribeTargetGroups(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
	if m.DescribeTGsFunc != nil {
		return m.DescribeTGsFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.DescribeTargetGroupsOutput{}, nil
}

func (m *MockELBv2Client) DescribeTargetHealth(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
	if m.DescribeTHFunc != nil {
		return m.DescribeTHFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.DescribeTargetHealthOutput{}, nil
}

func (m *MockELBv2Client) DeleteTargetGroup(ctx context.Context, params *elasticloadbalancingv2.DeleteTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteTargetGroupOutput, error) {
	if m.DeleteTGFunc != nil {
		return m.DeleteTGFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.DeleteTargetGroupOutput{}, nil
}

func (m *MockELBv2Client) RegisterTargets(ctx context.Context, params *elasticloadbalancingv2.RegisterTargetsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.RegisterTargetsOutput, error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.RegisterTargetsOutput{}, nil
}

func (m *MockELBv2Client) DeregisterTargets(ctx context.Context, params *elasticloadbalancingv2.DeregisterTargetsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeregisterTargetsOutput, error) {
	if m.DeregisterFunc != nil {
		return m.DeregisterFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.DeregisterTargetsOutput{}, nil
}

func (m *MockELBv2Client) CreateListener(ctx context.Context, params *elasticloadbalancingv2.CreateListenerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateListenerOutput, error) {
	if m.CreateListenerFunc != nil {
		return m.CreateListenerFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.CreateListenerOutput{}, nil
}

func (m *MockELBv2Client) DescribeListeners(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
	if m.DescribeListenerFunc != nil {
		return m.DescribeListenerFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.DescribeListenersOutput{}, nil
}

func (m *MockELBv2Client) DeleteListener(ctx context.Context, params *elasticloadbalancingv2.DeleteListenerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteListenerOutput, error) {
	if m.DeleteListenerFunc != nil {
		return m.DeleteListenerFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.DeleteListenerOutput{}, nil
}

func (m *MockELBv2Client) AddTags(ctx context.Context, params *elasticloadbalancingv2.AddTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.AddTagsOutput, error) {
	if m.AddTagsFunc != nil {
		return m.AddTagsFunc(ctx, params, optFns...)
	}
	return &elasticloadbalancingv2.AddTagsOutput{}, nil
}

func TestDeleteNLB_LoadBalancerNotFound(t *testing.T) {
	mock := &MockELBv2Client{
		DeleteLBFunc: func(ctx context.Context, params *elasticloadbalancingv2.DeleteLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteLoadBalancerOutput, error) {
			return nil, fmt.Errorf("LoadBalancerNotFound: One or more load balancers not found")
		},
	}

	provider := &Provider{elbv2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{LoadBalancerArn: "arn:aws:elasticloadbalancing:us-east-1:123:loadbalancer/net/gone/abc"}

	err := provider.deleteNLB(cache)
	if err != nil {
		t.Fatalf("expected no error when NLB is already deleted, got: %v", err)
	}
}

func TestDeleteNLB_RealError(t *testing.T) {
	mock := &MockELBv2Client{
		DeleteLBFunc: func(ctx context.Context, params *elasticloadbalancingv2.DeleteLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteLoadBalancerOutput, error) {
			return nil, fmt.Errorf("InternalError: something went wrong")
		},
	}

	provider := &Provider{elbv2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{LoadBalancerArn: "arn:aws:elasticloadbalancing:us-east-1:123:loadbalancer/net/test/abc"}

	err := provider.deleteNLB(cache)
	if err == nil {
		t.Fatal("expected error for InternalError, got nil")
	}
}

func TestDeleteListener_ListenerNotFound(t *testing.T) {
	mock := &MockELBv2Client{
		DescribeListenerFunc: func(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
			return &elasticloadbalancingv2.DescribeListenersOutput{
				Listeners: []elbv2types.Listener{
					{ListenerArn: aws.String("arn:listener/gone")},
				},
			}, nil
		},
		DeleteListenerFunc: func(ctx context.Context, params *elasticloadbalancingv2.DeleteListenerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteListenerOutput, error) {
			return nil, fmt.Errorf("ListenerNotFound: One or more listeners not found")
		},
	}

	provider := &Provider{elbv2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{LoadBalancerArn: "arn:lb/test"}

	err := provider.deleteListener(cache)
	if err != nil {
		t.Fatalf("expected no error when listener is already deleted, got: %v", err)
	}
}

func TestDeleteListener_DescribeNotFound(t *testing.T) {
	mock := &MockELBv2Client{
		DescribeListenerFunc: func(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
			return nil, fmt.Errorf("LoadBalancerNotFound: LB already gone")
		},
	}

	provider := &Provider{elbv2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{LoadBalancerArn: "arn:lb/gone"}

	err := provider.deleteListener(cache)
	if err != nil {
		t.Fatalf("expected no error when LB is already deleted during describe, got: %v", err)
	}
}

func TestDeleteTargetGroup_NotFound(t *testing.T) {
	mock := &MockELBv2Client{
		DescribeTHFunc: func(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			return nil, fmt.Errorf("TargetGroupNotFound: target group gone")
		},
		DeleteTGFunc: func(ctx context.Context, params *elasticloadbalancingv2.DeleteTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteTargetGroupOutput, error) {
			return nil, fmt.Errorf("TargetGroupNotFound: target group gone")
		},
	}

	provider := &Provider{elbv2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{TargetGroupArn: "arn:tg/gone"}

	err := provider.deleteTargetGroup(cache)
	if err != nil {
		t.Fatalf("expected no error when target group is already deleted, got: %v", err)
	}
}

func TestDeleteTargetGroup_RealError(t *testing.T) {
	mock := &MockELBv2Client{
		DescribeTHFunc: func(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			return &elasticloadbalancingv2.DescribeTargetHealthOutput{}, nil
		},
		DeleteTGFunc: func(ctx context.Context, params *elasticloadbalancingv2.DeleteTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteTargetGroupOutput, error) {
			return nil, fmt.Errorf("InternalError: something went wrong")
		},
	}

	provider := &Provider{elbv2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{TargetGroupArn: "arn:tg/test"}

	err := provider.deleteTargetGroup(cache)
	if err == nil {
		t.Fatal("expected error for InternalError, got nil")
	}
}

func TestDeleteNLBForCluster_DescribeNotFound(t *testing.T) {
	mock := &MockELBv2Client{
		DescribeLBsFunc: func(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
			return nil, fmt.Errorf("LoadBalancerNotFound: One or more load balancers not found")
		},
	}

	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
	}
	provider := &Provider{elbv2: mock, log: mockLogger(), sleep: noopSleep, Environment: env}
	cache := &ClusterCache{LoadBalancerDNS: "gone-nlb.elb.amazonaws.com"}

	err := provider.deleteNLBForCluster(cache)
	if err != nil {
		t.Fatalf("expected no error when NLB is already deleted, got: %v", err)
	}
}
