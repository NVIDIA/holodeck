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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func TestDeleteInternetGateway_DetachNotFound(t *testing.T) {
	// When DetachInternetGateway returns InvalidInternetGatewayID.NotFound,
	// deleteInternetGateway should treat it as success (IGW already gone)
	// and proceed to the delete step without error.
	detachCalls := 0
	deleteCalls := 0

	mock := &MockEC2Client{
		DetachIGWFunc: func(ctx context.Context, params *ec2.DetachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
			detachCalls++
			return nil, fmt.Errorf("InvalidInternetGatewayID.NotFound: The internetGateway ID 'igw-gone' does not exist")
		},
		DeleteIGWFunc: func(ctx context.Context, params *ec2.DeleteInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error) {
			deleteCalls++
			return nil, fmt.Errorf("InvalidInternetGatewayID.NotFound: The internetGateway ID 'igw-gone' does not exist")
		},
	}

	provider := &Provider{ec2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{InternetGwid: "igw-gone", Vpcid: "vpc-123"}

	err := provider.deleteInternetGateway(cache)
	if err != nil {
		t.Fatalf("expected no error when IGW is already gone, got: %v", err)
	}

	// Detach should be called exactly once (NotFound stops retries)
	if detachCalls != 1 {
		t.Errorf("expected 1 detach call (NotFound stops retries), got %d", detachCalls)
	}

	// Delete should also be called exactly once
	if deleteCalls != 1 {
		t.Errorf("expected 1 delete call (NotFound stops retries), got %d", deleteCalls)
	}
}

func TestDeleteInternetGateway_DetachNotAttached(t *testing.T) {
	// Original behavior: Gateway.NotAttached during detach is still treated as success.
	detachCalls := 0

	mock := &MockEC2Client{
		DetachIGWFunc: func(ctx context.Context, params *ec2.DetachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
			detachCalls++
			return nil, fmt.Errorf("Gateway.NotAttached: The gateway igw-123 is not attached")
		},
		DeleteIGWFunc: func(ctx context.Context, params *ec2.DeleteInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error) {
			return &ec2.DeleteInternetGatewayOutput{}, nil
		},
	}

	provider := &Provider{ec2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{InternetGwid: "igw-123", Vpcid: "vpc-456"}

	err := provider.deleteInternetGateway(cache)
	if err != nil {
		t.Fatalf("expected no error for Gateway.NotAttached, got: %v", err)
	}

	if detachCalls != 1 {
		t.Errorf("expected 1 detach call, got %d", detachCalls)
	}
}

func TestDeleteInternetGateway_DetachRealErrorRetries(t *testing.T) {
	// A non-NotFound error during detach should be retried (and eventually fail).
	detachCalls := 0

	mock := &MockEC2Client{
		DetachIGWFunc: func(ctx context.Context, params *ec2.DetachInternetGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
			detachCalls++
			return nil, fmt.Errorf("DependencyViolation: gateway has active connections")
		},
	}

	provider := &Provider{ec2: mock, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{InternetGwid: "igw-busy", Vpcid: "vpc-789"}

	err := provider.deleteInternetGateway(cache)
	if err == nil {
		t.Fatal("expected error for DependencyViolation, got nil")
	}

	// Should have retried maxRetries times
	if detachCalls != maxRetries {
		t.Errorf("expected %d detach calls (full retry), got %d", maxRetries, detachCalls)
	}
}
