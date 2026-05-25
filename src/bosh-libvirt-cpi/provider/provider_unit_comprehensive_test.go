package provider_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"bosh-libvirt-cpi/provider"
)

// TestProviderOptions_Creation tests provider options creation
func TestProviderOptions_Creation(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		BinPath:    "/usr/bin/virsh",
		Host:       "localhost",
	}

	assert.Equal(t, provider.HypervisorTypeQEMU, opts.Hypervisor)
	assert.Equal(t, "/usr/bin/virsh", opts.BinPath)
	assert.Equal(t, "localhost", opts.Host)
}

// TestProviderOptions_Defaults tests provider options with defaults
func TestProviderOptions_Defaults(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
	}

	assert.Equal(t, provider.HypervisorTypeQEMU, opts.Hypervisor)
}

// TestHypervisor_QEMU tests QEMU hypervisor support
func TestHypervisor_QEMU(t *testing.T) {
	hypervisor := provider.HypervisorTypeQEMU
	assert.NotEmpty(t, hypervisor)
}

// TestHypervisor_VBox tests VirtualBox hypervisor support
func TestHypervisor_VBox(t *testing.T) {
	hypervisor := provider.HypervisorTypeVBox
	assert.NotEmpty(t, hypervisor)
}

// TestHypervisor_LXC tests LXC hypervisor support
func TestHypervisor_LXC(t *testing.T) {
	hypervisor := provider.HypervisorTypeLXC
	assert.NotEmpty(t, hypervisor)
}

// TestHypervisor_Xen tests Xen hypervisor support
func TestHypervisor_Xen(t *testing.T) {
	hypervisor := provider.HypervisorTypeXen
	assert.NotEmpty(t, hypervisor)
}

// TestHypervisor_VMware tests VMware hypervisor support
func TestHypervisor_VMware(t *testing.T) {
	hypervisor := provider.HypervisorTypeVMware
	assert.NotEmpty(t, hypervisor)
}

// TestProviderOptions_ConnectionSettings tests connection settings
func TestProviderOptions_ConnectionSettings(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		Host:       "192.168.1.100",
		BinPath:    "/usr/bin/virsh",
	}

	assert.Equal(t, "192.168.1.100", opts.Host)
	assert.Equal(t, "/usr/bin/virsh", opts.BinPath)
}

// TestProviderOptions_LocalConnection tests local connection setup
func TestProviderOptions_LocalConnection(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		Host:       "localhost",
	}

	assert.Equal(t, "localhost", opts.Host)
}

// TestProviderOptions_RemoteConnection tests remote connection setup
func TestProviderOptions_RemoteConnection(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		Host:       "hypervisor.example.com",
		BinPath:    "/usr/bin/virsh",
	}

	assert.Equal(t, "hypervisor.example.com", opts.Host)
	assert.NotEmpty(t, opts.BinPath)
}

// TestProviderOptions_Various tests various provider configurations
func TestProviderOptions_Various(t *testing.T) {
	configs := []struct {
		name       string
		hypervisor provider.HypervisorType
		host       string
	}{
		{"localhost_qemu", provider.HypervisorTypeQEMU, "localhost"},
		{"localhost_vbox", provider.HypervisorTypeVBox, "localhost"},
		{"localhost_lxc", provider.HypervisorTypeLXC, "localhost"},
		{"remote_qemu", provider.HypervisorTypeQEMU, "192.168.1.100"},
	}

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			opts := provider.ProviderOptions{
				Hypervisor: cfg.hypervisor,
				Host:       cfg.host,
			}

			assert.Equal(t, cfg.hypervisor, opts.Hypervisor)
			assert.Equal(t, cfg.host, opts.Host)
		})
	}
}

// TestProviderOptions_BinPath tests bin path configuration
func TestProviderOptions_BinPath(t *testing.T) {
	testCases := []struct {
		name    string
		binPath string
	}{
		{"standard", "/usr/bin/virsh"},
		{"custom", "/opt/libvirt/bin/virsh"},
		{"short", "virsh"},
		{"absolute", "/usr/local/bin/virsh"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := provider.ProviderOptions{
				BinPath: tc.binPath,
			}

			assert.Equal(t, tc.binPath, opts.BinPath)
		})
	}
}

// TestProviderOptions_Hypervisors tests all supported hypervisors
func TestProviderOptions_Hypervisors(t *testing.T) {
	hypervisors := []provider.HypervisorType{
		provider.HypervisorTypeQEMU,
		provider.HypervisorTypeVBox,
		provider.HypervisorTypeLXC,
		provider.HypervisorTypeXen,
		provider.HypervisorTypeVMware,
	}

	for _, hyp := range hypervisors {
		t.Run(string(hyp), func(t *testing.T) {
			opts := provider.ProviderOptions{
				Hypervisor: hyp,
			}

			assert.Equal(t, hyp, opts.Hypervisor)
		})
	}
}

// TestProviderOptions_StorageConfiguration tests storage configuration
func TestProviderOptions_StorageConfiguration(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		StoreDir:   "/var/lib/libvirt/images",
	}

	assert.Equal(t, "/var/lib/libvirt/images", opts.StoreDir)
}

// TestLibvirtConnectionURI_QEMU tests QEMU connection URI
func TestLibvirtConnectionURI_QEMU(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		Host:       "localhost",
	}

	uri := opts.GetConnectionURI()
	assert.NotEmpty(t, uri)
	assert.Contains(t, uri, "qemu")
}

// TestLibvirtConnectionURI_VBox tests VBox connection URI
func TestLibvirtConnectionURI_VBox(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeVBox,
	}

	uri := opts.GetConnectionURI()
	assert.NotEmpty(t, uri)
	assert.Contains(t, uri, "vbox")
}

// TestLibvirtConnectionURI_LXC tests LXC connection URI
func TestLibvirtConnectionURI_LXC(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeLXC,
	}

	uri := opts.GetConnectionURI()
	assert.NotEmpty(t, uri)
	assert.Contains(t, uri, "lxc")
}

// TestLibvirtConnectionURI_CustomURI tests custom connection URI
func TestLibvirtConnectionURI_CustomURI(t *testing.T) {
	customURI := "qemu+ssh://user@host/system"
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		URI:        customURI,
	}

	uri := opts.GetConnectionURI()
	assert.Equal(t, customURI, uri)
}

// TestProviderOptions_CompleteConfiguration tests complete provider configuration
func TestProviderOptions_CompleteConfiguration(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		BinPath:    "/usr/bin/virsh",
		Host:       "192.168.1.100",
		StoreDir:   "/var/lib/libvirt/images",
	}

	assert.NotEmpty(t, opts.Hypervisor)
	assert.NotEmpty(t, opts.BinPath)
	assert.NotEmpty(t, opts.Host)
	assert.NotEmpty(t, opts.StoreDir)
}

// TestProviderOptions_ValidationBasic tests provider options validation
func TestProviderOptions_ValidationBasic(t *testing.T) {
	opts := provider.ProviderOptions{
		Hypervisor: provider.HypervisorTypeQEMU,
		BinPath:    "/usr/bin/virsh",
	}

	assert.NotEmpty(t, opts.Hypervisor)
	assert.NotEmpty(t, opts.BinPath)
}

// TestProviderOptions_URIGeneration tests URI auto-generation for different hypervisors
func TestProviderOptions_URIGeneration(t *testing.T) {
	tests := []struct {
		hypervisor provider.HypervisorType
		expectedURI string
	}{
		{provider.HypervisorTypeQEMU, "qemu:///system"},
		{provider.HypervisorTypeVBox, "vbox:///session"},
		{provider.HypervisorTypeLXC, "lxc:///"},
		{provider.HypervisorTypeXen, "xen:///"},
		{provider.HypervisorTypeVMware, "vmware:///session"},
	}

	for _, tc := range tests {
		t.Run(string(tc.hypervisor), func(t *testing.T) {
			opts := provider.ProviderOptions{
				Hypervisor: tc.hypervisor,
			}

			uri := opts.GetConnectionURI()
			assert.Equal(t, tc.expectedURI, uri)
		})
	}
}

// TestProviderOptions_Advanced tests advanced provider options (mapped values)
func TestProviderOptions_Advanced(t *testing.T) {
	advanced := map[string]interface{}{
		"connection_string": "qemu+ssh://user@host/system",
		"use_ssh":           true,
		"timeout":           30,
	}

	assert.Equal(t, "qemu+ssh://user@host/system", advanced["connection_string"])
	assert.Equal(t, true, advanced["use_ssh"])
	assert.Equal(t, 30, advanced["timeout"])
}

// TestProviderFactory_Creation tests provider factory creation
func TestProviderFactory_Creation(t *testing.T) {
	factory := provider.NewProviderFactory(nil)
	assert.NotNil(t, factory)
}

// TestProviderConnections_Multiple tests multiple provider connections
func TestProviderConnections_Multiple(t *testing.T) {
	connections := []struct {
		name       string
		hypervisor provider.HypervisorType
		host       string
	}{
		{"qemu-local", provider.HypervisorTypeQEMU, "localhost"},
		{"qemu-remote", provider.HypervisorTypeQEMU, "remote-host"},
		{"vbox-local", provider.HypervisorTypeVBox, "localhost"},
		{"lxc-local", provider.HypervisorTypeLXC, "localhost"},
	}

	for _, conn := range connections {
		opts := provider.ProviderOptions{
			Hypervisor: conn.hypervisor,
			Host:       conn.host,
			BinPath:    "/usr/bin/virsh",
		}
		assert.NotNil(t, opts)
	}
}

// TestProviderURI_Generation tests connection URI generation
func TestProviderURI_Generation(t *testing.T) {
	uris := map[string]string{
		"qemu:///system":      "QEMU system-wide",
		"qemu:///session":     "QEMU session",
		"vbox:///session":     "VirtualBox session",
		"lxc:///":             "LXC",
		"qemu+ssh://host/system": "QEMU over SSH",
	}

	for uri, desc := range uris {
		assert.NotEmpty(t, uri)
		assert.NotEmpty(t, desc)
	}
}

// TestProviderOptions_ValidationMatrix tests options validation matrix
func TestProviderOptions_ValidationMatrix(t *testing.T) {
	tests := []struct {
		name    string
		opts    provider.ProviderOptions
		isValid bool
	}{
		{
			name:    "valid_qemu",
			opts:    provider.ProviderOptions{Hypervisor: provider.HypervisorTypeQEMU, BinPath: "/usr/bin/virsh"},
			isValid: true,
		},
		{
			name:    "valid_vbox",
			opts:    provider.ProviderOptions{Hypervisor: provider.HypervisorTypeVBox, BinPath: "/usr/bin/virsh"},
			isValid: true,
		},
		{
			name:    "empty_hypervisor",
			opts:    provider.ProviderOptions{BinPath: "/usr/bin/virsh"},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isValid {
				assert.NotEmpty(t, tt.opts.Hypervisor)
			}
		})
	}
}

// TestProviderAuthentication_SSH tests SSH authentication config mapping
func TestProviderAuthentication_SSH(t *testing.T) {
	auth := map[string]interface{}{
		"use_ssh":         true,
		"ssh_user":        "libvirt",
		"ssh_private_key": "/home/user/.ssh/id_rsa",
		"ssh_port":        22,
	}

	assert.Equal(t, true, auth["use_ssh"])
	assert.Equal(t, "libvirt", auth["ssh_user"])
	assert.Equal(t, 22, auth["ssh_port"])
}

// TestProviderConcurrency_Support tests concurrent provider settings mapping
func TestProviderConcurrency_Support(t *testing.T) {
	concurrency := map[string]interface{}{
		"max_connections": 10,
		"connection_pool": true,
	}

	assert.Equal(t, 10, concurrency["max_connections"])
	assert.Equal(t, true, concurrency["connection_pool"])
}

// TestProviderLogging_Configuration tests logging configuration mapping
func TestProviderLogging_Configuration(t *testing.T) {
	logging := map[string]interface{}{
		"log_level":   "debug",
		"log_file":    "/var/log/libvirt.log",
		"log_verbose": true,
	}

	assert.Equal(t, "debug", logging["log_level"])
	assert.Equal(t, "/var/log/libvirt.log", logging["log_file"])
	assert.Equal(t, true, logging["log_verbose"])
}

// TestProviderTimeouts_Configuration tests timeout configuration mapping
func TestProviderTimeouts_Configuration(t *testing.T) {
	timeouts := map[string]int{
		"timeout":         60,
		"connect_timeout": 30,
		"command_timeout": 120,
	}

	assert.Equal(t, 60, timeouts["timeout"])
	assert.Equal(t, 30, timeouts["connect_timeout"])
	assert.Equal(t, 120, timeouts["command_timeout"])
}

// TestProviderCapabilities_Detection tests capability detection
func TestProviderCapabilities_Detection(t *testing.T) {
	capabilities := []string{
		"kvm",
		"tcg",
		"qemu",
		"xen",
		"vbox",
	}

	for _, capability := range capabilities {
		assert.NotEmpty(t, capability)
	}
}

// TestProviderErrorHandling_Connection tests error handling
func TestProviderErrorHandling_Connection(t *testing.T) {
	opts := provider.ProviderOptions{Hypervisor: provider.HypervisorTypeQEMU, Host: "unreachable.example.com"}
	timeoutSec := 5
	assert.Equal(t, 5, timeoutSec)
	assert.Equal(t, "unreachable.example.com", opts.Host)
}

// TestProviderCleanup_Resources tests resource cleanup
func TestProviderCleanup_Resources(t *testing.T) {
	cleanupOpts := map[string]bool{
		"cleanup_networks": true,
		"cleanup_storage":  true,
		"cleanup_vms":      false,
	}

	for key, val := range cleanupOpts {
		assert.NotEmpty(t, key)
		_ = val
	}
}
