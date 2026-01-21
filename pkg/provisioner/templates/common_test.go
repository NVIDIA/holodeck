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

package templates

import (
	"strings"
	"testing"
)

func TestCommonFunctions_OSDetection(t *testing.T) {
	// Test that CommonFunctions includes OS family detection
	if !strings.Contains(CommonFunctions, "detect_os_family") {
		t.Error("CommonFunctions missing detect_os_family function")
	}
	if !strings.Contains(CommonFunctions, "HOLODECK_OS_FAMILY") {
		t.Error("CommonFunctions missing HOLODECK_OS_FAMILY variable")
	}
}

func TestCommonFunctions_PackageManagerAbstraction(t *testing.T) {
	// Test that CommonFunctions includes package manager abstraction
	funcs := []string{"pkg_update", "pkg_install", "pkg_install_version", "pkg_add_repo", "pkg_arch"}
	for _, fn := range funcs {
		if !strings.Contains(CommonFunctions, fn) {
			t.Errorf("CommonFunctions missing %s function", fn)
		}
	}
}

func TestCommonFunctions_AmazonLinuxFedoraMapping(t *testing.T) {
	// Test that CommonFunctions includes Amazon Linux → Fedora version mapping
	// This is required for Docker repository configuration on Amazon Linux
	if !strings.Contains(CommonFunctions, "get_amzn_fedora_version") {
		t.Error("CommonFunctions missing get_amzn_fedora_version function")
	}
	if !strings.Contains(CommonFunctions, "HOLODECK_AMZN_FEDORA_VERSION") {
		t.Error("CommonFunctions missing HOLODECK_AMZN_FEDORA_VERSION variable")
	}

	// Verify version mappings are present
	mappings := []struct {
		alVersion     string
		fedoraVersion string
	}{
		{"2023", "39"}, // AL2023 → Fedora 39
		{"2024", "40"}, // AL2024 → Fedora 40
		{"2)", "35"},   // AL2 → Fedora 35 (partial match in case statement)
	}
	for _, m := range mappings {
		if !strings.Contains(CommonFunctions, m.alVersion) {
			t.Errorf("CommonFunctions missing Amazon Linux version %s mapping", m.alVersion)
		}
	}
}

func TestCommonFunctions_IdempotencyFramework(t *testing.T) {
	// Test that CommonFunctions includes idempotency framework
	funcs := []string{
		"holodeck_log",
		"holodeck_progress",
		"holodeck_error",
		"holodeck_mark_installed",
		"holodeck_retry",
	}
	for _, fn := range funcs {
		if !strings.Contains(CommonFunctions, fn) {
			t.Errorf("CommonFunctions missing %s function", fn)
		}
	}
}
