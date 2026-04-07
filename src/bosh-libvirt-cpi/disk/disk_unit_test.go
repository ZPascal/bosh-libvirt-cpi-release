package disk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	"bosh-libvirt-cpi/disk"
)

// TestDiskImpl_ID tests ID getter
func TestDiskImpl_ID(t *testing.T) {
	// Setup
	cid := apiv1.NewDiskCID("test-disk-123")
	d := disk.NewDiskImpl(cid, "/path/to/disk", nil, nil)

	// Execute
	result := d.ID()

	// Assert
	assert.Equal(t, "test-disk-123", result.AsString())
}

// TestDiskImpl_Path tests Path getter
func TestDiskImpl_Path(t *testing.T) {
	// Setup
	d := disk.NewDiskImpl(apiv1.NewDiskCID("disk-1"), "/var/lib/disks/disk-1", nil, nil)

	// Execute
	result := d.Path()

	// Assert
	assert.Equal(t, "/var/lib/disks/disk-1", result)
}

// TestDiskImpl_VMDKPath tests VMDKPath getter
func TestDiskImpl_VMDKPath(t *testing.T) {
	// Setup
	d := disk.NewDiskImpl(apiv1.NewDiskCID("disk-2"), "/var/lib/disks/disk-2", nil, nil)

	// Execute
	result := d.VMDKPath()

	// Assert
	assert.Contains(t, result, "/var/lib/disks/disk-2")
	assert.Contains(t, result, "disk.qcow2")
}

// TestDiskImpl_DiskPath tests DiskPath getter
func TestDiskImpl_DiskPath(t *testing.T) {
	// Setup
	d := disk.NewDiskImpl(apiv1.NewDiskCID("disk-3"), "/var/lib/disks/disk-3", nil, nil)

	// Execute
	result := d.DiskPath()

	// Assert
	assert.Contains(t, result, "/var/lib/disks/disk-3")
	assert.Contains(t, result, "disk.qcow2")
}

// TestDiskImpl_MultipleDiskIDs tests multiple disk instances
func TestDiskImpl_MultipleDiskIDs(t *testing.T) {
	// Setup
	diskIDs := []string{"disk-a", "disk-b", "disk-c"}
	disks := make([]disk.Disk, len(diskIDs))

	for i, id := range diskIDs {
		disks[i] = disk.NewDiskImpl(apiv1.NewDiskCID(id), "/path/"+id, nil, nil)
	}

	// Execute & Assert
	for i, d := range disks {
		assert.Equal(t, diskIDs[i], d.ID().AsString())
		assert.Equal(t, "/path/"+diskIDs[i], d.Path())
	}
}

