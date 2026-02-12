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
		id, err := manager.GenerateInstanceID()
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.Len(t, id, 8) // 4 bytes = 8 hex characters
		assert.Regexp(t, `^[0-9a-f]{8}$`, id)
		assert.False(t, ids[id], "Generated duplicate ID: %s", id)
		ids[id] = true
	}
}

func TestGetInstanceCacheFile(t *testing.T) {
	log := logger.NewLogger()
	manager := NewManager(log, "/tmp/holodeck-test")

	// Valid 8-char hex ID
	instanceID := "a1b2c3d4"
	expectedPath := filepath.Join("/tmp/holodeck-test", instanceID+".yaml")
	path, err := manager.GetInstanceCacheFile(instanceID)
	require.NoError(t, err)
	assert.Equal(t, expectedPath, path)

	// Valid UUID (backward compatibility)
	uuidID := "123e4567-e89b-12d3-a456-426614174000"
	expectedUUIDPath := filepath.Join("/tmp/holodeck-test", uuidID+".yaml")
	path, err = manager.GetInstanceCacheFile(uuidID)
	require.NoError(t, err)
	assert.Equal(t, expectedUUIDPath, path)
}

func TestGetInstanceCacheFile_RejectsTraversal(t *testing.T) {
	log := logger.NewLogger()
	manager := NewManager(log, t.TempDir())

	_, err := manager.GetInstanceCacheFile("../../etc/passwd")
	assert.Error(t, err, "should reject path traversal")
}

func TestGetInstanceCacheFile_RejectsInvalidFormat(t *testing.T) {
	log := logger.NewLogger()
	manager := NewManager(log, t.TempDir())

	tests := []struct {
		name string
		id   string
	}{
		{"empty string", ""},
		{"too short hex", "abcdef"},
		{"shell injection", "a1b2c3d4; rm -rf /"},
		{"uppercase hex", "A1B2C3D4"},
		{"non-hex characters", "test1234"},
		{"path separator", "a1b2/c3d4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.GetInstanceCacheFile(tt.id)
			assert.Error(t, err, "should reject ID: %q", tt.id)
		})
	}
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

	// Create a test instance file with valid 8-char hex ID
	instanceID := "a1b2c3d4"
	cacheFile, err := manager.GetInstanceCacheFile(instanceID)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Dir(cacheFile), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cacheFile, []byte(`apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-instance
  labels:
    holodeck-instance-id: a1b2c3d4
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

	// Create a test instance file with valid hex ID
	instanceID := "a1b2c3d4"
	cacheFile, err := manager.GetInstanceCacheFile(instanceID)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Dir(cacheFile), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cacheFile, []byte(`apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-instance
  labels:
    holodeck-instance-id: a1b2c3d4
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

	// Test getting instance with invalid ID format
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

	// Create a test instance file with valid hex ID
	instanceID := "a1b2c3d4"
	cacheFile, err := manager.GetInstanceCacheFile(instanceID)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Dir(cacheFile), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cacheFile, []byte(`apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-instance
  labels:
    holodeck-instance-id: a1b2c3d4
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

	// Test deleting instance with invalid ID format
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

	// Create a test instance file with UUID filename (backward compatibility)
	instanceID := "123e4567-e89b-12d3-a456-426614174000"
	cacheFile, err := manager.GetInstanceCacheFile(instanceID)
	require.NoError(t, err)
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
