//go:build unix

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
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise chownToWorkspaceOwner, the unexported helper
// invoked by RewriteKubeConfigServer's caller (ApplyRemoteAccess) to
// hand the rewritten kubeconfig back to the workspace owner. End-to-
// end coverage lives in TestApplyRemoteAccess; the cases below pin
// the two distinct branches of the helper itself.

// TestChownToWorkspaceOwner_AlreadyOwnedNoOp verifies the
// short-circuit path: when the parent directory is already owned by
// the current process (the common CLI case), chownToWorkspaceOwner
// returns nil and the file's ownership is unchanged.
func TestChownToWorkspaceOwner_AlreadyOwnedNoOp(t *testing.T) {
	dir := t.TempDir() // owned by current process
	path := filepath.Join(dir, "kubeconfig")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0600))

	before, err := os.Stat(path)
	require.NoError(t, err)
	beforeStat := before.Sys().(*syscall.Stat_t)

	require.NoError(t, chownToWorkspaceOwner(path))

	after, err := os.Stat(path)
	require.NoError(t, err)
	afterStat := after.Sys().(*syscall.Stat_t)

	assert.Equal(t, beforeStat.Uid, afterStat.Uid, "owner UID must be unchanged")
	assert.Equal(t, beforeStat.Gid, afterStat.Gid, "owner GID must be unchanged")
}

// TestChownToWorkspaceOwner_ParentMissingError verifies the error
// path: if the parent directory cannot be stat'd, the function must
// return a wrapped error referencing the missing parent. This fails
// if the implementation forgets to check the os.Stat error.
func TestChownToWorkspaceOwner_ParentMissingError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist", "kubeconfig")

	err := chownToWorkspaceOwner(missing)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat parent directory")
	assert.Contains(t, err.Error(), "does-not-exist")
}
