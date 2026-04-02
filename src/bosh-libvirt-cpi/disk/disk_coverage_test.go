package disk

import (
	"testing"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Runner for testing
type mockRunner struct {
	executeOutput string
	executeErr    error
	executeCalls  int
}

func (m *mockRunner) Execute(path string, args ...string) (string, int, error) {
	m.executeCalls++
	if m.executeErr != nil {
		return "", 1, m.executeErr
	}
	return m.executeOutput, 0, nil
}

func (m *mockRunner) Upload(srcDir, dstDir string) error {
	return nil
}

func (m *mockRunner) Put(path string, contents []byte) error {
	return nil
}

func (m *mockRunner) Get(path string) ([]byte, error) {
	return nil, nil
}

func (m *mockRunner) HomeDir() (string, error) {
	return "/home/test", nil
}

func TestDiskImpl_ID(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewDiskCID("disk-123")
	mock := &mockRunner{}

	disk := NewDiskImpl(cid, "/tmp/disk", mock, logger)

	assert.Equal(t, cid, disk.ID())
}

func TestDiskImpl_Path(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewDiskCID("disk-123")
	mock := &mockRunner{}
	diskPath := "/tmp/disk"

	disk := NewDiskImpl(cid, diskPath, mock, logger)

	assert.Equal(t, diskPath, disk.Path())
}

func TestDiskImpl_VMDKPath(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewDiskCID("disk-123")
	mock := &mockRunner{}

	disk := NewDiskImpl(cid, "/tmp/disk", mock, logger)

	expected := "/tmp/disk/disk.qcow2"
	assert.Equal(t, expected, disk.VMDKPath())
}

func TestDiskImpl_DiskPath(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewDiskCID("disk-123")
	mock := &mockRunner{}

	disk := NewDiskImpl(cid, "/tmp/disk", mock, logger)

	expected := "/tmp/disk/disk.qcow2"
	assert.Equal(t, expected, disk.DiskPath())
}

func TestDiskImpl_Exists(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewDiskCID("disk-123")

	t.Run("returns true when disk exists", func(t *testing.T) {
		mock := &mockRunner{executeOutput: ""}
		disk := NewDiskImpl(cid, "/tmp/disk", mock, logger)

		exists, err := disk.Exists()

		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, 1, mock.executeCalls)
	})

	t.Run("returns false when disk does not exist", func(t *testing.T) {
		mock := &mockRunner{executeErr: assert.AnError}
		disk := NewDiskImpl(cid, "/tmp/disk", mock, logger)

		exists, err := disk.Exists()

		assert.Error(t, err)
		assert.False(t, exists)
	})
}

func TestDiskImpl_Delete(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewDiskCID("disk-123")

	t.Run("successfully deletes disk", func(t *testing.T) {
		mock := &mockRunner{executeOutput: ""}
		disk := NewDiskImpl(cid, "/tmp/disk", mock, logger)

		err := disk.Delete()

		require.NoError(t, err)
		assert.Equal(t, 1, mock.executeCalls)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		mock := &mockRunner{executeErr: assert.AnError}
		disk := NewDiskImpl(cid, "/tmp/disk", mock, logger)

		err := disk.Delete()

		assert.Error(t, err)
	})
}

func TestDiskImpl_String(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewDiskCID("disk-123")
	mock := &mockRunner{}

	disk := NewDiskImpl(cid, "/tmp/disk", mock, logger)

	// Verify disk ID is not empty
	assert.NotEmpty(t, disk.ID())
}

