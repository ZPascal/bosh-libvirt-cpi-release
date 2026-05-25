package disk

import (
	"errors"
	"testing"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/stretchr/testify/assert"
)

type MockDiskRunner struct {
	output string
	status int
	err    error
}

func (m *MockDiskRunner) Execute(path string, args ...string) (string, int, error) {
	return m.output, m.status, m.err
}

func (m *MockDiskRunner) Upload(src, dst string) error           { return nil }
func (m *MockDiskRunner) Put(path string, contents []byte) error { return nil }
func (m *MockDiskRunner) Get(path string) ([]byte, error)        { return nil, nil }

func TestNewDiskImpl(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	runner := &MockDiskRunner{}
	cid := apiv1.NewDiskCID("disk-123")

	disk := NewDiskImpl(cid, "/path/to/disk", runner, logger)

	assert.Equal(t, cid, disk.ID())
	assert.Equal(t, "/path/to/disk", disk.Path())
}

func TestDiskImplID(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	runner := &MockDiskRunner{}
	cid := apiv1.NewDiskCID("disk-456")

	disk := NewDiskImpl(cid, "/storage/disk", runner, logger)

	assert.Equal(t, "disk-456", disk.ID().AsString())
}

func TestDiskImplPath(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	runner := &MockDiskRunner{}
	cid := apiv1.NewDiskCID("disk-789")
	path := "/var/lib/libvirt/images/disk-789"

	disk := NewDiskImpl(cid, path, runner, logger)

	assert.Equal(t, path, disk.Path())
}

func TestDiskImplVMDKPath(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	runner := &MockDiskRunner{}
	cid := apiv1.NewDiskCID("disk-vmdk")

	disk := NewDiskImpl(cid, "/storage/disk-vmdk", runner, logger)

	vmdk := disk.VMDKPath()
	assert.Contains(t, vmdk, "disk.qcow2")
	assert.Contains(t, vmdk, "/storage/disk-vmdk")
}

func TestDiskImplDiskPath(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	runner := &MockDiskRunner{}
	cid := apiv1.NewDiskCID("disk-path")

	disk := NewDiskImpl(cid, "/storage/disk-path", runner, logger)

	assert.Equal(t, disk.VMDKPath(), disk.DiskPath())
}

func TestDiskImplExists(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	runner := &MockDiskRunner{output: "disk exists", status: 0}
	cid := apiv1.NewDiskCID("disk-exists")

	disk := NewDiskImpl(cid, "/storage/disk-exists", runner, logger)

	exists, err := disk.Exists()

	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestDiskImplExistsError(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	runner := &MockDiskRunner{err: errors.New("command failed")}
	cid := apiv1.NewDiskCID("disk-error")

	disk := NewDiskImpl(cid, "/storage/disk-error", runner, logger)

	exists, err := disk.Exists()

	assert.Error(t, err)
	assert.False(t, exists)
}
