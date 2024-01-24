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
	"fmt"
	"strings"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func Dryrun(log *logger.FunLogger, env v1alpha1.Environment) error {
	// Resolve dependencies from top to bottom
	log.Wg.Add(1)

	go log.Loading("Resolving dependencies \U0001F4E6 ...")
	// Kubernetes -> Container Toolkit -> Container Runtime -> NVDriver
	if env.Spec.Kubernetes.Install {
		if !env.Spec.ContainerRuntime.Install {
			log.Fail <- struct{}{}
			return fmt.Errorf("cannot install Kubernetes without a container runtime")
		}
		// check if env.Spec.Kubernetes.KubernetesVersion is in the format of vX.Y.Z
		if env.Spec.Kubernetes.KubernetesInstaller == "kubeadm" && !strings.HasPrefix(env.Spec.Kubernetes.KubernetesVersion, "v") {
			log.Fail <- struct{}{}
			return fmt.Errorf("kubernetes version %s is not in the format of vX.Y.Z", env.Spec.Kubernetes.KubernetesVersion)
		}
	}

	if env.Spec.ContainerRuntime.Install && (env.Spec.ContainerRuntime.Name != v1alpha1.ContainerRuntimeContainerd &&
		env.Spec.ContainerRuntime.Name != v1alpha1.ContainerRuntimeCrio &&
		env.Spec.ContainerRuntime.Name != v1alpha1.ContainerRuntimeDocker) {
		log.Fail <- struct{}{}
		return fmt.Errorf("container runtime %s not supported", env.Spec.ContainerRuntime.Name)
	}

	log.Done <- struct{}{}
	log.Wg.Wait()

	return nil
}
