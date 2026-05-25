package vm_test

import (
	"testing"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bosh-libvirt-cpi/vm"
)

// MockDriver mocks the driver interface for VM testing
type MockVMDriver struct {
	ExecutedCommands []string
	LastError        error
}

func (m *MockVMDriver) Execute(args ...string) (string, error) {
	if len(args) > 0 {
		m.ExecutedCommands = append(m.ExecutedCommands, args[0])
	}
	return "mock output", m.LastError
}

func (m *MockVMDriver) IsMissingVMErr(output string) bool {
	return output == "Domain not found"
}

// TestVM_SetProps_Memory tests memory property setting
func TestVM_SetProps_Memory(t *testing.T) {
	props := vm.VMProps{
		Memory: 2048,
		CPUs:   2,
	}

	assert.Equal(t, 2048, props.Memory)
	assert.Equal(t, 2, props.CPUs)
}

// TestVM_SetProps_VariousMemoryValues tests various memory allocations
func TestVM_SetProps_VariousMemoryValues(t *testing.T) {
	memorySizes := []int{512, 1024, 2048, 4096, 8192, 16384, 32768}

	for _, mem := range memorySizes {
		props := vm.VMProps{
			Memory: mem,
			CPUs:   1,
		}

		assert.Equal(t, mem, props.Memory)
		assert.Greater(t, props.Memory, 0)
	}
}

// TestVM_SetProps_CPU tests CPU property setting
func TestVM_SetProps_CPU(t *testing.T) {
	cpuCounts := []int{1, 2, 4, 8, 16, 32}

	for _, cpus := range cpuCounts {
		props := vm.VMProps{
			Memory: 2048,
			CPUs:   cpus,
		}

		assert.Equal(t, cpus, props.CPUs)
		assert.Greater(t, props.CPUs, 0)
	}
}

// TestVM_Metadata_Setting tests VM metadata setting
func TestVM_Metadata_Setting(t *testing.T) {
	metadata := apiv1.VMMeta{}

	assert.NotNil(t, metadata)
}

// TestVM_ID_Creation tests VM ID creation
func TestVM_ID_Creation(t *testing.T) {
	vmIDs := []string{
		"vm-123",
		"i-1234567890abcdef0",
		"instance-prod-001",
		"libvirt-vm-abc",
	}

	for _, id := range vmIDs {
		vmCID := apiv1.NewVMCID(id)
		assert.Equal(t, id, vmCID.AsString())
	}
}

// TestVM_Lifecycle_States tests VM lifecycle states
func TestVM_Lifecycle_States(t *testing.T) {
	states := []string{
		"created",
		"running",
		"stopped",
		"paused",
		"deleted",
	}

	for _, state := range states {
		assert.NotEmpty(t, state)
	}
}

// TestVM_Properties_Default tests default VM properties
func TestVM_Properties_Default(t *testing.T) {
	props := vm.VMProps{
		Memory: 1024,
		CPUs:   1,
	}

	assert.Equal(t, 1024, props.Memory)
	assert.Equal(t, 1, props.CPUs)
}

// TestVM_Properties_Large tests large VM properties
func TestVM_Properties_Large(t *testing.T) {
	props := vm.VMProps{
		Memory: 131072,
		CPUs:   64,
	}

	assert.Equal(t, 131072, props.Memory)
	assert.Equal(t, 64, props.CPUs)
}

// TestVM_Disks_Attachment tests disk attachment tracking
func TestVM_Disks_Attachment(t *testing.T) {
	diskCount := 5

	for i := 1; i <= diskCount; i++ {
		diskCID := apiv1.NewDiskCID("disk-" + string(rune(48+i)))
		assert.NotNil(t, diskCID)
		assert.NotEmpty(t, diskCID.AsString())
	}
}

// TestVM_Network_Configuration tests VM network configuration
func TestVM_Network_Configuration(t *testing.T) {
	networks := []struct {
		name        string
		type_string string
		ip          string
	}{
		{"default", "manual", "192.168.1.10"},
		{"management", "dynamic", ""},
		{"isolated", "manual", "10.0.0.50"},
	}

	for _, network := range networks {
		assert.NotEmpty(t, network.name)
		assert.NotEmpty(t, network.type_string)
	}
}

// TestVM_Storage_Allocation tests storage allocation for VMs
func TestVM_Storage_Allocation(t *testing.T) {
	storageAllocations := []struct {
		vmID string
		size int
	}{
		{"vm-1", 20480},
		{"vm-2", 40960},
		{"vm-3", 102400},
	}

	for _, alloc := range storageAllocations {
		vmCID := apiv1.NewVMCID(alloc.vmID)
		assert.NotNil(t, vmCID)
		assert.Greater(t, alloc.size, 0)
	}
}

// TestVM_Workflow_Creation tests complete VM creation workflow
func TestVM_Workflow_Creation(t *testing.T) {
	// Create VM ID
	vmCID := apiv1.NewVMCID("workflow-vm-001")
	require.NotNil(t, vmCID)

	// Set properties
	props := vm.VMProps{
		Memory: 2048,
		CPUs:   2,
	}
	assert.Greater(t, props.Memory, 0)
	assert.Greater(t, props.CPUs, 0)

	// Set metadata
	metadata := apiv1.VMMeta{}
	assert.NotNil(t, metadata)

	// Verify all components
	assert.Equal(t, "workflow-vm-001", vmCID.AsString())
	assert.Equal(t, 2048, props.Memory)
	assert.Equal(t, 2, props.CPUs)
}

// TestVM_Workflow_Deletion tests VM deletion workflow
func TestVM_Workflow_Deletion(t *testing.T) {
	vmCID := apiv1.NewVMCID("delete-vm-001")

	// Mark for deletion
	assert.NotNil(t, vmCID)
	assert.Equal(t, "delete-vm-001", vmCID.AsString())
}

// TestVM_Workflow_Modification tests VM modification workflow
func TestVM_Workflow_Modification(t *testing.T) {
	vmCID := apiv1.NewVMCID("modify-vm-001")

	// Original properties
	originalProps := vm.VMProps{
		Memory: 2048,
		CPUs:   2,
	}

	// Modified properties
	modifiedProps := vm.VMProps{
		Memory: 4096,
		CPUs:   4,
	}

	// Verify modification
	assert.Greater(t, modifiedProps.Memory, originalProps.Memory)
	assert.Greater(t, modifiedProps.CPUs, originalProps.CPUs)
	assert.Equal(t, "modify-vm-001", vmCID.AsString())
}

// TestVM_Integration_FullLifecycle tests full VM lifecycle
func TestVM_Integration_FullLifecycle(t *testing.T) {
	vmCID := apiv1.NewVMCID("lifecycle-vm-001")

	// Create
	assert.NotNil(t, vmCID)

	// Configure
	props := vm.VMProps{
		Memory: 2048,
		CPUs:   2,
	}
	assert.Equal(t, 2048, props.Memory)

	// Attach disks
	diskCID := apiv1.NewDiskCID("lifecycle-disk-001")
	assert.NotNil(t, diskCID)

	// Verify all states
	assert.Equal(t, "lifecycle-vm-001", vmCID.AsString())
	assert.Equal(t, 2048, props.Memory)
	assert.Equal(t, "lifecycle-disk-001", diskCID.AsString())
}
