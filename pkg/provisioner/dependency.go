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

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/provisioner/templates"
)

var (
	functions = map[string]ProvisionFunc{
		"kubeadm":          kubeadm,
		"kind":             kind,
		"microk8s":         microk8s,
		"containerd":       containerd,
		"crio":             criO,
		"docker":           docker,
		"nvdriver":         nvdriver,
		"containerToolkit": containerToolkit,
	}
)

type ProvisionFunc func(tpl *bytes.Buffer, env v1alpha1.Environment) error

func nvdriver(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	nvdriver := templates.NewNvDriver()
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
	containerToolkit := templates.NewContainerToolkit(env)
	return containerToolkit.Execute(tpl, env)
}

func kubeadm(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	kubernetes, err := templates.NewKubernetes(env)
	if err != nil {
		return err
	}
	return kubernetes.Execute(tpl, env)
}

func microk8s(tpl *bytes.Buffer, env v1alpha1.Environment) error {
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
	return kind.Execute(tpl, env)
}

// DependencySolver is a struct that holds the dependency list
type DependencyResolver struct {
	Dependencies []ProvisionFunc
	env          v1alpha1.Environment
}

func NewDependencies(env v1alpha1.Environment) *DependencyResolver {
	return &DependencyResolver{
		env: env,
	}
}

func (d *DependencyResolver) withKubernetes() {
	switch d.env.Spec.Kubernetes.KubernetesInstaller {
	case "kubeadm":
		d.Dependencies = append(d.Dependencies, functions["kubeadm"])
	case "kind":
		d.Dependencies = append(d.Dependencies, functions["kind"])
	case "microk8s":
		// reset the list to only include microk8s
		d.Dependencies = nil
		d.Dependencies = append(d.Dependencies, functions["microk8s"])
	default:
		// default to kubeadm if KubernetesInstaller is empty
		d.Dependencies = append(d.Dependencies, functions["kubeadm"])
	}
}

func (d *DependencyResolver) withContainerRuntime() {
	switch d.env.Spec.ContainerRuntime.Name {
	case "containerd":
		d.Dependencies = append(d.Dependencies, functions["containerd"])
	case "crio":
		d.Dependencies = append(d.Dependencies, functions["crio"])
	case "docker":
		d.Dependencies = append(d.Dependencies, functions["docker"])
	default:
		// default to containerd if ContainerRuntime.Name is empty
		d.Dependencies = append(d.Dependencies, functions["containerd"])
	}
}

func (d *DependencyResolver) withContainerToolkit() {
	d.Dependencies = append(d.Dependencies, functions["containerToolkit"])
}

func (d *DependencyResolver) withNVDriver() {
	d.Dependencies = append(d.Dependencies, functions["nvdriver"])
}

// Resolve returns the dependency list in the correct order
func (d *DependencyResolver) Resolve() []ProvisionFunc {
	// Add NVDriver to the list
	if d.env.Spec.NVIDIADriver.Install {
		d.withNVDriver()
	}

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
