package stemcell

import (
	"bosh-libvirt-cpi/testhelpers/mocks"
	"errors"
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStemcellImpl_ID(t *testing.T) {
	cid := apiv1.NewStemcellCID("test-stemcell-123")
	stemcell := createTestStemcell(cid)
	assert.Equal(t, cid, stemcell.ID())
}
func TestStemcellImpl_Exists(t *testing.T) {
	t.Run("returns true when domain exists", func(t *testing.T) {
		cid := apiv1.NewStemcellCID("existing-stemcell")
		mockDriver := mocks.NewMockDriver()
		mockDriver.On("Execute", []string{"dominfo", "existing-stemcell"}).
			Return("State: shut off\nMemory: 512 MB", nil)
		stemcell := createTestStemcellWithDriver(cid, mockDriver)
		exists, err := stemcell.Exists()
		require.NoError(t, err)
		assert.True(t, exists)
		mockDriver.AssertExpectations(t)
	})
	t.Run("returns false when domain not found", func(t *testing.T) {
		cid := apiv1.NewStemcellCID("missing-stemcell")
		mockDriver := mocks.NewMockDriver()
		testErr := errors.New("domain not found")
		mockDriver.On("Execute", []string{"dominfo", "missing-stemcell"}).
			Return("error: failed to get domain", testErr)
		mockDriver.On("IsMissingVMErr", "error: failed to get domain").
			Return(true)
		stemcell := createTestStemcellWithDriver(cid, mockDriver)
		exists, err := stemcell.Exists()
		require.NoError(t, err)
		assert.False(t, exists)
		mockDriver.AssertExpectations(t)
	})
	t.Run("returns error for other failures", func(t *testing.T) {
		cid := apiv1.NewStemcellCID("error-stemcell")
		mockDriver := mocks.NewMockDriver()
		testErr := errors.New("permission denied")
		mockDriver.On("Execute", []string{"dominfo", "error-stemcell"}).
			Return("error: permission denied", testErr)
		mockDriver.On("IsMissingVMErr", "error: permission denied").
			Return(false)
		stemcell := createTestStemcellWithDriver(cid, mockDriver)
		_, err := stemcell.Exists()
		assert.Error(t, err)
		mockDriver.AssertExpectations(t)
	})
}
func TestStemcellImpl_Prepare(t *testing.T) {
	t.Run("creates snapshot successfully", func(t *testing.T) {
		cid := apiv1.NewStemcellCID("test-stemcell")
		mockDriver := mocks.NewMockDriver()
		mockDriver.On("Execute", []string{"snapshot-create-as", "test-stemcell", "snapshot-123", "--description", "BOSH stemcell snapshot"}).
			Return("Domain snapshot snapshot-123 created", nil)
		stemcell := createTestStemcellWithDriver(cid, mockDriver)
		snapshotCID, err := stemcell.Prepare("snapshot-123")
		require.NoError(t, err)
		assert.NotNil(t, snapshotCID)
		assert.Equal(t, "snapshot-123", snapshotCID.AsString())
		mockDriver.AssertExpectations(t)
	})
	t.Run("handles snapshot creation error", func(t *testing.T) {
		cid := apiv1.NewStemcellCID("test-stemcell")
		mockDriver := mocks.NewMockDriver()
		testErr := errors.New("snapshot failed")
		mockDriver.On("Execute", []string{"snapshot-create-as", "test-stemcell", "snapshot-fail", "--description", "BOSH stemcell snapshot"}).
			Return("", testErr)
		stemcell := createTestStemcellWithDriver(cid, mockDriver)
		_, err := stemcell.Prepare("snapshot-fail")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Creating snapshot")
		mockDriver.AssertExpectations(t)
	})
}
func TestStemcellImpl_Delete(t *testing.T) {
	t.Run("deletes stemcell successfully", func(t *testing.T) {
		cid := apiv1.NewStemcellCID("delete-stemcell")
		mockDriver := mocks.NewMockDriver()
		mockRunner := mocks.NewMockRunner()
		mockDriver.On("Execute", []string{"undefine", "delete-stemcell", "--remove-all-storage", "--snapshots-metadata"}).
			Return("Domain delete-stemcell has been undefined", nil)
		mockRunner.On("Execute", "rm", []string{"-rf", "/tmp/stemcells/delete-stemcell"}).
			Return("", 0, nil)
		stemcell := createTestStemcellFull(cid, mockDriver, mockRunner, "/tmp/stemcells/delete-stemcell")
		err := stemcell.Delete()
		require.NoError(t, err)
		mockDriver.AssertExpectations(t)
		mockRunner.AssertExpectations(t)
	})
	t.Run("handles missing domain gracefully", func(t *testing.T) {
		cid := apiv1.NewStemcellCID("missing-stemcell")
		mockDriver := mocks.NewMockDriver()
		mockRunner := mocks.NewMockRunner()
		testErr := errors.New("domain not found")
		mockDriver.On("Execute", []string{"undefine", "missing-stemcell", "--remove-all-storage", "--snapshots-metadata"}).
			Return("error: Domain not found", testErr)
		mockDriver.On("IsMissingVMErr", "error: Domain not found").
			Return(true)
		mockRunner.On("Execute", "rm", []string{"-rf", "/tmp/stemcells/missing-stemcell"}).
			Return("", 0, nil)
		stemcell := createTestStemcellFull(cid, mockDriver, mockRunner, "/tmp/stemcells/missing-stemcell")
		err := stemcell.Delete()
		require.NoError(t, err)
		mockDriver.AssertExpectations(t)
		mockRunner.AssertExpectations(t)
	})
}

// Helper functions
func createTestStemcell(cid apiv1.StemcellCID) StemcellImpl {
	mockDriver := mocks.NewMockDriver()
	mockRunner := mocks.NewMockRunner()
	logger := boshlog.NewLogger(boshlog.LevelNone)
	return NewStemcellImpl(cid, "/tmp/test", mockDriver, mockRunner, logger)
}
func createTestStemcellWithDriver(cid apiv1.StemcellCID, driver *mocks.MockDriver) StemcellImpl {
	mockRunner := mocks.NewMockRunner()
	logger := boshlog.NewLogger(boshlog.LevelNone)
	return NewStemcellImpl(cid, "/tmp/stemcells/"+cid.AsString(), driver, mockRunner, logger)
}
func createTestStemcellFull(cid apiv1.StemcellCID, driver *mocks.MockDriver, runner *mocks.MockRunner, path string) StemcellImpl {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	return NewStemcellImpl(cid, path, driver, runner, logger)
}
