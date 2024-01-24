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

type ProvisionFunc func(tpl *bytes.Buffer, env v1alpha1.Environment) error

type DependencyGraph map[string][]string

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

// buildDependencyGraph builds a dependency graph based on the environment
// and returns a topologically sorted list of provisioning functions
// to be executed in an opinionated order
func buildDependencyGraph(env v1alpha1.Environment) ([]ProvisionFunc, error) {
	//  Predefined dependency graph
	graph := DependencyGraph{
		"kubeadm":          {},
		"containerToolkit": {"containerToolkit", "nvdriver"},
	}

	// for kubeadm
	if env.Spec.ContainerRuntime.Name == v1alpha1.ContainerRuntimeContainerd {
		graph["kubeadm"] = append(graph["kubeadm"], "containerd")
	} else if env.Spec.ContainerRuntime.Name == v1alpha1.ContainerRuntimeCrio {
		graph["kubeadm"] = append(graph["kubeadm"], "crio")
	} else if env.Spec.ContainerRuntime.Name == v1alpha1.ContainerRuntimeDocker {
		graph["kubeadm"] = append(graph["kubeadm"], "docker")
	} else if env.Spec.ContainerRuntime.Name == v1alpha1.ContainerRuntimeNone {
		// default to containerd if ContainerRuntime is empty
		graph["kubeadm"] = append(graph["kubeadm"], "containerd")
	}

	// if container toolkit is enabled then add container toolkit and nvdriver to kubeadm
	if env.Spec.NVContainerToolKit.Install {
		graph["kubeadm"] = append(graph["kubeadm"], "containerToolkit")
		graph["kubeadm"] = append(graph["kubeadm"], "nvdriver")
	}

	// for container toolkit
	if env.Spec.ContainerRuntime.Name == v1alpha1.ContainerRuntimeContainerd {
		graph["containerToolkit"] = append(graph["containerToolkit"], "containerd")
	} else if env.Spec.ContainerRuntime.Name == v1alpha1.ContainerRuntimeCrio {
		graph["containerToolkit"] = append(graph["containerToolkit"], "crio")
	} else if env.Spec.ContainerRuntime.Name == v1alpha1.ContainerRuntimeDocker {
		graph["containerToolkit"] = append(graph["containerToolkit"], "docker")
	} else if env.Spec.ContainerRuntime.Name == v1alpha1.ContainerRuntimeNone {
		// default to containerd if ContainerRuntime is empty
		graph["kubeadm"] = append(graph["kubeadm"], "containerd")
	}

	// user might request to install container toolkit without nvdriver for testing purpose
	if env.Spec.NVDriver.Install {
		graph["containerToolkit"] = append(graph["containerToolkit"], "nvdriver")
	}

	ordered := []ProvisionFunc{}
	// We go from up to bottom in the graph
	// Kubernetes -> Container Toolkit -> Container Runtime -> NVDriver
	// if a dependency is needed and not defined, we set an opinionated default
	if env.Spec.Kubernetes.Install {
		switch env.Spec.Kubernetes.KubernetesInstaller {
		case "kubeadm":
			for _, f := range graph["kubeadm"] {
				ordered = append(ordered, functions[f])
			}
			return ordered, nil
		case "kind":
			return []ProvisionFunc{docker, containerToolkit, nvdriver, kind}, nil
		case "microk8s":
			return []ProvisionFunc{microk8s}, nil
		default:
			// default to kubeadm if KubernetesInstaller is empty
			for _, f := range graph["kubeadm"] {
				ordered = append(ordered, functions[f])
			}
			return ordered, nil
		}
	}

	// If no kubernetes is requested, we move to container-toolkit
	if env.Spec.NVContainerToolKit.Install {
		for _, f := range graph["containerToolkit"] {
			ordered = append(ordered, functions[f])
		}
		return ordered, nil
	}

	// If no container-toolkit is requested, we move to container-runtime
	if env.Spec.ContainerRuntime.Install {
		switch env.Spec.ContainerRuntime.Name {
		case "containerd":
			ordered = append(ordered, functions["containerd"])
			return ordered, nil
		case "crio":
			return ordered, nil
		case "docker":
			return ordered, nil
		default:
			// default to containerd if ContainerRuntime.Name is empty
			return ordered, nil
		}
	}

	// If no container-runtime is requested, we move to nvdriver
	if env.Spec.NVDriver.Install {
		ordered = append(ordered, functions["nvdriver"])
		return ordered, nil
	}

	return nil, nil
}

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
