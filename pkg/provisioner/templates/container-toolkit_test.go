/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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
	"bytes"
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewContainerToolkit_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	ctk := NewContainerToolkit(env)
	if ctk.ContainerRuntime != "containerd" {
		t.Errorf("expected default ContainerRuntime to be 'containerd', got '%s'", ctk.ContainerRuntime)
	}
	if ctk.EnableCDI != false {
		t.Errorf("expected default EnableCDI to be false, got %v", ctk.EnableCDI)
	}
	if ctk.Source != "package" {
		t.Errorf("expected default Source to be 'package', got '%s'", ctk.Source)
	}
	if ctk.PackageChannel != "stable" {
		t.Errorf("expected default PackageChannel to be 'stable', got '%s'", ctk.PackageChannel)
	}
}

func TestNewContainerToolkit_Custom(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "docker",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				EnableCDI: true,
			},
		},
	}
	ctk := NewContainerToolkit(env)
	if ctk.ContainerRuntime != "docker" {
		t.Errorf("expected ContainerRuntime to be 'docker', got '%s'", ctk.ContainerRuntime)
	}
	if ctk.EnableCDI != true {
		t.Errorf("expected EnableCDI to be true, got %v", ctk.EnableCDI)
	}
	if ctk.Source != "package" {
		t.Errorf("expected default Source to be 'package', got '%s'", ctk.Source)
	}
}

func TestContainerToolkit_Execute(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				EnableCDI: true,
			},
		},
	}
	ctk := NewContainerToolkit(env)
	var buf bytes.Buffer
	err := ctk.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "nvidia-ctk runtime configure --runtime=containerd --set-as-default --enable-cdi=true") {
		t.Errorf("template output missing expected runtime config: %s", out)
	}

	// Test CNI path verification after nvidia-ctk
	if !strings.Contains(out, "Verifying CNI configuration after nvidia-ctk") {
		t.Error("template output missing CNI verification message")
	}
	if !strings.Contains(out, `bin_dir = "/opt/cni/bin"`) {
		t.Error("template output missing correct CNI bin_dir check")
	}
	// Should NOT contain the old path with /usr/libexec/cni
	if strings.Contains(out, "/usr/libexec/cni") {
		t.Error("template output should not contain /usr/libexec/cni path")
	}
}

func TestNewContainerToolkit_BackwardCompatibility(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Version: "1.17.6-1", // Legacy version field
			},
		},
	}
	ctk := NewContainerToolkit(env)
	if ctk.Source != "package" {
		t.Errorf("expected Source to be 'package' for backward compatibility, got '%s'", ctk.Source)
	}
	if ctk.PackageVersion != "1.17.6-1" {
		t.Errorf("expected PackageVersion to be '1.17.6-1', got '%s'", ctk.PackageVersion)
	}
}

func TestNewContainerToolkit_PackageSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Source: v1alpha1.NCTSourcePackage,
				Package: &v1alpha1.NCTPackageConfig{
					Channel: "experimental",
					Version: "1.17.6-1",
				},
			},
		},
	}
	ctk := NewContainerToolkit(env)
	if ctk.Source != "package" {
		t.Errorf("expected Source to be 'package', got '%s'", ctk.Source)
	}
	if ctk.PackageChannel != "experimental" {
		t.Errorf("expected PackageChannel to be 'experimental', got '%s'", ctk.PackageChannel)
	}
	if ctk.PackageVersion != "1.17.6-1" {
		t.Errorf("expected PackageVersion to be '1.17.6-1', got '%s'", ctk.PackageVersion)
	}
}

func TestNewContainerToolkit_GitSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Source: v1alpha1.NCTSourceGit,
				Git: &v1alpha1.NCTGitConfig{
					Repo: "https://github.com/NVIDIA/nvidia-container-toolkit.git",
					Ref:  "refs/tags/v1.17.6",
					Build: &v1alpha1.NCTBuildConfig{
						MakeTargets: []string{"all", "test"},
						ExtraEnv: map[string]string{
							"GOFLAGS": "-buildvcs=true",
						},
					},
				},
			},
		},
	}
	ctk := NewContainerToolkit(env)
	if ctk.Source != "git" {
		t.Errorf("expected Source to be 'git', got '%s'", ctk.Source)
	}
	if ctk.GitRepo != "https://github.com/NVIDIA/nvidia-container-toolkit.git" {
		t.Errorf("expected GitRepo to be 'https://github.com/NVIDIA/nvidia-container-toolkit.git', got '%s'", ctk.GitRepo)
	}
	if ctk.GitRef != "refs/tags/v1.17.6" {
		t.Errorf("expected GitRef to be 'refs/tags/v1.17.6', got '%s'", ctk.GitRef)
	}
	if len(ctk.GitMakeTargets) != 2 || ctk.GitMakeTargets[0] != "all" || ctk.GitMakeTargets[1] != "test" {
		t.Errorf("expected GitMakeTargets to be ['all', 'test'], got %v", ctk.GitMakeTargets)
	}
	if ctk.GitExtraEnv["GOFLAGS"] != "-buildvcs=true" {
		t.Errorf("expected GitExtraEnv['GOFLAGS'] to be '-buildvcs=true', got '%s'", ctk.GitExtraEnv["GOFLAGS"])
	}
}

func TestNewContainerToolkit_GitSource_DefaultRepo(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Source: v1alpha1.NCTSourceGit,
				Git: &v1alpha1.NCTGitConfig{
					Ref: "main",
				},
			},
		},
	}
	ctk := NewContainerToolkit(env)
	if ctk.GitRepo != "https://github.com/NVIDIA/nvidia-container-toolkit.git" {
		t.Errorf("expected default GitRepo to be 'https://github.com/NVIDIA/nvidia-container-toolkit.git', got '%s'", ctk.GitRepo)
	}
}

func TestNewContainerToolkit_LatestSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Source: v1alpha1.NCTSourceLatest,
				Latest: &v1alpha1.NCTLatestConfig{
					Track: "main",
					Repo:  "https://github.com/NVIDIA/nvidia-container-toolkit.git",
				},
			},
		},
	}
	ctk := NewContainerToolkit(env)
	if ctk.Source != "latest" {
		t.Errorf("expected Source to be 'latest', got '%s'", ctk.Source)
	}
	if ctk.LatestTrack != "main" {
		t.Errorf("expected LatestTrack to be 'main', got '%s'", ctk.LatestTrack)
	}
	if ctk.LatestRepo != "https://github.com/NVIDIA/nvidia-container-toolkit.git" {
		t.Errorf("expected LatestRepo to be 'https://github.com/NVIDIA/nvidia-container-toolkit.git', got '%s'", ctk.LatestRepo)
	}
}

func TestNewContainerToolkit_LatestSource_DefaultRepo(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Source: v1alpha1.NCTSourceLatest,
				Latest: &v1alpha1.NCTLatestConfig{
					Track: "main",
				},
			},
		},
	}
	ctk := NewContainerToolkit(env)
	if ctk.LatestRepo != "https://github.com/NVIDIA/nvidia-container-toolkit.git" {
		t.Errorf("expected default LatestRepo to be 'https://github.com/NVIDIA/nvidia-container-toolkit.git', got '%s'", ctk.LatestRepo)
	}
}

func TestContainerToolkit_Execute_PackageSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				EnableCDI: true,
				Source:    v1alpha1.NCTSourcePackage,
				Package: &v1alpha1.NCTPackageConfig{
					Channel: "experimental",
					Version: "1.17.6-1",
				},
			},
		},
	}
	ctk := NewContainerToolkit(env)
	var buf bytes.Buffer
	err := ctk.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()
	
	// Should contain package installation
	if !strings.Contains(out, "Install container toolkit from package") {
		t.Error("template output missing package installation section")
	}
	if !strings.Contains(out, "experimental/deb/nvidia-container-toolkit.list") {
		t.Error("template output missing experimental channel")
	}
	if !strings.Contains(out, "nvidia-container-toolkit=1.17.6-1") {
		t.Error("template output missing pinned version")
	}
}

func TestContainerToolkit_Execute_GitSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				EnableCDI: true,
				Source:    v1alpha1.NCTSourceGit,
				Git: &v1alpha1.NCTGitConfig{
					Repo: "https://github.com/NVIDIA/nvidia-container-toolkit.git",
					Ref:  "refs/tags/v1.17.6",
					Build: &v1alpha1.NCTBuildConfig{
						MakeTargets: []string{"all", "test"},
						ExtraEnv: map[string]string{
							"GOFLAGS": "-buildvcs=true",
						},
					},
				},
			},
		},
	}
	ctk := NewContainerToolkit(env)
	var buf bytes.Buffer
	err := ctk.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()
	
	// Should contain git installation
	if !strings.Contains(out, "Install container toolkit from git") {
		t.Error("template output missing git installation section")
	}
	if !strings.Contains(out, "git clone https://github.com/NVIDIA/nvidia-container-toolkit.git") {
		t.Error("template output missing git clone")
	}
	if !strings.Contains(out, "git checkout refs/tags/v1.17.6") {
		t.Error("template output missing git checkout")
	}
	if !strings.Contains(out, "make all test") {
		t.Error("template output missing make targets")
	}
	if !strings.Contains(out, `export GOFLAGS="-buildvcs=true"`) {
		t.Error("template output missing environment variable")
	}
	if !strings.Contains(out, "PROVENANCE.json") {
		t.Error("template output missing provenance file creation")
	}
}

func TestContainerToolkit_Execute_LatestSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				EnableCDI: true,
				Source:    v1alpha1.NCTSourceLatest,
				Latest: &v1alpha1.NCTLatestConfig{
					Track: "main",
					Repo:  "https://github.com/NVIDIA/nvidia-container-toolkit.git",
				},
			},
		},
	}
	ctk := NewContainerToolkit(env)
	var buf bytes.Buffer
	err := ctk.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()
	
	// Should contain latest installation
	if !strings.Contains(out, "Installing NVIDIA Container Toolkit from latest main branch") {
		t.Errorf("template output missing latest installation section. Output:\n%s", out)
	}
	if !strings.Contains(out, "git checkout main") {
		t.Error("template output missing branch checkout")
	}
	if !strings.Contains(out, "make all") {
		t.Error("template output missing default make target")
	}
	if !strings.Contains(out, "PROVENANCE.json") {
		t.Error("template output missing provenance file creation")
	}
}
