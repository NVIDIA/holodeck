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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestKubernetes_RemoteAccess_YAMLRoundTrip(t *testing.T) {
	in := []byte(`install: true
remoteAccess: true
`)
	var k Kubernetes
	require.NoError(t, yaml.Unmarshal(in, &k))
	assert.True(t, k.RemoteAccess, "remoteAccess should unmarshal to true")
	assert.True(t, k.Install)

	out, err := yaml.Marshal(k)
	require.NoError(t, err)

	var k2 Kubernetes
	require.NoError(t, yaml.Unmarshal(out, &k2))
	assert.Equal(t, k, k2, "round-trip should be idempotent")

	// Zero-value default: remoteAccess: false should marshal as
	// omitted (omitempty).
	zero := Kubernetes{Install: true}
	zeroOut, err := yaml.Marshal(zero)
	require.NoError(t, err)
	assert.NotContains(t, string(zeroOut), "remoteAccess",
		"default-false RemoteAccess must omit when serialized")
}
