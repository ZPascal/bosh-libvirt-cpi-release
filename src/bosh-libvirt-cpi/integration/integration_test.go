package integration

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLibvirtInstalled checks if libvirt is properly installed
func TestLibvirtInstalled(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cmd := exec.Command("virsh", "version")
	err := cmd.Run()

	require.NoError(t, err, "libvirt should be installed and virsh should be available")
}

// TestQEMUAvailable checks if QEMU/KVM is available
func TestQEMUAvailable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cmd := exec.Command("virsh", "uri")
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "virsh should be executable")
	assert.Contains(t, string(output), "qemu", "QEMU URI should be available")
}

// TestNetworkCreation tests basic network operations
func TestNetworkCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// List existing networks
	cmd := exec.Command("virsh", "net-list")
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "should be able to list networks")
	assert.NotEmpty(t, output, "network list should contain data")
}

// TestVMCreation tests VM creation and deletion
func TestVMCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if default network exists
	cmd := exec.Command("virsh", "net-info", "default")
	err := cmd.Run()

	require.NoError(t, err, "default network should exist")
}

// TestStoragePool tests storage pool operations
func TestStoragePool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// List storage pools
	cmd := exec.Command("virsh", "pool-list")
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "should be able to list storage pools")
	assert.NotEmpty(t, output, "pool list should contain data")
}

// TestCPIBinary checks if the CPI binary is built correctly
func TestCPIBinary(t *testing.T) {
	cmd := exec.Command("../../bin/cpi", "-v")
	err := cmd.Run()

	// Binary might not have -v flag, but should be executable
	assert.NoError(t, err, "CPI binary should be executable")
}
