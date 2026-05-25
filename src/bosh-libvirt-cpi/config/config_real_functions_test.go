package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==== REAL FUNCTIONING TESTS ====

// TestConfigurationManager_Creation creates a real ConfigurationManager
func TestConfigurationManager_Creation(t *testing.T) {
	schema := ConfigSchema{
		Version:        "1.0",
		Description:    "Test Config",
		DefaultValues:  map[string]interface{}{},
		RequiredFields: []string{},
	}

	cm := NewConfigurationManager(schema)

	assert.NotNil(t, cm)
	assert.Equal(t, "1.0", cm.schema.Version)
	assert.Equal(t, "Test Config", cm.schema.Description)
}

// TestConfigurationManager_LoadFromFile tests loading actual config from file
func TestConfigurationManager_LoadFromFile(t *testing.T) {
	// Create temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	// Write test config
	testConfig := `{"timeout": 30, "retries": 3}`
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	require.NoError(t, err)

	// Create ConfigurationManager
	schema := ConfigSchema{
		Version:        "1.0",
		DefaultValues:  map[string]interface{}{},
		RequiredFields: []string{},
	}
	cm := NewConfigurationManager(schema)

	// Load from file
	err = cm.LoadFromFile(tmpFile)
	require.NoError(t, err)

	// Verify config loaded
	assert.NotNil(t, cm.config)
}

// TestConfigurationManager_SaveToFileExtended tests saving config to file (extended version)
func TestConfigurationManager_SaveToFileExtended(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "output2.json")

	schema := ConfigSchema{
		Version:        "1.0",
		DefaultValues:  map[string]interface{}{},
		RequiredFields: []string{},
	}
	cm := NewConfigurationManager(schema)

	// Set config with multiple fields
	cm.config = map[string]interface{}{
		"timeout":    120,
		"retries":    10,
		"hostname":   "test-host",
		"nested":     map[string]interface{}{"key": "value"},
	}

	// Save to file
	err := cm.SaveToFile(tmpFile)
	require.NoError(t, err)

	// Verify file exists and has content
	assert.True(t, fileExists(tmpFile))

	data, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Greater(t, len(data), 10)  // Should have reasonable content
}

// TestConfigurationManager_SaveWithoutConfig tests SaveToFile with no config
func TestConfigurationManager_SaveWithoutConfig(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "output.json")

	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	// Try to save without config - should fail
	err := cm.SaveToFile(tmpFile)
	assert.Error(t, err)
}

// TestConfigurationManager_GetValueExtended tests retrieval of configuration values
func TestConfigurationManager_GetValueExtended(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	testConfig := `{"timeout": 30, "name": "test-config"}`
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	require.NoError(t, err)

	schema := ConfigSchema{
		Version:        "1.0",
		DefaultValues:  map[string]interface{}{},
		RequiredFields: []string{},
	}
	cm := NewConfigurationManager(schema)

	err = cm.LoadFromFile(tmpFile)
	require.NoError(t, err)

	// Get values
	value, err := cm.GetValue("timeout")
	assert.NoError(t, err)
	assert.NotNil(t, value)

	nameValue, err := cm.GetValue("name")
	assert.NoError(t, err)
	assert.NotNil(t, nameValue)
}

// TestConfigurationManager_Watchers tests watcher functionality
func TestConfigurationManager_Watchers(t *testing.T) {
	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	// Watchers list should be initialized
	assert.Equal(t, 0, len(cm.watchers))

	// After creation watchers should exist
	assert.NotNil(t, cm.watchers)
}

// TestConfigurationManager_Schema verifies schema storage
func TestConfigurationManager_Schema(t *testing.T) {
	schema := ConfigSchema{
		Version:            "2.0",
		Description:        "Production Config",
		RequiredFields:     []string{"hostname", "port"},
		AllowedHypervisors: []string{"qemu", "xen"},
	}

	cm := NewConfigurationManager(schema)

	assert.Equal(t, "2.0", cm.schema.Version)
	assert.Equal(t, "Production Config", cm.schema.Description)
	assert.Equal(t, 2, len(cm.schema.RequiredFields))
	assert.Equal(t, 2, len(cm.schema.AllowedHypervisors))
}

// TestConfigurationManager_ConcurrentAccess tests thread-safe access
func TestConfigurationManager_ConcurrentAccess(t *testing.T) {
	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	cm.config = map[string]interface{}{
		"key": "value",
	}

	// Multiple readers should be safe
	done := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		go func() {
			_, _ = cm.GetValue("key")
			done <- true
		}()
	}

	for i := 0; i < 3; i++ {
		<-done
	}
}

// ==== HELPER FUNCTION ====

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ==== INTEGRATION TESTS ====

// TestConfigurationManager_FullWorkflow tests complete config workflow
func TestConfigurationManager_FullWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	// Create manager
	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	// Set config
	cm.config = map[string]interface{}{
		"timeout":  30,
		"retries":  3,
		"hostname": "localhost",
	}

	// Save to file
	err := cm.SaveToFile(tmpFile)
	require.NoError(t, err)

	// Create new manager and load
	cm2 := NewConfigurationManager(schema)
	err = cm2.LoadFromFile(tmpFile)
	require.NoError(t, err)

	// Verify values
	assert.NotNil(t, cm2.config)
}

// TestConfigurationManager_CreateDirectory tests directory creation
func TestConfigurationManager_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "config", "output.json")

	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	cm.config = map[string]interface{}{
		"test": "value",
	}

	// Save to nested directory that doesn't exist
	err := cm.SaveToFile(nestedPath)
	require.NoError(t, err)

	// Verify directory was created
	assert.True(t, fileExists(nestedPath))
}

// BenchmarkConfigurationManager_GetValue benchmarks value retrieval
func BenchmarkConfigurationManager_GetValue(b *testing.B) {
	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)
	cm.config = map[string]interface{}{
		"key": "value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cm.GetValue("key")
	}
}

// BenchmarkConfigurationManager_LoadFromFile benchmarks file loading
func BenchmarkConfigurationManager_LoadFromFile(b *testing.B) {
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	testConfig := `{"timeout": 30, "retries": 3}`
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	require.NoError(b, err)

	schema := ConfigSchema{
		Version: "1.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm := NewConfigurationManager(schema)
		_ = cm.LoadFromFile(tmpFile)
	}
}
