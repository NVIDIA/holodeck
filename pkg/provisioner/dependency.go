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

package provisioner

import (
	"bytes"
	"context"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/gitref"
	"github.com/NVIDIA/holodeck/pkg/provisioner/templates"
)

const (
	kubeadmInstaller          = "kubeadm"
	kindInstaller             = "kind"
	microk8sInstaller         = "microk8s"
	containerdRuntime         = "containerd"
	crioRuntime               = "crio"
	dockerRuntime             = "docker"
	nvdriverInstaller         = "nvdriver"
	containerToolkitInstaller = "containerToolkit"
	kernelInstaller           = "kernel"
)

var (
	functions = map[string]ProvisionFunc{
		kubeadmInstaller:          kubeadm,
		kindInstaller:             kind,
		microk8sInstaller:         microk8s,
		containerdRuntime:         containerd,
		crioRuntime:               criO,
		dockerRuntime:             docker,
		nvdriverInstaller:         nvdriver,
		containerToolkitInstaller: containerToolkit,
		kernelInstaller:           kernel,
	}
)

type ProvisionFunc func(tpl *bytes.Buffer, env v1alpha1.Environment) error

func nvdriver(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	nvdriver := templates.NewNvDriver(env)
	return nvdriver.Execute(tpl, env)
}

func docker(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	docker := templates.NewDocker(env)
	return docker.Execute(tpl, env)
}

func containerd(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	containerd := templates.NewContainerd(env)
	return containerd.Execute(tpl, env)
}

func criO(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	criO := templates.NewCriO(env)
	return criO.Execute(tpl, env)
}

func containerToolkit(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	ctk, err := templates.NewContainerToolkit(env)
	if err != nil {
		return err
	}

	// Resolve git ref if using git or latest source
	if ctk.Source == "git" || ctk.Source == "latest" {
		// Use 35s context to ensure HTTP client timeout (30s) fires first
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()

		resolver := gitref.NewGitHubResolver()
		var ref string
		if ctk.Source == "git" {
			ref = ctk.GitRef
		} else {
			ref = ctk.TrackBranch
		}

		_, shortSHA, err := resolver.Resolve(ctx, ctk.GitRepo, ref)
		if err != nil {
			return err
		}
		ctk.SetResolvedCommit(shortSHA)
	}

	return ctk.Execute(tpl, env)
}

func kubeadm(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	kubernetes, err := templates.NewKubernetes(env)
	if err != nil {
		return err
	}

	// Resolve git ref if using git source
	if kubernetes.Source == "git" {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()

		resolver := gitref.NewGitHubResolver()
		_, shortSHA, err := resolver.Resolve(ctx, kubernetes.GitRepo, kubernetes.GitRef)
		if err != nil {
			return err
		}
		kubernetes.SetResolvedCommit(shortSHA)
	}
	// Note: "latest" source resolves at provision time on the remote host

	return kubernetes.Execute(tpl, env)
}

func microk8s(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	// MicroK8s only supports release source (validated in types)
	microk8s, err := templates.NewKubernetes(env)
	if err != nil {
		return err
	}
	return microk8s.Execute(tpl, env)
}

func kind(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	kind, err := templates.NewKubernetes(env)
	if err != nil {
		return err
	}

	// Resolve git ref if using git source
	if kind.Source == "git" {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()

		resolver := gitref.NewGitHubResolver()
		_, shortSHA, err := resolver.Resolve(ctx, kind.GitRepo, kind.GitRef)
		if err != nil {
			return err
		}
		kind.SetResolvedCommit(shortSHA)
	}

	return kind.Execute(tpl, env)
}

func kernel(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	kernel, err := templates.NewKernelTemplate(env)
	if err != nil {
		return err
	}
	_, err = tpl.Write(kernel.Bytes())
	return err
}

// DependencySolver is a struct that holds the dependency list
type DependencyResolver struct {
	Dependencies []ProvisionFunc
	env          *v1alpha1.Environment
}

// DependencyConfigurator defines methods for configuring dependencies
type DependencyConfigurator interface {
	withKubernetes()
	withContainerRuntime()
	withContainerToolkit()
	withNVDriver()
	withKernel()
	Resolve() []ProvisionFunc
}

// NewDependencies creates a new DependencyResolver for the given environment.
func NewDependencies(env *v1alpha1.Environment) *DependencyResolver {
	return &DependencyResolver{
		Dependencies: []ProvisionFunc{},
		env:          env,
	}
}

func (d *DependencyResolver) withKubernetes() {
	switch d.env.Spec.Kubernetes.KubernetesInstaller {
	case kubeadmInstaller:
		d.Dependencies = append(d.Dependencies, functions[kubeadmInstaller])
	case kindInstaller:
		d.Dependencies = append(d.Dependencies, functions[kindInstaller])
	case microk8sInstaller:
		// reset the list to only include microk8s
		d.Dependencies = nil
		d.Dependencies = append(d.Dependencies, functions[microk8sInstaller])
	default:
		// default to kubeadm if KubernetesInstaller is empty
		d.Dependencies = append(d.Dependencies, functions[kubeadmInstaller])
	}
}

func (d *DependencyResolver) withContainerRuntime() {
	switch d.env.Spec.ContainerRuntime.Name {
	case containerdRuntime:
		d.Dependencies = append(d.Dependencies, functions[containerdRuntime])
	case crioRuntime:
		d.Dependencies = append(d.Dependencies, functions[crioRuntime])
	case dockerRuntime:
		d.Dependencies = append(d.Dependencies, functions[dockerRuntime])
	default:
		// default to containerd if ContainerRuntime.Name is empty
		d.Dependencies = append(d.Dependencies, functions[containerdRuntime])
	}
}

func (d *DependencyResolver) withContainerToolkit() {
	d.Dependencies = append(d.Dependencies, functions[containerToolkitInstaller])
}

func (d *DependencyResolver) withNVDriver() {
	d.Dependencies = append(d.Dependencies, functions[nvdriverInstaller])
}

func (d *DependencyResolver) withKernel() {
	d.Dependencies = append(d.Dependencies, functions[kernelInstaller])
}

// ensureKindCompatibleDocker ensures Docker version is compatible with KIND
// source builds, which require Docker API v1.44+ (Docker 20.10+).
// This method automatically sets a minimum Docker version if:
// - Kubernetes installer is KIND
// - Source is 'git' or 'latest' (requires building node images)
// - Docker is the container runtime
// - No explicit version is already set
func (d *DependencyResolver) ensureKindCompatibleDocker() {
	// Check if this is KIND with git or latest source
	if d.env.Spec.Kubernetes.Install &&
		d.env.Spec.Kubernetes.KubernetesInstaller == kindInstaller &&
		(d.env.Spec.Kubernetes.Source == "git" || d.env.Spec.Kubernetes.Source == "latest") {

		// Check if Docker is the runtime and needs version upgrade
		if d.env.Spec.ContainerRuntime.Install &&
			d.env.Spec.ContainerRuntime.Name == dockerRuntime &&
			d.env.Spec.ContainerRuntime.Version == "" {

			// Set minimum Docker version for KIND source builds (API v1.44+)
			// Using Ubuntu 22.04 package version format
			d.env.Spec.ContainerRuntime.Version = "5:24.0.0-1~ubuntu.22.04~jammy"
		}
	}
}

// Resolve returns the dependency list in the correct order
func (d *DependencyResolver) Resolve() []ProvisionFunc {
	// Add Kernel to the list first since it's a system-level dependency
	if d.env.Spec.Kernel.Version != "" {
		d.withKernel()
	}

	// Add NVDriver to the list
	if d.env.Spec.NVIDIADriver.Install {
		d.withNVDriver()
	}

	// Ensure compatible Docker version for KIND source builds
	d.ensureKindCompatibleDocker()

	// Add Container Runtime to the list
	if d.env.Spec.ContainerRuntime.Install {
		d.withContainerRuntime()
	}

	// Add Container Toolkit to the list
	if d.env.Spec.NVIDIAContainerToolkit.Install {
		d.withContainerToolkit()
	}

	// Add Kubernetes to the list
	if d.env.Spec.Kubernetes.Install {
		d.withKubernetes()
	}

	return d.Dependencies
}
