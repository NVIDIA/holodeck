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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKubeadmConfigLocalPath_UniquePerEnvironment(t *testing.T) {
	pathA := kubeadmConfigLocalPath("env-alpha-abc123")
	pathB := kubeadmConfigLocalPath("env-beta-def456")

	assert.NotEqual(t, pathA, pathB, "different environments must produce different file paths")
	assert.Contains(t, pathA, "env-alpha-abc123")
	assert.Contains(t, pathB, "env-beta-def456")
	assert.True(t, strings.HasSuffix(pathA, ".yaml"))
}
