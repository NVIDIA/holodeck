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

package utils

import (
	"net/url"
	"os"
	"testing"

	"github.com/NVIDIA/holodeck/pkg/testutil"
)

// TestMain runs this package's tests in a child process that cannot reach
// real AWS credentials: some of these tests construct real AWS clients, and
// they must never reach a real account, whatever credentials the machine has.
func TestMain(m *testing.M) { os.Exit(testutil.RunAWSIsolated(m)) }

// TestAWSIsolation fails unless the AWS isolation from TestMain is in effect.
func TestAWSIsolation(t *testing.T) { testutil.RequireAWSIsolation(t) }

// TestIPLookupHostsMatchDefaultIPServices keeps testutil's NO_PROXY
// exemption equal to the hosts GetIPAddress actually calls.
func TestIPLookupHostsMatchDefaultIPServices(t *testing.T) {
	var hosts []string
	for _, s := range DefaultIPServices() {
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatalf("parse %q: %v", s.URL, err)
		}
		hosts = append(hosts, u.Hostname())
	}
	got, want := testutil.IPLookupHosts(), hosts
	if len(got) != len(want) {
		t.Fatalf("testutil.IPLookupHosts() = %v, DefaultIPServices hosts = %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("testutil.IPLookupHosts() = %v, DefaultIPServices hosts = %v", got, want)
		}
	}
}
