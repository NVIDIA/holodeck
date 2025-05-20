/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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

package instances

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// InstanceLabelKey is the key used to label AWS instances
	InstanceLabelKey = "holodeck-instance-id"
	// InstanceProvisionedLabelKey is the key used to label AWS instances with their provisioning status
	InstanceProvisionedLabelKey = "holodeck-instance-provisioned"
)

// Instance represents a running Holodeck instance
type Instance struct {
	ID          string
	Name        string
	Provider    v1alpha1.Provider
	CreatedAt   time.Time
	Status      string
	CacheFile   string
	Provisioned bool
}

// Manager handles instance operations
type Manager struct {
	log       *logger.FunLogger
	cachePath string
}

// NewManager creates a new instance manager
func NewManager(log *logger.FunLogger, cachePath string) *Manager {
	if cachePath == "" {
		cachePath = filepath.Join(os.Getenv("HOME"), ".cache", "holodeck")
	}
	return &Manager{
		log:       log,
		cachePath: cachePath,
	}
}

// GenerateInstanceID creates a unique ID for a new instance
func (m *Manager) GenerateInstanceID() string {
	b := make([]byte, 4) // 4 bytes = 8 hex characters
	_, err := rand.Read(b)
	if err != nil {
		m.log.Error(err)
		return ""
	}
	return hex.EncodeToString(b)
}

// GetInstanceCacheFile returns the cache file path for an instance
func (m *Manager) GetInstanceCacheFile(instanceID string) string {
	return filepath.Join(m.cachePath, instanceID+".yaml")
}

// getProviderStatus retrieves the status of an instance from its provider
func (m *Manager) getProviderStatus(env v1alpha1.Environment, cacheFile string) string {
	status := "unknown"
	if env.Spec.Provider == v1alpha1.ProviderAWS {
		client, err := aws.New(m.log, env, cacheFile)
		if err != nil {
			m.log.Warning("Failed to create AWS provider for status check: %v", err)
			return status
		}
		conditions, err := client.Status()
		if err != nil {
			m.log.Warning("Failed to get instance status: %v", err)
			return status
		}
		// Check conditions in order of priority
		statusFound := false
		for _, condition := range conditions {
			if statusFound {
				break
			}
			switch condition.Type {
			case v1alpha1.ConditionTerminated:
				if condition.Status == metav1.ConditionTrue {
					status = "terminated"
					statusFound = true
				}
			case v1alpha1.ConditionDegraded:
				if condition.Status == metav1.ConditionTrue {
					status = "degraded"
					statusFound = true
				}
			case v1alpha1.ConditionProgressing:
				if condition.Status == metav1.ConditionTrue {
					status = "progressing"
					statusFound = true
				}
			case v1alpha1.ConditionAvailable:
				if condition.Status == metav1.ConditionTrue {
					status = "running"
					statusFound = true
				}
			}
		}
	}
	return status
}

// ListInstances returns all running instances
func (m *Manager) ListInstances() ([]Instance, error) {
	var instances []Instance

	// Read all cache files
	files, err := os.ReadDir(m.cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache directory: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".yaml" {
			continue
		}

		instanceID := file.Name()[:len(file.Name())-5] // Remove .yaml extension
		cacheFile := filepath.Join(m.cachePath, file.Name())

		env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](cacheFile)
		if err != nil {
			m.log.Warning("Failed to read cache file %s: %v", cacheFile, err)
			continue
		}

		// Get instance status from provider
		status := m.getProviderStatus(env, cacheFile)

		// Get file info for creation time
		fileInfo, err := os.Stat(cacheFile)
		if err != nil {
			m.log.Warning("Failed to get file info for %s: %v", cacheFile, err)
			continue
		}

		// For old cache files, use the filename as the ID
		// This ensures backward compatibility with existing cache files
		if env.Labels == nil || env.Labels[InstanceLabelKey] == "" {
			// Try to use the filename as the ID if it looks like a UUID
			if len(instanceID) == 36 { // UUID length
				env.Labels = make(map[string]string)
				env.Labels[InstanceLabelKey] = instanceID
			} else {
				// Skip files that don't have an instance ID and aren't UUIDs
				m.log.Warning("Found old cache file without instance ID, skipping: %s", cacheFile)
				continue
			}
		}

		instance := Instance{
			ID:          env.Labels[InstanceLabelKey],
			Name:        env.Name,
			Provider:    env.Spec.Provider,
			CreatedAt:   fileInfo.ModTime(),
			Status:      status,
			CacheFile:   cacheFile,
			Provisioned: env.Labels[InstanceProvisionedLabelKey] == "true",
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// GetInstance returns details for a specific instance
func (m *Manager) GetInstance(instanceID string) (*Instance, error) {
	cacheFile := m.GetInstanceCacheFile(instanceID)

	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %v", err)
	}

	fileInfo, err := os.Stat(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	// Get instance status from provider
	status := m.getProviderStatus(env, cacheFile)

	return &Instance{
		ID:        instanceID,
		Name:      env.Name,
		Provider:  env.Spec.Provider,
		CreatedAt: fileInfo.ModTime(),
		Status:    status,
		CacheFile: cacheFile,
	}, nil
}

// DeleteInstance removes an instance
func (m *Manager) DeleteInstance(instanceID string) error {
	cacheFile := m.GetInstanceCacheFile(instanceID)

	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](cacheFile)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %v", err)
	}

	// Delete resources based on provider
	switch env.Spec.Provider {
	case v1alpha1.ProviderAWS:
		client, err := aws.New(m.log, env, cacheFile)
		if err != nil {
			return fmt.Errorf("failed to create AWS provider: %v", err)
		}
		if err := client.Delete(); err != nil {
			return fmt.Errorf("failed to delete AWS resources: %v", err)
		}
	case v1alpha1.ProviderSSH:
		m.log.Info("SSH infrastructure cleanup not implemented")
	}

	// Remove cache file
	if err := os.Remove(cacheFile); err != nil {
		return fmt.Errorf("failed to remove cache file: %v", err)
	}

	return nil
}

// GetInstanceByFilename returns details for a specific instance by its filename
func (m *Manager) GetInstanceByFilename(filename string) (*Instance, error) {
	cacheFile := m.GetInstanceCacheFile(filename)

	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %v", err)
	}

	fileInfo, err := os.Stat(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	// Get instance status from provider
	status := m.getProviderStatus(env, cacheFile)

	// For old cache files, use the filename as the ID if it looks like a UUID
	instanceID := filename
	if len(filename) != 36 { // Not a UUID
		return nil, fmt.Errorf("invalid instance ID format")
	}

	return &Instance{
		ID:        instanceID,
		Name:      env.Name,
		Provider:  env.Spec.Provider,
		CreatedAt: fileInfo.ModTime(),
		Status:    status,
		CacheFile: cacheFile,
	}, nil
}
