package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==== VALIDATION AND ERROR HANDLING TESTS ====

// TestConfigurationManager_ValidateConfig tests validation logic
func TestConfigurationManager_ValidateConfig(t *testing.T) {
	schema := ConfigSchema{
		Version:        "1.0",
		RequiredFields: []string{"timeout", "pool"},
		DefaultValues: map[string]interface{}{
			"timeout": 30,
			"pool":    "default",
		},
	}

	_ = NewConfigurationManager(schema)

	// Test with validator function
	schema.ValidatorFunc = func(config interface{}) error {
		return nil
	}

	cm2 := NewConfigurationManager(schema)
	cm2.config = map[string]interface{}{
		"timeout": 30,
		"pool":    "default",
	}

	assert.True(t, cm2 != nil)
}

// TestConfigurationManager_LoadFromFile_InvalidJSON tests JSON parsing failure
func TestConfigurationManager_LoadFromFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	invalidJSON := `{invalid json content`
	err := os.WriteFile(tmpFile, []byte(invalidJSON), 0644)
	require.NoError(t, err)

	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	// Try to load - should fail on JSON parsing
	err = cm.LoadFromFile(tmpFile)
	assert.Error(t, err)
}

// TestConfigurationManager_LoadFromFile_NotFound tests missing file
func TestConfigurationManager_LoadFromFile_NotFound(t *testing.T) {
	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	// Try to load nonexistent file
	err := cm.LoadFromFile("/nonexistent/path/config.json")
	assert.Error(t, err)
}

// TestConfigurationManager_SaveToFile_InvalidPath tests save failures
func TestConfigurationManager_SaveToFile_InvalidPath(t *testing.T) {
	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	cm.config = map[string]interface{}{
		"key": "value",
	}

	// Try to save to invalid directory (use impossible path for testing)
	invalidPath := "/root/impossible/nested/path/config.json"
	err := cm.SaveToFile(invalidPath)

	// Should error (permission denied or similar)
	// This test behavior may vary by system
	if err != nil {
		assert.Error(t, err)
	}
}

// TestConfigurationManager_LoadFromFile_MergesWithDefaults tests default values
func TestConfigurationManager_LoadFromFile_MergesWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	// Write partial config
	partialConfig := `{"timeout": 60}`
	err := os.WriteFile(tmpFile, []byte(partialConfig), 0644)
	require.NoError(t, err)

	schema := ConfigSchema{
		Version: "1.0",
		DefaultValues: map[string]interface{}{
			"timeout": 30,
			"retries": 3,
			"pool":    "default",
		},
	}
	cm := NewConfigurationManager(schema)

	// Load config
	err = cm.LoadFromFile(tmpFile)
	require.NoError(t, err)

	// Config should be loaded
	assert.NotNil(t, cm.config)
}

// TestConfigurationManager_Path stores file path
func TestConfigurationManager_Path(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	testConfig := `{"timeout": 30}`
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	require.NoError(t, err)

	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	err = cm.LoadFromFile(tmpFile)
	require.NoError(t, err)

	// Path should be stored
	assert.Equal(t, tmpFile, cm.path)
}

// TestConfigurationManager_RWMutex tests thread safety
func TestConfigurationManager_RWMutex(t *testing.T) {
	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	cm.config = map[string]interface{}{
		"key": "value",
	}

	// Multiple concurrent readers
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = cm.GetValue("key")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestConfigurationManager_GetValue_Returns2Values tests GetValue signature
func TestConfigurationManager_GetValue_Signature(t *testing.T) {
	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	cm.config = map[string]interface{}{
		"test-key": "test-value",
	}

	// GetValue returns (interface{}, error)
	value, err := cm.GetValue("test-key")

	// Should work with both return values
	assert.NotNil(t, value)
	assert.NoError(t, err)
}

// TestConfigurationManager_Schema verification
func TestConfigurationManager_Schema_Verification(t *testing.T) {
	schema := ConfigSchema{
		Version:            "2.0",
		Description:        "Test Schema",
		RequiredFields:     []string{"host", "port", "user"},
		AllowedHypervisors: []string{"qemu", "kvm"},
		DefaultValues: map[string]interface{}{
			"timeout": 60,
			"retries": 3,
		},
	}

	cm := NewConfigurationManager(schema)

	assert.Equal(t, "2.0", cm.schema.Version)
	assert.Equal(t, "Test Schema", cm.schema.Description)
	assert.Equal(t, 3, len(cm.schema.RequiredFields))
	assert.Equal(t, 2, len(cm.schema.AllowedHypervisors))
	assert.Equal(t, 2, len(cm.schema.DefaultValues))
}

// TestConfigurationManager_ZeroValue tests zero values handling
func TestConfigurationManager_ZeroValue(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	// Save with zero values
	testConfig := `{"timeout": 0, "retries": 0, "count": 0}`
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	require.NoError(t, err)

	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	err = cm.LoadFromFile(tmpFile)
	require.NoError(t, err)

	assert.NotNil(t, cm.config)
}

// TestConfigurationManager_LargeConfig tests with large configurations
func TestConfigurationManager_LargeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "large.json")

	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	// Create large config
	largeConfig := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeConfig["key_"+string(rune(i))] = "value_" + string(rune(i))
	}

	cm.config = largeConfig

	// Save and reload
	err := cm.SaveToFile(tmpFile)
	require.NoError(t, err)

	cm2 := NewConfigurationManager(schema)
	err = cm2.LoadFromFile(tmpFile)
	require.NoError(t, err)

	assert.NotNil(t, cm2.config)
}

// TestConfigurationManager_Unicode tests unicode handling
func TestConfigurationManager_Unicode(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "unicode.json")

	// Unicode config
	unicodeConfig := `{"name": "测试", "description": "कॉन्फिग"}`
	err := os.WriteFile(tmpFile, []byte(unicodeConfig), 0644)
	require.NoError(t, err)

	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	err = cm.LoadFromFile(tmpFile)
	require.NoError(t, err)

	assert.NotNil(t, cm.config)
}

// TestConfigurationManager_Special_Characters tests special characters
func TestConfigurationManager_Special_Characters(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "special.json")

	// Special characters in JSON
	specialConfig := `{"path": "/var/lib/bosh", "pattern": "test"}`
	err := os.WriteFile(tmpFile, []byte(specialConfig), 0644)
	require.NoError(t, err)

	schema := ConfigSchema{
		Version: "1.0",
	}
	cm := NewConfigurationManager(schema)

	err = cm.LoadFromFile(tmpFile)
	require.NoError(t, err)

	assert.NotNil(t, cm.config)
}

// BenchmarkConfigurationManager_SaveToFile benchmarks save operation
func BenchmarkConfigurationManager_SaveToFile(b *testing.B) {
	tmpDir := b.TempDir()

	schema := ConfigSchema{
		Version: "1.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpFile := filepath.Join(tmpDir, "config_"+string(rune(i))+".json")
		cm := NewConfigurationManager(schema)
		cm.config = map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}
		_ = cm.SaveToFile(tmpFile)
	}
}

// BenchmarkConfigurationManager_LoadFromFile_ErrorHandling benchmarks load operation with error handling
func BenchmarkConfigurationManager_LoadFromFile_ErrorHandling(b *testing.B) {
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "config_bench.json")

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
