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

// chownToWorkspaceOwner is a no-op on non-Unix platforms (Windows,
// Plan9). The URL rewrite still happens; file mode stays whatever
// RewriteKubeConfigServer wrote (0600).
func chownToWorkspaceOwner(_ string) error {
	return nil
}
