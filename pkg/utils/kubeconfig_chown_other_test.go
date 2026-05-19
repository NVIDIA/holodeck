//go:build !unix

/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestChownToWorkspaceOwner_NonUnixIsNoOp pins the non-Unix build
// (Windows, Plan9): chownToWorkspaceOwner returns nil for any input,
// including paths whose parent directory doesn't exist. This contract
// matters for callers of RewriteKubeConfigServer that wrap their
// behaviour with chownToWorkspaceOwner — on Windows the call must not
// fail just because the platform has no POSIX ownership model.
func TestChownToWorkspaceOwner_NonUnixIsNoOp(t *testing.T) {
	// Path that doesn't exist; unix build would error on os.Stat, but
	// the non-unix stub must ignore the path entirely.
	require.NoError(t, chownToWorkspaceOwner("Z:\\does\\not\\exist\\kubeconfig"))
	require.NoError(t, chownToWorkspaceOwner(""))
}
