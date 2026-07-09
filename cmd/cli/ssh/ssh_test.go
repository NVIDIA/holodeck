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

package ssh

import (
	"context"
	"reflect"
	"testing"

	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v3"
)

// TestRemoteCommand_ExtractionThroughRealCommand drives the real ssh command
// through the v3 parser and asserts that remoteCommand() derives the correct
// remote command for each documented invocation. It guards the v3 "--"
// semantics fix: v3 consumes the "--" terminator and drops it from Args(), so
// the remote command is Args().Tail(). Reverting remoteCommand to the old
// v2-style "scan Args for a literal --" loop makes the passthrough case return
// nil (there is no "--" left to find), turning this test RED.
func TestRemoteCommand_ExtractionThroughRealCommand(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want []string
	}{
		{
			name: "passthrough preserves remote flags after --",
			argv: []string{"holodeck", "ssh", "abc123", "--", "kubectl", "get", "nodes", "--remote-flag"},
			want: []string{"kubectl", "get", "nodes", "--remote-flag"},
		},
		{
			name: "node flag before -- does not leak into remote command",
			argv: []string{"holodeck", "ssh", "abc123", "--node", "worker-0", "--", "nvidia-smi"},
			want: []string{"nvidia-smi"},
		},
		{
			name: "no remote command means interactive (empty)",
			argv: []string{"holodeck", "ssh", "abc123"},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			sshCmd := NewCommand(logger.NewLogger())
			// Inject a capturing Action so we observe the real command's parsed
			// Args + the real remoteCommand() without performing live I/O.
			sshCmd.Action = func(_ context.Context, cmd *cli.Command) error {
				got = remoteCommand(cmd.Args())
				return nil
			}
			root := &cli.Command{Name: "holodeck", Commands: []*cli.Command{sshCmd}}

			if err := root.Run(context.Background(), tt.argv); err != nil {
				t.Fatalf("parsing %v failed: %v", tt.argv, err)
			}
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("remoteCommand(%v) = %v, want %v", tt.argv, got, tt.want)
			}
		})
	}
}
