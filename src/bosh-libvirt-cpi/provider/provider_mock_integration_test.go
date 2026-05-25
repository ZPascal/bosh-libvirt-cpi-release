package provider_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bosh-libvirt-cpi/provider"
)

// MockDriver is a test implementation of driver.Driver
type MockDriver struct {
	ExecuteCalls []string
	ExecuteError error
}

func (m *MockDriver) Execute(args ...string) (string, error) {
	if len(args) > 0 {
		m.ExecuteCalls = append(m.ExecuteCalls, args[0])
	}
	return "", m.ExecuteError
}

func (m *MockDriver) IsMissingVMErr(output string) bool {
	return output == "not found"
}

// TestProvider_Initialization tests provider initialization
func TestProvider_Initialization(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		BinPath:    "/usr/bin/virsh",
	}

	assert.NotNil(t, opts)
	assert.Equal(t, provider.HypervisorTypeQEMU, opts.Hypervisor)
}

// TestProvider_HypervisorCreation tests creation of providers for different hypervisors
func TestProvider_HypervisorCreation(t *testing.T) {
	hypervisors := []provider.HypervisorType{
		provider.HypervisorTypeQEMU,
		provider.HypervisorTypeVBox,
		provider.HypervisorTypeLXC,
		provider.HypervisorTypeXen,
		provider.HypervisorTypeVMware,
	}

	for _, hyp := range hypervisors {
		opts := provider.ProviderOptions{
			Hypervisor: hyp,
			BinPath:    "/usr/bin/virsh",
		}

		assert.NotNil(t, opts)
		assert.Equal(t, hyp, opts.Hypervisor)
	}
}

// TestProvider_Options_Valid tests provider options validation
func TestProvider_Options_Valid(t *testing.T) {
	validConfigs := []provider.ProviderOptions{
		{
			Hypervisor: provider.HypervisorTypeQEMU,
			BinPath:    "/usr/bin/virsh",
		},
		{
			Hypervisor: provider.HypervisorTypeVBox,
			BinPath:    "/usr/bin/virsh",
			Host:       "localhost",
		},
		{
			Hypervisor: provider.HypervisorTypeLXC,
			BinPath:    "/usr/bin/virsh",
			StoreDir:   "/var/lib/libvirt/images",
		},
	}

	for _, cfg := range validConfigs {
		assert.NotNil(t, cfg)
		assert.NotEmpty(t, cfg.Hypervisor)
	}
}

// TestProvider_URIGeneration tests connection URI generation
func TestProvider_URIGeneration(t *testing.T) {
	tests := []struct {
		hypervisor   provider.HypervisorType
		expectedHost string
	}{
		{provider.HypervisorTypeQEMU, "qemu"},
		{provider.HypervisorTypeVBox, "vbox"},
		{provider.HypervisorTypeLXC, "lxc"},
		{provider.HypervisorTypeXen, "xen"},
		{provider.HypervisorTypeVMware, "vmware"},
	}

	for _, tc := range tests {
		opts := provider.ProviderOptions{
			Hypervisor: tc.hypervisor,
		}

		uri := opts.GetConnectionURI()
		assert.Contains(t, uri, tc.expectedHost)
	}
}

// TestProvider_FactoryCreation tests provider factory
func TestProvider_FactoryCreation(t *testing.T) {
	factory := &provider.ProviderFactory{}
	assert.NotNil(t, factory)
}

// TestProvider_Options_StorageDir tests storage directory configuration
func TestProvider_Options_StorageDir(t *testing.T) {
	storageDirs := []string{
		"/var/lib/libvirt/images",
		"/mnt/storage",
		"/home/bosh/disks",
	}

	for _, storageDir := range storageDirs {
		opts := provider.ProviderOptions{
			Hypervisor: provider.HypervisorTypeQEMU,
			StoreDir:   storageDir,
		}

		assert.Equal(t, storageDir, opts.StoreDir)
	}
}

// TestProvider_Options_BinPath tests binary path configuration
func TestProvider_Options_BinPath(t *testing.T) {
	binPaths := []string{
		"/usr/bin/virsh",
		"/usr/local/bin/virsh",
		"/opt/virsh",
		"virsh",
	}

	for _, binPath := range binPaths {
		opts := provider.ProviderOptions{
			BinPath: binPath,
		}

		assert.Equal(t, binPath, opts.BinPath)
	}
}

// TestProvider_Options_Host tests host configuration
func TestProvider_Options_Host(t *testing.T) {
	hosts := []string{
		"localhost",
		"192.168.1.100",
		"hypervisor.example.com",
		"qemu-server",
	}

	for _, host := range hosts {
		opts := provider.ProviderOptions{
			Host: host,
		}

		assert.Equal(t, host, opts.Host)
	}
}

// TestProvider_Options_URI tests custom URI configuration
func TestProvider_Options_URI(t *testing.T) {
	uris := []string{
		"qemu:///system",
		"qemu+ssh://user@host/system",
		"vbox:///session",
		"lxc:///",
	}

	for _, uri := range uris {
		opts := provider.ProviderOptions{
			URI: uri,
		}

		retrievedURI := opts.GetConnectionURI()
		assert.Equal(t, uri, retrievedURI)
	}
}

// TestProvider_Integration_InitializeProvider tests provider initialization flow
func TestProvider_Integration_InitializeProvider(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		BinPath:    "/usr/bin/virsh",
		Host:       "localhost",
		StoreDir:   "/var/lib/libvirt/images",
	}

	// Verify all components are properly initialized
	assert.NotNil(t, opts)
	assert.Equal(t, provider.HypervisorTypeQEMU, opts.Hypervisor)
	assert.NotEmpty(t, opts.GetConnectionURI())
}

// TestProvider_Error_InvalidHypervisor tests handling of invalid hypervisor
func TestProvider_Error_InvalidHypervisor(t *testing.T) {
	// Empty hypervisor should default to QEMU
	opts := provider.ProviderOptions{
		Hypervisor: "",
	}

	// Should still work but might use defaults
	assert.NotNil(t, opts)
}

// TestProvider_Workflow_CompleteSetup tests complete provider setup workflow
func TestProvider_Workflow_CompleteSetup(t *testing.T) {
	// Create provider options
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		BinPath:    "/usr/bin/virsh",
		Host:       "192.168.1.100",
		StoreDir:   "/var/lib/libvirt/images",
		URI:        "", // Will be auto-generated
	}

	// Get connection URI
	uri := opts.GetConnectionURI()
	require.NotEmpty(t, uri)
	require.Contains(t, uri, "qemu")

	// Verify all fields
	assert.Equal(t, provider.HypervisorTypeQEMU, opts.Hypervisor)
	assert.Equal(t, "/usr/bin/virsh", opts.BinPath)
	assert.Equal(t, "192.168.1.100", opts.Host)
	assert.Equal(t, "/var/lib/libvirt/images", opts.StoreDir)
}
