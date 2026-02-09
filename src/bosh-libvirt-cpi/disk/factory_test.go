package disk

import (
	"bosh-libvirt-cpi/testhelpers/mocks"
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	fakeuuid "github.com/cloudfoundry/bosh-utils/uuid/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFactory_Create(t *testing.T) {
	t.Run("creates disk with unique ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		fs := fakesys.NewFakeFileSystem()
		uuidGen := &fakeuuid.FakeGenerator{}
		uuidGen.GeneratedUUID = "fake-uuid-123"
		mockDriver := mocks.NewMockDriver()
		mockRunner := mocks.NewMockRunner()
		// Mock mkdir success
		mockRunner.On("Execute", "mkdir", []string{"-p", tmpDir + "/disk-fake-uuid-123"}).
			Return("", 0, nil)
		factory := NewFactory(tmpDir, uuidGen, mockDriver, mockRunner, logger)
		disk, err := factory.Create(1024)
		require.NoError(t, err)
		assert.NotNil(t, disk)
		assert.Equal(t, "disk-fake-uuid-123", disk.ID().AsString())
	})
}
func TestFactory_Find(t *testing.T) {
	t.Run("finds disk by CID", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		uuidGen := &fakeuuid.FakeGenerator{}
		mockDriver := mocks.NewMockDriver()
		mockRunner := mocks.NewMockRunner()
		factory := NewFactory(tmpDir, uuidGen, mockDriver, mockRunner, logger)
		cid := apiv1.NewDiskCID("disk-123")
		disk, err := factory.Find(cid)
		require.NoError(t, err)
		assert.Equal(t, cid, disk.ID())
	})
}
