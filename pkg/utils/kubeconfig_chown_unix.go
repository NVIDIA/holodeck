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
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// chownToWorkspaceOwner changes the file's owner to match its parent
// directory. In the GitHub Actions container, the parent directory is
// the bind-mounted workspace owned by the runner user; chowning the
// kubeconfig to that UID/GID lets the runner step (running as the
// runner user, not the action container's root) read the file.
//
// No-op when the current process already owns the parent directory
// (covers normal CLI use where the user runs holodeck as themselves).
func chownToWorkspaceOwner(path string) error {
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("stat parent directory %q: %w", parent, err)
	}
	sys, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		// Defensive: should not happen on unix builds.
		return nil
	}
	if int(sys.Uid) == os.Getuid() {
		return nil // already correct owner
	}
	if err := os.Chown(path, int(sys.Uid), int(sys.Gid)); err != nil {
		return fmt.Errorf("chown %q to %d:%d: %w", path, sys.Uid, sys.Gid, err)
	}
	return nil
}
