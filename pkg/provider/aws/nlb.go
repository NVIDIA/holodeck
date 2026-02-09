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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	// Port for Kubernetes API server
	k8sAPIPort = 6443
	// Health check settings for NLB target group
	healthCheckIntervalSeconds = 10
	healthCheckTimeoutSeconds  = 5
	healthyThresholdCount      = 2
	unhealthyThresholdCount    = 2
	// Timeout for ELBv2 API calls
	elbv2APITimeout = 2 * time.Minute
)

// createNLB creates a Network Load Balancer for HA control plane
func (p *Provider) createNLB(cache *ClusterCache) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating Network Load Balancer")

	lbType := elbv2types.LoadBalancerTypeEnumNetwork
	lbName := fmt.Sprintf("%s-nlb", p.ObjectMeta.Name)

	// Determine subnet IDs (use the same subnet for NLB)
	subnetIDs := []string{cache.Subnetid}

	// Create load balancer
	createLBInput := &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:          aws.String(lbName),
		Type:          lbType,
		Subnets:       subnetIDs,
		Scheme:        elbv2types.LoadBalancerSchemeEnumInternetFacing,
		IpAddressType: elbv2types.IpAddressTypeIpv4,
		Tags:          p.convertTagsToELBv2Tags(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancel()

	createLBOutput, err := p.elbv2.CreateLoadBalancer(ctx, createLBInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating load balancer: %w", err)
	}

	if len(createLBOutput.LoadBalancers) == 0 {
		p.fail()
		return fmt.Errorf("load balancer creation returned no load balancers")
	}

	lb := createLBOutput.LoadBalancers[0]
	cache.LoadBalancerArn = aws.ToString(lb.LoadBalancerArn)
	cache.LoadBalancerDNS = aws.ToString(lb.DNSName)

	p.log.Info("Created Network Load Balancer: %s (%s)", cache.LoadBalancerDNS, cache.LoadBalancerArn)
	p.done()
	return nil
}

// createTargetGroup creates a target group for the Kubernetes API server (port 6443)
func (p *Provider) createTargetGroup(cache *ClusterCache) error {
	if cache.LoadBalancerArn == "" {
		return fmt.Errorf("load balancer ARN is required to create target group")
	}

	p.log.Wg.Add(1)
	go p.log.Loading("Creating target group for Kubernetes API")

	tgName := fmt.Sprintf("%s-k8s-api-tg", p.ObjectMeta.Name)

	// Create target group for Kubernetes API (port 6443)
	createTGInput := &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:                       aws.String(tgName),
		Protocol:                   elbv2types.ProtocolEnumTcp,
		Port:                       aws.Int32(k8sAPIPort),
		VpcId:                      aws.String(cache.Vpcid),
		TargetType:                 elbv2types.TargetTypeEnumInstance,
		HealthCheckProtocol:        elbv2types.ProtocolEnumTcp,
		HealthCheckPort:            aws.String(fmt.Sprintf("%d", k8sAPIPort)),
		HealthCheckIntervalSeconds: aws.Int32(healthCheckIntervalSeconds),
		HealthCheckTimeoutSeconds:  aws.Int32(healthCheckTimeoutSeconds),
		HealthyThresholdCount:      aws.Int32(healthyThresholdCount),
		UnhealthyThresholdCount:    aws.Int32(unhealthyThresholdCount),
		Tags:                       p.convertTagsToELBv2Tags(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancel()

	createTGOutput, err := p.elbv2.CreateTargetGroup(ctx, createTGInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating target group: %w", err)
	}

	if len(createTGOutput.TargetGroups) == 0 {
		p.fail()
		return fmt.Errorf("target group creation returned no target groups")
	}

	tg := createTGOutput.TargetGroups[0]
	cache.TargetGroupArn = aws.ToString(tg.TargetGroupArn)

	p.log.Info("Created target group: %s", cache.TargetGroupArn)

	// Create listener to forward traffic from NLB to target group
	if err := p.createListener(cache); err != nil {
		return fmt.Errorf("error creating listener: %w", err)
	}

	p.done()
	return nil
}

// createListener creates a listener on the load balancer forwarding to the target group
func (p *Provider) createListener(cache *ClusterCache) error {
	if cache.LoadBalancerArn == "" || cache.TargetGroupArn == "" {
		return fmt.Errorf("load balancer ARN and target group ARN are required")
	}

	createListenerInput := &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: aws.String(cache.LoadBalancerArn),
		Protocol:        elbv2types.ProtocolEnumTcp,
		Port:            aws.Int32(k8sAPIPort),
		DefaultActions: []elbv2types.Action{
			{
				Type: elbv2types.ActionTypeEnumForward,
				ForwardConfig: &elbv2types.ForwardActionConfig{
					TargetGroups: []elbv2types.TargetGroupTuple{
						{
							TargetGroupArn: aws.String(cache.TargetGroupArn),
						},
					},
				},
			},
		},
		Tags: p.convertTagsToELBv2Tags(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancel()

	_, err := p.elbv2.CreateListener(ctx, createListenerInput)
	if err != nil {
		return fmt.Errorf("error creating listener: %w", err)
	}

	p.log.Info("Created listener on port %d", k8sAPIPort)
	return nil
}

// registerTargets registers control-plane instances with the load balancer target group
func (p *Provider) registerTargets(cache *ClusterCache) error {
	if cache.TargetGroupArn == "" {
		return fmt.Errorf("target group ARN is required")
	}

	if len(cache.ControlPlaneInstances) == 0 {
		return fmt.Errorf("no control-plane instances to register")
	}

	p.log.Wg.Add(1)
	go p.log.Loading("Registering control-plane instances with load balancer")

	// Build target list from control-plane instances
	targets := make([]elbv2types.TargetDescription, 0, len(cache.ControlPlaneInstances))
	for _, inst := range cache.ControlPlaneInstances {
		if inst.InstanceID == "" {
			continue
		}
		targets = append(targets, elbv2types.TargetDescription{
			Id:   aws.String(inst.InstanceID),
			Port: aws.Int32(k8sAPIPort),
		})
	}

	if len(targets) == 0 {
		p.fail()
		return fmt.Errorf("no valid control-plane instances to register")
	}

	registerInput := &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: aws.String(cache.TargetGroupArn),
		Targets:        targets,
	}

	ctx, cancel := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancel()

	_, err := p.elbv2.RegisterTargets(ctx, registerInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error registering targets: %w", err)
	}

	p.log.Info("Registered %d control-plane instance(s) with load balancer", len(targets))
	p.done()
	return nil
}

// deleteNLB deletes the Network Load Balancer and associated resources
func (p *Provider) deleteNLB(cache *ClusterCache) error {
	if cache.LoadBalancerArn == "" {
		// No load balancer to delete
		return nil
	}

	p.log.Wg.Add(1)
	go p.log.Loading("Deleting Network Load Balancer")

	// Delete listener first (if exists)
	if cache.TargetGroupArn != "" {
		if err := p.deleteListener(cache); err != nil {
			p.log.Warning("Error deleting listener: %v", err)
		}
	}

	// Delete target group
	if cache.TargetGroupArn != "" {
		if err := p.deleteTargetGroup(cache); err != nil {
			p.log.Warning("Error deleting target group: %v", err)
		}
	}

	// Delete load balancer
	deleteLBInput := &elasticloadbalancingv2.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String(cache.LoadBalancerArn),
	}

	ctx, cancel := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancel()

	_, err := p.elbv2.DeleteLoadBalancer(ctx, deleteLBInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error deleting load balancer: %w", err)
	}

	p.log.Info("Deleted Network Load Balancer: %s", cache.LoadBalancerArn)
	p.done()
	return nil
}

// deleteListener deletes the listener associated with the load balancer
func (p *Provider) deleteListener(cache *ClusterCache) error {
	if cache.LoadBalancerArn == "" {
		return nil
	}

	// Describe listeners to find the listener ARN
	describeInput := &elasticloadbalancingv2.DescribeListenersInput{
		LoadBalancerArn: aws.String(cache.LoadBalancerArn),
	}

	ctx, cancel := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancel()

	describeOutput, err := p.elbv2.DescribeListeners(ctx, describeInput)
	if err != nil {
		return fmt.Errorf("error describing listeners: %w", err)
	}

	// Delete all listeners
	for _, listener := range describeOutput.Listeners {
		deleteInput := &elasticloadbalancingv2.DeleteListenerInput{
			ListenerArn: listener.ListenerArn,
		}

		ctxDel, cancelDel := context.WithTimeout(context.Background(), elbv2APITimeout)
		_, err := p.elbv2.DeleteListener(ctxDel, deleteInput)
		cancelDel()

		if err != nil {
			return fmt.Errorf("error deleting listener %s: %w", aws.ToString(listener.ListenerArn), err)
		}
	}

	return nil
}

// deleteTargetGroup deletes the target group
func (p *Provider) deleteTargetGroup(cache *ClusterCache) error {
	if cache.TargetGroupArn == "" {
		return nil
	}

	// Describe targets to get current targets
	describeTargetsInput := &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(cache.TargetGroupArn),
	}

	ctxTargets, cancelTargets := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancelTargets()

	targetsOutput, err := p.elbv2.DescribeTargetHealth(ctxTargets, describeTargetsInput)
	if err == nil && len(targetsOutput.TargetHealthDescriptions) > 0 {
		targets := make([]elbv2types.TargetDescription, 0, len(targetsOutput.TargetHealthDescriptions))
		for _, th := range targetsOutput.TargetHealthDescriptions {
			if th.Target != nil {
				targets = append(targets, *th.Target)
			}
		}
		if len(targets) > 0 {
			deregisterInput := &elasticloadbalancingv2.DeregisterTargetsInput{
				TargetGroupArn: aws.String(cache.TargetGroupArn),
				Targets:        targets,
			}
			ctxDereg, cancelDereg := context.WithTimeout(context.Background(), elbv2APITimeout)
			_, _ = p.elbv2.DeregisterTargets(ctxDereg, deregisterInput)
			cancelDereg()
		}
	}

	// Delete target group
	deleteTGInput := &elasticloadbalancingv2.DeleteTargetGroupInput{
		TargetGroupArn: aws.String(cache.TargetGroupArn),
	}

	ctx, cancel := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancel()

	_, err = p.elbv2.DeleteTargetGroup(ctx, deleteTGInput)
	if err != nil {
		return fmt.Errorf("error deleting target group: %w", err)
	}

	return nil
}

// convertTagsToELBv2Tags converts EC2 tags to ELBv2 tags
func (p *Provider) convertTagsToELBv2Tags() []elbv2types.Tag {
	tags := make([]elbv2types.Tag, 0, len(p.Tags))
	for _, tag := range p.Tags {
		tags = append(tags, elbv2types.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}
	return tags
}
