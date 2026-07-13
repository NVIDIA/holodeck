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

// This file exercises the unexported SSH connect helpers (dryrunDialer,
// connectOrDie) that back the "ssh" provider branch of the command built by
// NewCommand; dryrun_test.go covers NewCommand's command-level wiring.
package dryrun

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

func TestDryrunDialer_HasHandshakeTimeout(t *testing.T) {
	d := dryrunDialer("/tmp/none", "u", logger.NewLogger())
	assert.NotZero(t, d.Timeouts.Handshake, "dryrun must set a handshake timeout (was missing)")
	assert.Equal(t, 20, d.Retry.MaxAttempts, "dryrun keeps its 20-attempt envelope")
}

func TestDryrunConnect_Succeeds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub)
	require.NoError(t, connectOrDie(keyPath, "tester", srv.Addr()))
}
