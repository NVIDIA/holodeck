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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/NVIDIA/holodeck/internal/aws/awsfake"
)

func TestDeleteInternetGateway_DetachNotFound(t *testing.T) {
	// An absent IGW makes both DetachInternetGateway and DeleteInternetGateway
	// return InvalidInternetGatewayID.NotFound, which the idempotent-delete
	// branches treat as success. Each is called exactly once — NotFound stops
	// the retry loop.
	f := awsfake.New()

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{InternetGwid: "igw-gone", Vpcid: "vpc-123"}

	err := provider.deleteInternetGateway(cache)
	if err != nil {
		t.Fatalf("expected no error when IGW is already gone, got: %v", err)
	}

	// Detach should be called exactly once (NotFound stops retries)
	if got := f.Store.CallsTo("DetachInternetGateway"); got != 1 {
		t.Errorf("expected 1 detach call (NotFound stops retries), got %d", got)
	}

	// Delete should also be called exactly once
	if got := f.Store.CallsTo("DeleteInternetGateway"); got != 1 {
		t.Errorf("expected 1 delete call (NotFound stops retries), got %d", got)
	}
}

func TestDeleteInternetGateway_DetachNotAttached(t *testing.T) {
	// An existing but unattached IGW makes Detach return Gateway.NotAttached,
	// which is still treated as success; the subsequent Delete removes it.
	f := awsfake.New()
	igw, err := f.EC2.CreateInternetGateway(context.Background(), &ec2.CreateInternetGatewayInput{})
	if err != nil {
		t.Fatalf("CreateInternetGateway: %v", err)
	}
	igwID := aws.ToString(igw.InternetGateway.InternetGatewayId)

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{InternetGwid: igwID, Vpcid: "vpc-456"}

	err = provider.deleteInternetGateway(cache)
	if err != nil {
		t.Fatalf("expected no error for Gateway.NotAttached, got: %v", err)
	}

	if got := f.Store.CallsTo("DetachInternetGateway"); got != 1 {
		t.Errorf("expected 1 detach call, got %d", got)
	}
}

func TestDeleteInternetGateway_DetachRealErrorRetries(t *testing.T) {
	// A non-NotFound error during detach should be retried the full maxRetries
	// times (and eventually fail). Queue one injected error per attempt.
	f := awsfake.New()
	for i := 0; i < maxRetries; i++ {
		f.Store.FailNext("DetachInternetGateway", fmt.Errorf("DependencyViolation: gateway has active connections"))
	}

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{InternetGwid: "igw-busy", Vpcid: "vpc-789"}

	err := provider.deleteInternetGateway(cache)
	if err == nil {
		t.Fatal("expected error for DependencyViolation, got nil")
	}

	// Should have retried maxRetries times
	if got := f.Store.CallsTo("DetachInternetGateway"); got != maxRetries {
		t.Errorf("expected %d detach calls (full retry), got %d", maxRetries, got)
	}
}
