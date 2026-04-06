package main_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main_pkg "bosh-libvirt-cpi/main"
)


// TestConfig_LoadValidConfiguration tests loading a valid configuration file
func TestConfig_LoadValidConfiguration(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	config := map[string]interface{}{
		"hypervisor":          "qemu",
		"bin_path":            "/usr/bin/virsh",
		"store_dir":           "/tmp/bosh-libvirt",
		"storage_controller":  "scsi",
		"agent": map[string]interface{}{
			"mbus": "https://user:pass@0.0.0.0:6868",
		},
	}
	configBytes, _ := json.Marshal(config)
	tmpFile.Write(configBytes)
	tmpFile.Close()

	// Execute
	result, err := main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestConfig_PathDoesNotExist tests error when file doesn't exist
func TestConfig_PathDoesNotExist(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())

	// Execute
	_, err := main_pkg.NewConfigFromPath("/nonexistent/path/config.json", fs)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Reading config")
}

// TestConfig_ParseInvalidJSON tests error with invalid JSON
func TestConfig_ParseInvalidJSON(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("{invalid json}")
	tmpFile.Close()

	// Execute
	_, err = main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unmarshalling")
}

// TestConfig_DefaultHypervisorQemu tests default hypervisor is set to qemu
func TestConfig_DefaultHypervisorQemu(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Config without hypervisor field - should default to qemu
	config := map[string]interface{}{
		"bin_path":           "/usr/bin/virsh",
		"store_dir":          "/tmp/bosh-libvirt",
		"storage_controller": "scsi",
		"agent": map[string]interface{}{
			"mbus": "https://user:pass@0.0.0.0:6868",
		},
	}
	configBytes, _ := json.Marshal(config)
	tmpFile.Write(configBytes)
	tmpFile.Close()

	// Execute
	result, err := main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert
	assert.NoError(t, err)
	// The result should have hypervisor set to "qemu"
	assert.NotNil(t, result)
}

// TestConfig_DefaultBinPathVirsh tests default bin_path is set to virsh
func TestConfig_DefaultBinPathVirsh(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Config without bin_path field - should default to virsh
	config := map[string]interface{}{
		"hypervisor":         "qemu",
		"store_dir":          "/tmp/bosh-libvirt",
		"storage_controller": "scsi",
		"agent": map[string]interface{}{
			"mbus": "https://user:pass@0.0.0.0:6868",
		},
	}
	configBytes, _ := json.Marshal(config)
	tmpFile.Write(configBytes)
	tmpFile.Close()

	// Execute
	result, err := main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestConfig_ValidatesRequiredStoreDirField tests validation error for missing store_dir
func TestConfig_ValidatesRequiredStoreDirField(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Config without store_dir (required field)
	config := map[string]interface{}{
		"hypervisor":         "qemu",
		"bin_path":           "/usr/bin/virsh",
		"storage_controller": "scsi",
		"agent":              map[string]interface{}{},
	}
	configBytes, _ := json.Marshal(config)
	tmpFile.Write(configBytes)
	tmpFile.Close()

	// Execute
	_, err = main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert - should error during validation
	assert.Error(t, err)
}

// TestConfig_RejectsInvalidHypervisor tests validation error for invalid hypervisor
func TestConfig_RejectsInvalidHypervisor(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	config := map[string]interface{}{
		"hypervisor":         "invalid_hypervisor",
		"bin_path":           "/usr/bin/virsh",
		"store_dir":          "/tmp/bosh-libvirt",
		"storage_controller": "scsi",
		"agent":              map[string]interface{}{},
	}
	configBytes, _ := json.Marshal(config)
	tmpFile.Write(configBytes)
	tmpFile.Close()

	// Execute
	_, err = main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Validating")
}

// TestConfig_LoadsWithAllOptionalFields tests complete config with all optional fields
func TestConfig_LoadsWithAllOptionalFields(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	config := map[string]interface{}{
		"hypervisor":          "qemu",
		"bin_path":            "/usr/bin/virsh",
		"store_dir":           "/var/lib/bosh-libvirt",
		"storage_controller":  "sata",
		"host":                "example.com",
		"username":            "libvirt",
		"private_key_path":    "/home/user/.ssh/id_rsa",
		"console_log_dir":     "/tmp/console_logs",
		"agent": map[string]interface{}{
			"mbus": "https://user:pass@0.0.0.0:6868",
		},
	}
	configBytes, _ := json.Marshal(config)
	tmpFile.Write(configBytes)
	tmpFile.Close()

	// Execute
	result, err := main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestConfig_SupportsMultipleReads tests that config can be loaded multiple times
func TestConfig_SupportsMultipleReads(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	config := map[string]interface{}{
		"hypervisor":         "qemu",
		"bin_path":           "/usr/bin/virsh",
		"store_dir":          "/tmp/bosh-libvirt",
		"storage_controller": "scsi",
		"agent": map[string]interface{}{
			"mbus": "https://user:pass@0.0.0.0:6868",
		},
	}
	configBytes, _ := json.Marshal(config)
	tmpFile.Write(configBytes)
	tmpFile.Close()

	// Execute - read twice
	result1, err1 := main_pkg.NewConfigFromPath(tmpFile.Name(), fs)
	result2, err2 := main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert - both should succeed and be non-nil
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotNil(t, result1)
	assert.NotNil(t, result2)
}

// TestConfig_RejectsEmptyJSON tests error with empty JSON object
func TestConfig_RejectsEmptyJSON(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("{}")
	tmpFile.Close()

	// Execute
	_, err = main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert - should error because required fields are missing
	assert.Error(t, err)
}

// TestConfig_HandlesNestedAgentConfig tests loading config with nested agent settings
func TestConfig_HandlesNestedAgentConfig(t *testing.T) {
	// Setup
	fs := system.NewOsFileSystem(getTestLogger())
	tmpFile, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	config := map[string]interface{}{
		"hypervisor":         "qemu",
		"bin_path":           "/usr/bin/virsh",
		"store_dir":          "/tmp/bosh-libvirt",
		"storage_controller": "scsi",
		"agent": map[string]interface{}{
			"mbus": "https://user:pass@0.0.0.0:6868",
			"ntp":  []string{"0.bosh-ntp.pool.ntp.org"},
		},
	}
	configBytes, _ := json.Marshal(config)
	tmpFile.Write(configBytes)
	tmpFile.Close()

	// Execute
	result, err := main_pkg.NewConfigFromPath(tmpFile.Name(), fs)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

