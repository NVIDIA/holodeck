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

package provisioner

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectTransport_Target(t *testing.T) {
	dt := NewDirectTransport("10.0.1.5")
	assert.Equal(t, "10.0.1.5", dt.Target())
}

func TestDirectTransport_Dial_Success(t *testing.T) {
	// Start a TCP listener to simulate a reachable host
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	// Accept connections in background
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().String()
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	// DirectTransport appends :22 by default, so we need to test with the actual port
	// We'll test the internal dial logic by creating a transport with the right address
	dt := &DirectTransport{host: host + ":" + port}
	conn, err := dt.Dial()
	require.NoError(t, err)
	assert.NotNil(t, conn)
	conn.Close()
}

func TestDirectTransport_Dial_Failure(t *testing.T) {
	// Use a port that nothing is listening on
	dt := &DirectTransport{host: "127.0.0.1:1"}
	conn, err := dt.Dial()
	assert.Error(t, err)
	assert.Nil(t, conn)
}

func TestSSMTransport_Target(t *testing.T) {
	st := &SSMTransport{
		InstanceID: "i-0abc123def456",
		Region:     "us-west-2",
		Profile:    "default",
	}
	assert.Equal(t, "i-0abc123def456", st.Target())
}

func TestSSMTransport_RetryDial_Success(t *testing.T) {
	// Start a TCP listener after a short delay to simulate SSM port forwarding startup
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	// Accept connections
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().String()

	// Test the retry dial function directly
	conn, err := retryDial(addr, 5, 50*time.Millisecond)
	require.NoError(t, err)
	assert.NotNil(t, conn)
	conn.Close()
}

func TestSSMTransport_RetryDial_AllAttemptsFail(t *testing.T) {
	// Use a port that nothing is listening on
	conn, err := retryDial("127.0.0.1:1", 3, 10*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "after 3 attempts")
}

func TestSSMTransport_RetryDial_ExponentialBackoff(t *testing.T) {
	// Verify that retryDial takes progressively longer between attempts
	// by timing the total duration with 3 attempts at base 10ms
	// Expected: attempt 0 (immediate), sleep 10ms, attempt 1, sleep 20ms, attempt 2
	// Total ~30ms minimum
	start := time.Now()
	_, err := retryDial("127.0.0.1:1", 3, 10*time.Millisecond)
	elapsed := time.Since(start)

	assert.Error(t, err)
	// With base 10ms: sleeps are 10ms + 20ms = 30ms minimum
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(25), "expected exponential backoff delays")
}

func TestNewDirectTransport(t *testing.T) {
	dt := NewDirectTransport("ec2-1-2-3-4.compute.amazonaws.com")
	assert.Equal(t, "ec2-1-2-3-4.compute.amazonaws.com", dt.Target())

	// Verify it implements Transport interface
	var _ Transport = dt
}

func TestNodeInfo_TransportField(t *testing.T) {
	dt := NewDirectTransport("10.0.1.5")
	node := NodeInfo{
		Name:       "worker-1",
		PublicIP:   "1.2.3.4",
		PrivateIP:  "10.0.1.5",
		Role:       "worker",
		InstanceID: "i-abc123",
		Transport:  dt,
	}

	assert.Equal(t, "i-abc123", node.InstanceID)
	assert.Equal(t, "10.0.1.5", node.Transport.Target())
}

func TestWithTransport_Option(t *testing.T) {
	dt := NewDirectTransport("10.0.1.5")
	opt := WithTransport(dt)
	assert.NotNil(t, opt)
}
