package main_test

import (
	"os"
	"testing"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/stretchr/testify/assert"

	main_pkg "bosh-libvirt-cpi/main"
)

func getTestLogger() logger.Logger {
	return logger.NewAsyncWriterLogger(logger.LevelDebug, os.Stderr)
}

// TestNewConfigFromPath_Success tests successful config loading
func TestNewConfigFromPath_Success(t *testing.T) {
	// Setup: Create a temporary config file
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.json"

	configContent := `{
		"libvirt": {
			"host": "qemu+unix:///system"
		},
		"cpi": {
			"host": "localhost",
			"port": 6868,
			"user": "vcap"
		}
	}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	// Execute: Load config
	log := getTestLogger()
	fs := system.NewOsFileSystem(log)
	config, err := main_pkg.NewConfigFromPath(configPath, fs)

	// Assert: Config loaded successfully
	assert.NoError(t, err)
	assert.NotNil(t, config)
}

// TestNewConfigFromPath_FileNotFound tests missing config file
func TestNewConfigFromPath_FileNotFound(t *testing.T) {
	// Execute: Try to load non-existent config
	log := getTestLogger()
	fs := system.NewOsFileSystem(log)
	_, err := main_pkg.NewConfigFromPath("/nonexistent/config.json", fs)

	// Assert: Error returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Reading config")
}

// TestNewConfigFromPath_InvalidJSON tests invalid config JSON
func TestNewConfigFromPath_InvalidJSON(t *testing.T) {
	// Setup: Create config file with invalid JSON
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.json"

	configContent := `{ invalid json }`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	// Execute: Try to load invalid config
	log := getTestLogger()
	fs := system.NewOsFileSystem(log)
	_, err = main_pkg.NewConfigFromPath(configPath, fs)

	// Assert: Error returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unmarshalling config")
}

// TestNewConfigFromPath_HypervisorDefault tests default hypervisor
func TestNewConfigFromPath_HypervisorDefault(t *testing.T) {
	// Setup: Create config without hypervisor specified
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.json"

	configContent := `{
		"cpi": {
			"host": "localhost"
		},
		"store_dir": "/tmp/libvirt-store"
	}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	// Execute: Load config
	log := getTestLogger()
	fs := system.NewOsFileSystem(log)
	config, err := main_pkg.NewConfigFromPath(configPath, fs)

	// Assert: Default hypervisor applied
	assert.NoError(t, err)
	assert.NotNil(t, config)
	// The hypervisor should be set to default value
	assert.NotEmpty(t, config.Hypervisor)
}

// TestNewConfigFromPath_MinimalConfig tests minimal valid config
func TestNewConfigFromPath_MinimalConfig(t *testing.T) {
	// Setup: Create minimal config
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.json"

	configContent := `{
		"store_dir": "/tmp/libvirt-store"
	}`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	// Execute: Load minimal config
	log := getTestLogger()
	fs := system.NewOsFileSystem(log)
	config, err := main_pkg.NewConfigFromPath(configPath, fs)

	// Assert: Minimal config loads successfully
	assert.NoError(t, err)
	assert.NotNil(t, config)
}

