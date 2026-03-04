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

	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// ELBv2Client defines the interface for ELBv2 operations used throughout holodeck.
// This interface enables dependency injection and facilitates unit testing.
type ELBv2Client interface {
	// Load Balancer operations
	CreateLoadBalancer(ctx context.Context, params *elasticloadbalancingv2.CreateLoadBalancerInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateLoadBalancerOutput, error)
	DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error)
	DeleteLoadBalancer(ctx context.Context, params *elasticloadbalancingv2.DeleteLoadBalancerInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteLoadBalancerOutput, error)

	// Target Group operations
	CreateTargetGroup(ctx context.Context, params *elasticloadbalancingv2.CreateTargetGroupInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateTargetGroupOutput, error)
	DescribeTargetGroups(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error)
	DescribeTargetHealth(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error)
	DeleteTargetGroup(ctx context.Context, params *elasticloadbalancingv2.DeleteTargetGroupInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteTargetGroupOutput, error)

	// Target registration operations
	RegisterTargets(ctx context.Context, params *elasticloadbalancingv2.RegisterTargetsInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.RegisterTargetsOutput, error)
	DeregisterTargets(ctx context.Context, params *elasticloadbalancingv2.DeregisterTargetsInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeregisterTargetsOutput, error)

	// Listener operations
	CreateListener(ctx context.Context, params *elasticloadbalancingv2.CreateListenerInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateListenerOutput, error)
	DescribeListeners(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error)
	DeleteListener(ctx context.Context, params *elasticloadbalancingv2.DeleteListenerInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteListenerOutput, error)

	// Tag operations
	AddTags(ctx context.Context, params *elasticloadbalancingv2.AddTagsInput,
		optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.AddTagsOutput, error)
}

// Ensure *elasticloadbalancingv2.Client implements ELBv2Client at compile time.
var _ ELBv2Client = (*elasticloadbalancingv2.Client)(nil)

// ELBv2Types provides access to ELBv2 types for convenience.
type ELBv2Types struct {
	LoadBalancerTypeEnum elbv2types.LoadBalancerTypeEnum
	ProtocolEnum         elbv2types.ProtocolEnum
	TargetTypeEnum       elbv2types.TargetTypeEnum
}
