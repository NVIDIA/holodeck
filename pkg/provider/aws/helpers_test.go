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
	"fmt"
	"time"
)

// noopSleep is a no-op sleep function used in tests to skip real delays.
var noopSleep = func(time.Duration) {}

// strPtr returns a pointer to s, for building AWS SDK input/output literals.
func strPtr(s string) *string {
	return &s
}

// ErrMockDescribeImages is a sentinel error injected for DescribeImages failures.
var ErrMockDescribeImages = fmt.Errorf("mock describe images error")
