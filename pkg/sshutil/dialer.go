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

package sshutil

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/NVIDIA/holodeck/internal/logger"
)

type HostKeyPolicy string

const (
	HostKeyPolicyAcceptNew HostKeyPolicy = "accept-new"
	HostKeyPolicyStrict    HostKeyPolicy = "strict"
	HostKeyPolicyOff       HostKeyPolicy = "off"
)

type RetryPolicy struct {
	MaxAttempts int
	Delay       time.Duration
	Exponential bool
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

type TimeoutConfig struct {
	Handshake time.Duration
	Keepalive time.Duration
}

type AuthConfig struct {
	User        string
	KeyPath     string
	UseAgent    bool
	AgentSocket string
}

type Dialer struct {
	Auth     AuthConfig
	Retry    RetryPolicy
	Timeouts TimeoutConfig
	HostKey  HostKeyPolicy
	Log      *logger.FunLogger
}

func (d *Dialer) Dial(ctx context.Context, target string, t Transport) (*ssh.Client, error) {
	return nil, errors.New("not implemented")
}
