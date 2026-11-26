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

package status_test

import (
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
