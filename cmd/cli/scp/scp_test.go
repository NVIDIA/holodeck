/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

package scp

import (
	"testing"
)

func TestParsePath_Local(t *testing.T) {
	spec := parsePath("/home/user/file.txt")

	if spec.isRemote {
		t.Error("expected local path")
	}
	if spec.path != "/home/user/file.txt" {
		t.Errorf("expected /home/user/file.txt, got %s", spec.path)
	}
	if spec.instanceID != "" {
		t.Errorf("expected empty instance ID, got %s", spec.instanceID)
	}
}

func TestParsePath_Remote(t *testing.T) {
	spec := parsePath("abc123:/tmp/file.txt")

	if !spec.isRemote {
		t.Error("expected remote path")
	}
	if spec.instanceID != "abc123" {
		t.Errorf("expected instance ID abc123, got %s", spec.instanceID)
	}
	if spec.path != "/tmp/file.txt" {
		t.Errorf("expected /tmp/file.txt, got %s", spec.path)
	}
}

func TestParsePath_WindowsPath(t *testing.T) {
	spec := parsePath("C:\\Users\\file.txt")

	if spec.isRemote {
		t.Error("Windows path should not be parsed as remote")
	}
}

func TestParsePath_RelativePath(t *testing.T) {
	spec := parsePath("./local/file.txt")

	if spec.isRemote {
		t.Error("expected local path")
	}
	if spec.path != "./local/file.txt" {
		t.Errorf("expected ./local/file.txt, got %s", spec.path)
	}
}

func TestParsePath_RemoteHomeDir(t *testing.T) {
	spec := parsePath("abc123:~/config")

	if !spec.isRemote {
		t.Error("expected remote path")
	}
	if spec.instanceID != "abc123" {
		t.Errorf("expected instance ID abc123, got %s", spec.instanceID)
	}
	if spec.path != "~/config" {
		t.Errorf("expected ~/config, got %s", spec.path)
	}
}

func TestParsePath_RemoteRootPath(t *testing.T) {
	spec := parsePath("myinstance:/var/log/syslog")

	if !spec.isRemote {
		t.Error("expected remote path")
	}
	if spec.instanceID != "myinstance" {
		t.Errorf("expected instance ID myinstance, got %s", spec.instanceID)
	}
	if spec.path != "/var/log/syslog" {
		t.Errorf("expected /var/log/syslog, got %s", spec.path)
	}
}

func TestParsePath_JustFilename(t *testing.T) {
	spec := parsePath("file.txt")

	if spec.isRemote {
		t.Error("expected local path")
	}
	if spec.path != "file.txt" {
		t.Errorf("expected file.txt, got %s", spec.path)
	}
}
