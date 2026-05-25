package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ConfigurationManager manages application configuration with validation and hot-reload support
type ConfigurationManager struct {
	mu       sync.RWMutex
	config   interface{}
	schema   ConfigSchema
	watchers []ConfigWatcher
	path     string
}

// ConfigSchema defines the structure and validation rules for configuration
type ConfigSchema struct {
	Version            string
	Description        string
	ValidatorFunc      func(interface{}) error
	DefaultValues      map[string]interface{}
	RequiredFields     []string
	AllowedHypervisors []string
}

// ConfigWatcher is called when configuration changes
type ConfigWatcher func(oldConfig, newConfig interface{}) error

// NewConfigurationManager creates a new configuration manager
func NewConfigurationManager(schema ConfigSchema) *ConfigurationManager {
	return &ConfigurationManager{
		schema:   schema,
		watchers: make([]ConfigWatcher, 0),
	}
}

// LoadFromFile loads configuration from a JSON file
func (cm *ConfigurationManager) LoadFromFile(path string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cm.validateConfig(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	oldConfig := cm.config
	cm.config = config
	cm.path = path

	// Notify watchers
	for _, watcher := range cm.watchers {
		if err := watcher(oldConfig, config); err != nil {
			return fmt.Errorf("watcher error: %w", err)
		}
	}

	return nil
}

// SaveToFile saves configuration to a JSON file
func (cm *ConfigurationManager) SaveToFile(path string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.config == nil {
		return fmt.Errorf("no configuration to save")
	}

	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfig returns the current configuration
func (cm *ConfigurationManager) GetConfig() interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// SetConfig sets the configuration after validation
func (cm *ConfigurationManager) SetConfig(config interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if err := cm.validateConfig(config); err != nil {
		return err
	}

	oldConfig := cm.config
	cm.config = config

	// Notify watchers
	for _, watcher := range cm.watchers {
		if err := watcher(oldConfig, config); err != nil {
			return err
		}
	}

	return nil
}

// GetValue retrieves a specific configuration value
func (cm *ConfigurationManager) GetValue(key string) (interface{}, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}

	configMap, ok := cm.config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("configuration is not a map")
	}

	value, exists := configMap[key]
	if !exists {
		return nil, fmt.Errorf("key '%s' not found in configuration", key)
	}

	return value, nil
}

// SetValue sets a specific configuration value
func (cm *ConfigurationManager) SetValue(key string, value interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.config == nil {
		cm.config = make(map[string]interface{})
	}

	configMap, ok := cm.config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("configuration is not a map")
	}

	oldValue := configMap[key]
	configMap[key] = value

	// Re-validate
	if err := cm.validateConfig(cm.config); err != nil {
		// Restore old value on validation error
		if oldValue != nil {
			configMap[key] = oldValue
		} else {
			delete(configMap, key)
		}
		return err
	}

	return nil
}

// ValidateConfiguration validates the configuration
func (cm *ConfigurationManager) ValidateConfiguration() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.validateConfig(cm.config)
}

// validateConfig performs internal validation
func (cm *ConfigurationManager) validateConfig(config interface{}) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	configMap, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("configuration must be a map")
	}

	// Check required fields
	for _, field := range cm.schema.RequiredFields {
		if _, exists := configMap[field]; !exists {
			return fmt.Errorf("required field '%s' not found in configuration", field)
		}
	}

	// Run custom validator if provided
	if cm.schema.ValidatorFunc != nil {
		if err := cm.schema.ValidatorFunc(config); err != nil {
			return err
		}
	}

	return nil
}

// AddWatcher adds a configuration watcher
func (cm *ConfigurationManager) AddWatcher(watcher ConfigWatcher) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.watchers = append(cm.watchers, watcher)
}

// MergeWithDefaults merges the current configuration with default values
func (cm *ConfigurationManager) MergeWithDefaults() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.config == nil {
		cm.config = make(map[string]interface{})
	}

	configMap, ok := cm.config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("configuration must be a map")
	}

	// Merge defaults
	for key, value := range cm.schema.DefaultValues {
		if _, exists := configMap[key]; !exists {
			configMap[key] = value
		}
	}

	return cm.validateConfig(cm.config)
}

// Export exports configuration as JSON bytes
func (cm *ConfigurationManager) Export() ([]byte, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.config == nil {
		return nil, fmt.Errorf("no configuration to export")
	}

	return json.MarshalIndent(cm.config, "", "  ")
}

// Import imports configuration from JSON bytes
func (cm *ConfigurationManager) Import(data []byte) error {
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse configuration: %w", err)
	}

	return cm.SetConfig(config)
}

// HypervisorConfig represents configuration for a specific hypervisor
type HypervisorConfig struct {
	Type               string            `json:"type"`
	URI                string            `json:"uri"`
	BinPath            string            `json:"bin_path"`
	StoreDir           string            `json:"store_dir"`
	StorageController  string            `json:"storage_controller"`
	AutoEnableNetworks bool              `json:"auto_enable_networks"`
	ConnectionTimeout  int               `json:"connection_timeout_seconds"`
	RetryAttempts      int               `json:"retry_attempts"`
	CustomParams       map[string]string `json:"custom_params"`
}

// Validate validates the hypervisor configuration
func (hc *HypervisorConfig) Validate() error {
	if hc.Type == "" {
		return fmt.Errorf("hypervisor type is required")
	}

	if hc.URI == "" {
		return fmt.Errorf("hypervisor URI is required")
	}

	validTypes := []string{"qemu", "vbox", "lxc", "xen", "vmware"}
	valid := false
	for _, t := range validTypes {
		if hc.Type == t {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("invalid hypervisor type: %s", hc.Type)
	}

	if hc.ConnectionTimeout < 0 {
		return fmt.Errorf("connection timeout must be non-negative")
	}

	if hc.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts must be non-negative")
	}

	return nil
}

// ConfigEnvironment holds environment-specific settings
type ConfigEnvironment struct {
	Development HypervisorConfig `json:"development"`
	Staging     HypervisorConfig `json:"staging"`
	Production  HypervisorConfig `json:"production"`
}

// ValidateEnvironment validates the environment configuration
func (ce *ConfigEnvironment) ValidateEnvironment() error {
	if err := ce.Development.Validate(); err != nil {
		return fmt.Errorf("development config invalid: %w", err)
	}

	if err := ce.Staging.Validate(); err != nil {
		return fmt.Errorf("staging config invalid: %w", err)
	}

	if err := ce.Production.Validate(); err != nil {
		return fmt.Errorf("production config invalid: %w", err)
	}

	return nil
}

// GetConfigForEnvironment returns the configuration for a specific environment
func (ce *ConfigEnvironment) GetConfigForEnvironment(env string) (*HypervisorConfig, error) {
	switch env {
	case "development":
		return &ce.Development, nil
	case "staging":
		return &ce.Staging, nil
	case "production":
		return &ce.Production, nil
	default:
		return nil, fmt.Errorf("unknown environment: %s", env)
	}
}
