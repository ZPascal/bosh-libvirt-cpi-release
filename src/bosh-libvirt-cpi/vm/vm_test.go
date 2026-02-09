package vm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/testhelpers/mocks"
)

func TestVMImpl_ID(t *testing.T) {
	cid := apiv1.NewVMCID("test-vm-123")
	vm := createTestVM(cid)

	assert.Equal(t, cid, vm.ID())
}

func TestVMImpl_SetProps(t *testing.T) {
	t.Run("sets memory successfully", func(t *testing.T) {
		cid := apiv1.NewVMCID("test-vm")
		mockDriver := mocks.NewMockDriver()

		// Expect setmaxmem call
		mockDriver.On("Execute", []string{"setmaxmem", "test-vm", "2097152", "--config"}).
			Return("", nil)
		// Expect setmem call
		mockDriver.On("Execute", []string{"setmem", "test-vm", "2097152", "--config"}).
			Return("", nil)

		vm := createTestVMWithDriver(cid, mockDriver)
		props := VMProps{
			Memory:        2048, // 2GB
			CPUs:          0,
			EphemeralDisk: 5000,
		}

		err := vm.SetProps(props)

		require.NoError(t, err)
		mockDriver.AssertExpectations(t)
	})

	t.Run("sets CPUs successfully", func(t *testing.T) {
		cid := apiv1.NewVMCID("test-vm")
		mockDriver := mocks.NewMockDriver()

		// Expect setvcpus maximum call
		mockDriver.On("Execute", []string{"setvcpus", "test-vm", "4", "--config", "--maximum"}).
			Return("", nil)
		// Expect setvcpus call
		mockDriver.On("Execute", []string{"setvcpus", "test-vm", "4", "--config"}).
			Return("", nil)

		vm := createTestVMWithDriver(cid, mockDriver)
		props := VMProps{
			Memory:        0,
			CPUs:          4,
			EphemeralDisk: 5000,
		}

		err := vm.SetProps(props)

		require.NoError(t, err)
		mockDriver.AssertExpectations(t)
	})

	t.Run("sets both memory and CPUs", func(t *testing.T) {
		cid := apiv1.NewVMCID("test-vm")
		mockDriver := mocks.NewMockDriver()

		// Memory calls
		mockDriver.On("Execute", []string{"setmaxmem", "test-vm", "4194304", "--config"}).
			Return("", nil)
		mockDriver.On("Execute", []string{"setmem", "test-vm", "4194304", "--config"}).
			Return("", nil)
		// CPU calls
		mockDriver.On("Execute", []string{"setvcpus", "test-vm", "2", "--config", "--maximum"}).
			Return("", nil)
		mockDriver.On("Execute", []string{"setvcpus", "test-vm", "2", "--config"}).
			Return("", nil)

		vm := createTestVMWithDriver(cid, mockDriver)
		props := VMProps{
			Memory:        4096, // 4GB
			CPUs:          2,
			EphemeralDisk: 10000,
		}

		err := vm.SetProps(props)

		require.NoError(t, err)
		mockDriver.AssertExpectations(t)
	})

	t.Run("handles memory error", func(t *testing.T) {
		cid := apiv1.NewVMCID("test-vm")
		mockDriver := mocks.NewMockDriver()

		mockDriver.On("Execute", []string{"setmaxmem", "test-vm", "2097152", "--config"}).
			Return("", assert.AnError)

		vm := createTestVMWithDriver(cid, mockDriver)
		props := VMProps{Memory: 2048}

		err := vm.SetProps(props)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Setting max memory")
	})

	t.Run("handles CPU error", func(t *testing.T) {
		cid := apiv1.NewVMCID("test-vm")
		mockDriver := mocks.NewMockDriver()

		mockDriver.On("Execute", []string{"setvcpus", "test-vm", "4", "--config", "--maximum"}).
			Return("", assert.AnError)

		vm := createTestVMWithDriver(cid, mockDriver)
		props := VMProps{CPUs: 4}

		err := vm.SetProps(props)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Setting maximum vcpus")
	})
}

func TestVMImpl_SetMetadata(t *testing.T) {
	t.Run("marshals and saves metadata", func(t *testing.T) {
		cid := apiv1.NewVMCID("test-vm")
		mockStore := &mockStore{}
		vm := createTestVMWithStore(cid, mockStore)

		meta := apiv1.VMMeta{
			"director":   "bosh-director",
			"deployment": "test-deployment",
		}

		err := vm.SetMetadata(meta)

		require.NoError(t, err)
		assert.True(t, mockStore.putCalled)
		assert.Equal(t, "metadata.json", mockStore.putPath)
		assert.Contains(t, string(mockStore.putContents), "director")
		assert.Contains(t, string(mockStore.putContents), "test-deployment")
	})

	t.Run("handles store error", func(t *testing.T) {
		cid := apiv1.NewVMCID("test-vm")
		mockStore := &mockStore{putError: assert.AnError}
		vm := createTestVMWithStore(cid, mockStore)

		meta := apiv1.VMMeta{"key": "value"}
		err := vm.SetMetadata(meta)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Saving VM metadata")
	})
}

// Helper functions

func createTestVM(cid apiv1.VMCID) VMImpl {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	mockDriver := mocks.NewMockDriver()
	return createTestVMWithDriver(cid, mockDriver)
}

func createTestVMWithDriver(cid apiv1.VMCID, driver *mocks.MockDriver) VMImpl {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	mockStore := &mockStore{}

	return NewVMImpl(
		cid,
		nil, // portDevices
		mockStore,
		apiv1.StemcellAPIVersion(2),
		driver,
		logger,
	)
}

func createTestVMWithStore(cid apiv1.VMCID, store Store) VMImpl {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	mockDriver := mocks.NewMockDriver()

	return NewVMImpl(
		cid,
		nil,
		store,
		apiv1.StemcellAPIVersion(2),
		mockDriver,
		logger,
	)
}

// Mock Store implementation

type mockStore struct {
	putCalled    bool
	putPath      string
	putContents  []byte
	putError     error
	getCalled    bool
	getPath      string
	getContents  []byte
	getError     error
	deleteCalled bool
	deleteError  error
}

func (s *mockStore) Put(path string, contents []byte) error {
	s.putCalled = true
	s.putPath = path
	s.putContents = contents
	return s.putError
}

func (s *mockStore) Get(path string) ([]byte, error) {
	s.getCalled = true
	s.getPath = path
	return s.getContents, s.getError
}

func (s *mockStore) Delete() error {
	s.deleteCalled = true
	return s.deleteError
}

func (s *mockStore) Path(filename string) string {
	return "/tmp/test/" + filename
}
