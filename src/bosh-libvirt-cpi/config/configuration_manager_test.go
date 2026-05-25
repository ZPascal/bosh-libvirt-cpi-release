package config

import (
	"os"
	"testing"
)

func TestNewConfigurationManager(t *testing.T) {
	schema := ConfigSchema{
		Version:        "1.0",
		RequiredFields: []string{"hypervisor", "uri"},
	}

	cm := NewConfigurationManager(schema)

	if cm == nil {
		t.Errorf("expected non-nil configuration manager")
	}

	if cm.schema.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", cm.schema.Version)
	}
}

func TestConfigurationManager_SetConfig(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
		"uri":        "qemu:///system",
	}

	err := cm.SetConfig(config)
	if err != nil {
		t.Errorf("SetConfig failed: %v", err)
	}

	retrievedConfig := cm.GetConfig()
	if retrievedConfig == nil {
		t.Errorf("expected non-nil config")
	}
}

func TestConfigurationManager_SetConfig_Missing_RequiredField(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor", "uri"},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
	}

	err := cm.SetConfig(config)
	if err == nil {
		t.Errorf("expected error for missing required field")
	}
}

func TestConfigurationManager_GetValue(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
		"uri":        "qemu:///system",
	}

	cm.SetConfig(config)

	value, err := cm.GetValue("hypervisor")
	if err != nil {
		t.Errorf("GetValue failed: %v", err)
	}

	if value != "qemu" {
		t.Errorf("expected hypervisor to be 'qemu', got %v", value)
	}
}

func TestConfigurationManager_GetValue_NotFound(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
	}

	cm.SetConfig(config)

	_, err := cm.GetValue("nonexistent")
	if err == nil {
		t.Errorf("expected error for nonexistent key")
	}
}

func TestConfigurationManager_SetValue(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
	}

	cm.SetConfig(config)

	err := cm.SetValue("uri", "qemu:///system")
	if err != nil {
		t.Errorf("SetValue failed: %v", err)
	}

	value, _ := cm.GetValue("uri")
	if value != "qemu:///system" {
		t.Errorf("expected uri to be 'qemu:///system', got %v", value)
	}
}

func TestHypervisorConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  HypervisorConfig
		isValid bool
	}{
		{
			name: "valid_qemu_config",
			config: HypervisorConfig{
				Type:              "qemu",
				URI:               "qemu:///system",
				BinPath:           "virsh",
				StoreDir:          "/var/vcap/store",
				ConnectionTimeout: 30,
				RetryAttempts:     3,
			},
			isValid: true,
		},
		{
			name: "missing_type",
			config: HypervisorConfig{
				URI: "qemu:///system",
			},
			isValid: false,
		},
		{
			name: "invalid_type",
			config: HypervisorConfig{
				Type: "invalid",
				URI:  "qemu:///system",
			},
			isValid: false,
		},
		{
			name: "negative_timeout",
			config: HypervisorConfig{
				Type:              "qemu",
				URI:               "qemu:///system",
				ConnectionTimeout: -1,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.isValid && err != nil {
				t.Errorf("expected valid config, got error: %v", err)
			}
			if !tt.isValid && err == nil {
				t.Errorf("expected invalid config to fail validation")
			}
		})
	}
}

func TestConfigurationManager_MergeWithDefaults(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
		DefaultValues: map[string]interface{}{
			"uri":                "qemu:///system",
			"connection_timeout": 30,
		},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
	}

	cm.SetConfig(config)
	cm.MergeWithDefaults()

	retrievedConfig := cm.GetConfig().(map[string]interface{})

	if retrievedConfig["uri"] != "qemu:///system" {
		t.Errorf("expected uri to be merged from defaults")
	}

	if retrievedConfig["connection_timeout"] != 30 {
		t.Errorf("expected connection_timeout to be merged from defaults")
	}
}

func TestConfigurationManager_ValidateConfiguration(t *testing.T) {
	validatorCalled := false

	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
		ValidatorFunc: func(config interface{}) error {
			validatorCalled = true
			return nil
		},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
	}

	cm.SetConfig(config)
	cm.ValidateConfiguration()

	if !validatorCalled {
		t.Errorf("custom validator was not called")
	}
}

func TestConfigurationManager_Export(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
		"uri":        "qemu:///system",
	}

	cm.SetConfig(config)

	data, err := cm.Export()
	if err != nil {
		t.Errorf("Export failed: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("expected non-empty export data")
	}
}

func TestConfigurationManager_Import(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
	}

	cm := NewConfigurationManager(schema)

	jsonData := []byte(`{"hypervisor":"qemu","uri":"qemu:///system"}`)

	err := cm.Import(jsonData)
	if err != nil {
		t.Errorf("Import failed: %v", err)
	}

	config := cm.GetConfig().(map[string]interface{})
	if config["hypervisor"] != "qemu" {
		t.Errorf("expected hypervisor to be 'qemu'")
	}
}

func TestConfigurationManager_SaveToFile(t *testing.T) {
	schema := ConfigSchema{
		RequiredFields: []string{"hypervisor"},
	}

	cm := NewConfigurationManager(schema)

	config := map[string]interface{}{
		"hypervisor": "qemu",
		"uri":        "qemu:///system",
	}

	cm.SetConfig(config)

	// Create temp file
	tmpfile, err := os.CreateTemp("", "config_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	err = cm.SaveToFile(tmpfile.Name())
	if err != nil {
		t.Errorf("SaveToFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tmpfile.Name()); err != nil {
		t.Errorf("file was not created: %v", err)
	}
}

func TestHypervisorConfig_ValidTypes(t *testing.T) {
	validTypes := []string{"qemu", "vbox", "lxc", "xen", "vmware"}

	for _, hypervisorType := range validTypes {
		config := HypervisorConfig{
			Type: hypervisorType,
			URI:  "test:///system",
		}

		err := config.Validate()
		if err != nil {
			t.Errorf("validation failed for valid type %s: %v", hypervisorType, err)
		}
	}
}
