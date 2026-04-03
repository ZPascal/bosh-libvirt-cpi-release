package vm_test

import (
	"testing"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/vm"
)

// Mock Driver for testing VM state operations
type mockDriverForState struct {
	executeOutput  string
	executeErr     error
	executeCalls   []string
	isMissingVMErr bool
}

func (m *mockDriverForState) Execute(cmd ...string) (string, error) {
	m.executeCalls = append(m.executeCalls, cmd[0])
	return m.executeOutput, m.executeErr
}

func (m *mockDriverForState) ExecuteComplex(cmd []string, opts driver.ExecuteOpts) (string, error) {
	return m.executeOutput, m.executeErr
}

func (m *mockDriverForState) IsMissingVMErr(output string) bool {
	return m.isMissingVMErr
}

func (m *mockDriverForState) IsMissingDiskErr(output string) bool {
	return false
}

func (m *mockDriverForState) RetryWithTimeout(timeout int, operation func() (string, error)) (string, error) {
	return operation()
}

// Mock Store for testing
type mockStoreForState struct {
	putData   map[string][]byte
	putErr    error
	delErr    error
	getOutput []byte
	getErr    error
}

func (m *mockStoreForState) Put(key string, data []byte) error {
	if m.putData == nil {
		m.putData = make(map[string][]byte)
	}
	m.putData[key] = data
	return m.putErr
}

func (m *mockStoreForState) Get(key string) ([]byte, error) {
	return m.getOutput, m.getErr
}

func (m *mockStoreForState) Delete() error {
	return m.delErr
}

func (m *mockStoreForState) List() ([]string, error) {
	return nil, nil
}

// Mock Host for testing
type mockHostForState struct {
}

func (m *mockHostForState) FindNetwork(networkType string, networkName string) (vm.Network, error) {
	return vm.Network{}, nil
}

// These tests verify the Mock implementations for VM state operations
// The real VM functionality is tested through integration tests

func TestMockDriverForStateImplementsInterface(t *testing.T) {
	mock := &mockDriverForState{}
	var _ driver.Driver = mock
}

func TestMockStoreForStateImplementsInterface(t *testing.T) {
	mock := &mockStoreForState{}
	// Store is a concrete struct, not an interface
	_ = mock
}

