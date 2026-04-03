package vm_test

import (
	"errors"
	"regexp"
	"testing"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/stretchr/testify/assert"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/testhelpers/mocks"
	"bosh-libvirt-cpi/vm"
	bpds "bosh-libvirt-cpi/vm/portdevices"
)

// Helper to create test VM with mock driver
func createVMWithMockDriver(t *testing.T, mockDriver driver.Driver) vm.VMImpl {
	cid := apiv1.NewVMCID("test-vm-123")
	portDevices := bpds.NewPortDevices(nil)
	store := mocks.NewFakeStore()
	logger := boshlog.NewLogger(boshlog.LevelError)
	ctx := apiv1.CallContext{DirectorUUID: "test-director"}
	stemcellAPIVersion := apiv1.NewStemcellAPIVersion(ctx)

	return vm.NewVMImpl(cid, portDevices, store, stemcellAPIVersion, mockDriver, logger)
}

// Test VM.Exists() with valid VM
func TestVMExists_SuccessfulCheck(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `Id:           1
Name:         test-vm-123
Groups:       /
Memory:       4096 MiB`, nil
		}
		return "", nil
	}
	mockDriver.IsMissingVMErrFunc = func(output string) bool {
		return false
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	exists, err := vmImpl.Exists()

	assert.NoError(t, err)
	assert.True(t, exists)
}

// Test VM.Exists() with missing VM
func TestVMExists_VMNotFound(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return "error: The machine with UUID could not be found", errors.New("not found")
		}
		return "", nil
	}
	mockDriver.IsMissingVMErrFunc = func(output string) bool {
		return true
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	exists, err := vmImpl.Exists()

	assert.NoError(t, err)
	assert.False(t, exists)
}

// Test VM.Start() successful
func TestVMStart_SuccessfulStart(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteComplexFunc = func(args []string, opts driver.ExecuteOpts) (string, error) {
		if len(args) >= 2 && args[0] == "startvm" {
			return `VM "test-vm-123" has been successfully started`, nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	err := vmImpl.Start(false)

	assert.NoError(t, err)
}

// Test VM.Start() with headless mode
func TestVMStart_HeadlessMode(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	callArgs := []string{}
	mockDriver.ExecuteComplexFunc = func(args []string, opts driver.ExecuteOpts) (string, error) {
		callArgs = args
		if len(args) >= 4 && args[0] == "startvm" && args[2] == "--type" && args[3] == "headless" {
			return `VM "test-vm-123" has been successfully started`, nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	err := vmImpl.Start(false)

	assert.NoError(t, err)
	assert.Contains(t, callArgs, "headless")
}

// Test VM.Start() with GUI mode
func TestVMStart_GUIMode(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteComplexFunc = func(args []string, opts driver.ExecuteOpts) (string, error) {
		if len(args) >= 4 && args[0] == "startvm" && args[2] == "--type" && args[3] == "gui" {
			return `VM "test-vm-123" has been successfully started`, nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	err := vmImpl.Start(true)

	assert.NoError(t, err)
}

// Test VM.Reboot()
func TestVMReboot_Successful(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `VMState="running"`, nil
		}
		if len(args) >= 2 && args[0] == "controlvm" {
			return "", nil
		}
		return "", nil
	}
	mockDriver.ExecuteComplexFunc = func(args []string, opts driver.ExecuteOpts) (string, error) {
		if len(args) >= 2 && args[0] == "startvm" {
			return `VM "test-vm-123" has been successfully started`, nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	err := vmImpl.Reboot()

	assert.NoError(t, err)
}

// Test VM.IsRunning() returns true
func TestVMIsRunning_VMRunning(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `VMState="running"`, nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	isRunning, err := vmImpl.IsRunning()

	assert.NoError(t, err)
	assert.True(t, isRunning)
}

// Test VM.IsRunning() returns false
func TestVMIsRunning_VMStopped(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `VMState="poweroff"`, nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	isRunning, err := vmImpl.IsRunning()

	assert.NoError(t, err)
	assert.False(t, isRunning)
}

// Test VM.State() parsing
func TestVMState_ParsingRunning(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `VMState="running"`, nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	state, err := vmImpl.State()

	assert.NoError(t, err)
	assert.Equal(t, "running", state)
}

// Test VM.State() parsing poweroff
func TestVMState_ParsingPoweroff(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `VMState="poweroff"`, nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	state, err := vmImpl.State()

	assert.NoError(t, err)
	assert.Equal(t, "poweroff", state)
}

// Test VM.HaltIfRunning() halts running VM
func TestVMHaltIfRunning_VMRunning(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	haltCalled := false
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `VMState="running"`, nil
		}
		if len(args) >= 2 && args[0] == "controlvm" && args[2] == "poweroff" {
			haltCalled = true
			return "", nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	err := vmImpl.HaltIfRunning()

	assert.NoError(t, err)
	assert.True(t, haltCalled)
}

// Test VM.HaltIfRunning() with stopped VM
func TestVMHaltIfRunning_VMStopped(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	haltCalled := false
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `VMState="poweroff"`, nil
		}
		if len(args) >= 2 && args[0] == "controlvm" {
			haltCalled = true
			return "", nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	err := vmImpl.HaltIfRunning()

	assert.NoError(t, err)
	assert.False(t, haltCalled)
}

// Test VM.Delete()
func TestVMDelete_SuccessfulDeletion(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	undefineCalled := false
	mockDriver.ExecuteFunc = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "showvminfo" {
			return `VMState="poweroff"`, nil
		}
		if len(args) >= 2 && args[0] == "unregistervm" {
			undefineCalled = true
			return "", nil
		}
		return "", nil
	}

	vmImpl := createVMWithMockDriver(t, mockDriver)
	err := vmImpl.Delete()

	assert.NoError(t, err)
	assert.True(t, undefineCalled)
}

// Test VM.ID()
func TestVMID_ReturnsCorrectID(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	vmImpl := createVMWithMockDriver(t, mockDriver)

	id := vmImpl.ID()

	assert.Equal(t, "test-vm-123", id.AsString())
}

// Test VM.SetMetadata()
func TestVMSetMetadata_Successful(t *testing.T) {
	mockDriver := mocks.NewSimpleMockDriver()
	vmImpl := createVMWithMockDriver(t, mockDriver)

	metadata := apiv1.VMMeta{}
	err := vmImpl.SetMetadata(metadata)

	assert.NoError(t, err)
}

// Test state regex matching
func TestVMStateRegex_RunningPattern(t *testing.T) {
	pattern := regexp.MustCompile(`VMState="(.+?)"`)
	str := `VMState="running"`

	matches := pattern.FindStringSubmatch(str)
	assert.NotNil(t, matches)
	assert.Equal(t, "running", matches[1])
}

func TestVMStateRegex_PoweroffPattern(t *testing.T) {
	pattern := regexp.MustCompile(`VMState="(.+?)"`)
	str := `VMState="poweroff"`

	matches := pattern.FindStringSubmatch(str)
	assert.NotNil(t, matches)
	assert.Equal(t, "poweroff", matches[1])
}

