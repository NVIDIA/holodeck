package instances

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	// Test with default cache path
	log := logger.NewLogger()
	manager := NewManager(log, "")
	expectedPath := filepath.Join(os.Getenv("HOME"), ".cache", "holodeck")
	assert.Equal(t, expectedPath, manager.cachePath)

	// Test with custom cache path
	customPath := "/tmp/holodeck-test"
	manager = NewManager(log, customPath)
	assert.Equal(t, customPath, manager.cachePath)
}

func TestGenerateInstanceID(t *testing.T) {
	log := logger.NewLogger()
	manager := NewManager(log, "")

	// Generate multiple IDs and verify they are unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := manager.GenerateInstanceID()
		assert.NotEmpty(t, id)
		assert.Len(t, id, 8) // 4 bytes = 8 hex characters
		assert.False(t, ids[id], "Generated duplicate ID: %s", id)
		ids[id] = true
	}
}

func TestGetInstanceCacheFile(t *testing.T) {
	log := logger.NewLogger()
	manager := NewManager(log, "/tmp/holodeck-test")
	instanceID := "test123"

	expectedPath := filepath.Join("/tmp/holodeck-test", instanceID+".yaml")
	assert.Equal(t, expectedPath, manager.GetInstanceCacheFile(instanceID))
}

func TestListInstances(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "holodeck-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove temp directory: %v", err)
		}
	}()

	log := logger.NewLogger()
	manager := NewManager(log, tempDir)

	// Create a test instance file
	instanceID := "test123"
	cacheFile := manager.GetInstanceCacheFile(instanceID)
	err = os.MkdirAll(filepath.Dir(cacheFile), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cacheFile, []byte(`apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-instance
  labels:
    holodeck-instance-id: test123
spec:
  provider: aws
`), 0600)
	require.NoError(t, err)

	// Test listing instances
	instances, err := manager.ListInstances()
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, instanceID, instances[0].ID)
	assert.Equal(t, "test-instance", instances[0].Name)
	assert.Equal(t, v1alpha1.ProviderAWS, instances[0].Provider)
}

func TestGetInstance(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "holodeck-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove temp directory: %v", err)
		}
	}()

	log := logger.NewLogger()
	manager := NewManager(log, tempDir)

	// Create a test instance file
	instanceID := "test123"
	cacheFile := manager.GetInstanceCacheFile(instanceID)
	err = os.MkdirAll(filepath.Dir(cacheFile), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cacheFile, []byte(`apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-instance
  labels:
    holodeck-instance-id: test123
spec:
  provider: aws
`), 0600)
	require.NoError(t, err)

	// Test getting instance
	instance, err := manager.GetInstance(instanceID)
	require.NoError(t, err)
	assert.Equal(t, instanceID, instance.ID)
	assert.Equal(t, "test-instance", instance.Name)
	assert.Equal(t, v1alpha1.ProviderAWS, instance.Provider)

	// Test getting non-existent instance
	_, err = manager.GetInstance("nonexistent")
	assert.Error(t, err)
}

func TestDeleteInstance(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "holodeck-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove temp directory: %v", err)
		}
	}()

	log := logger.NewLogger()
	manager := NewManager(log, tempDir)

	// Create a test instance file
	instanceID := "test123"
	cacheFile := manager.GetInstanceCacheFile(instanceID)
	err = os.MkdirAll(filepath.Dir(cacheFile), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cacheFile, []byte(`apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-instance
  labels:
    holodeck-instance-id: test123
spec:
  provider: aws
`), 0600)
	require.NoError(t, err)

	// Test deleting instance
	err = manager.DeleteInstance(instanceID)
	require.NoError(t, err)

	// Verify file is deleted
	_, err = os.Stat(cacheFile)
	assert.True(t, os.IsNotExist(err))

	// Test deleting non-existent instance
	err = manager.DeleteInstance("nonexistent")
	assert.Error(t, err)
}

func TestGetInstanceByFilename(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "holodeck-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove temp directory: %v", err)
		}
	}()

	log := logger.NewLogger()
	manager := NewManager(log, tempDir)

	// Create a test instance file with UUID filename
	instanceID := "123e4567-e89b-12d3-a456-426614174000"
	cacheFile := manager.GetInstanceCacheFile(instanceID)
	err = os.MkdirAll(filepath.Dir(cacheFile), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cacheFile, []byte(`apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-instance
  labels:
    holodeck-instance-id: test123
spec:
  provider: aws
`), 0600)
	require.NoError(t, err)

	// Test getting instance by filename
	instance, err := manager.GetInstanceByFilename(instanceID)
	require.NoError(t, err)
	assert.Equal(t, instanceID, instance.ID)
	assert.Equal(t, "test-instance", instance.Name)
	assert.Equal(t, v1alpha1.ProviderAWS, instance.Provider)

	// Test getting instance with invalid filename
	_, err = manager.GetInstanceByFilename("invalid")
	assert.Error(t, err)
}
