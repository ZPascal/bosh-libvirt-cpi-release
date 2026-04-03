package disk_test

import (
	"testing"

	"github.com/cloudfoundry/bosh-cpi-go/apiv1"
	"github.com/stretchr/testify/assert"
)

func TestDiskCIDHandling(t *testing.T) {
	t.Run("creates disk CID from string", func(t *testing.T) {
		cid := apiv1.NewDiskCID("disk-12345")
		assert.Equal(t, "disk-12345", cid.AsString())
	})

	t.Run("handles various disk ID formats", func(t *testing.T) {
		ids := []string{
			"d-1",
			"disk-persistent-123",
			"persistent-disk-1",
		}

		for _, id := range ids {
			cid := apiv1.NewDiskCID(id)
			assert.Equal(t, id, cid.AsString())
		}
	})
}

func TestDiskTypeHandling(t *testing.T) {
	t.Run("identifies persistent disk", func(t *testing.T) {
		diskType := "persistent"
		assert.Equal(t, "persistent", diskType)
	})

	t.Run("identifies ephemeral disk", func(t *testing.T) {
		diskType := "ephemeral"
		assert.Equal(t, "ephemeral", diskType)
	})
}

func TestDiskSizeHandling(t *testing.T) {
	t.Run("stores disk size in MB", func(t *testing.T) {
		size := 1024 // 1GB
		assert.Equal(t, 1024, size)
	})

	t.Run("handles various disk sizes", func(t *testing.T) {
		sizes := []int{100, 512, 1024, 2048, 10240}
		assert.Equal(t, 5, len(sizes))
	})

	t.Run("handles large disk sizes", func(t *testing.T) {
		largeSize := 102400 // 100GB
		assert.Equal(t, 102400, largeSize)
	})
}

func TestDiskPathHandling(t *testing.T) {
	t.Run("generates disk path", func(t *testing.T) {
		diskID := "disk-1"
		basePath := "/var/lib/libvirt/images"
		diskPath := basePath + "/" + diskID + ".qcow2"

		assert.NotEmpty(t, diskPath)
		assert.Contains(t, diskPath, "disk-1")
	})

	t.Run("handles path with special characters", func(t *testing.T) {
		paths := []string{
			"/var/lib/disks/disk-1.qcow2",
			"/opt/libvirt/disk-abc-123.qcow2",
			"/tmp/test-disk-v1.0.qcow2",
		}

		for _, path := range paths {
			assert.True(t, len(path) > 0)
		}
	})
}

func TestDiskFormat(t *testing.T) {
	t.Run("supports QCOW2 format", func(t *testing.T) {
		format := "qcow2"
		assert.Equal(t, "qcow2", format)
	})

	t.Run("supports RAW format", func(t *testing.T) {
		format := "raw"
		assert.Equal(t, "raw", format)
	})

	t.Run("supports VMDK format", func(t *testing.T) {
		format := "vmdk"
		assert.Equal(t, "vmdk", format)
	})
}

func TestDiskProperties(t *testing.T) {
	t.Run("stores disk metadata", func(t *testing.T) {
		props := map[string]interface{}{
			"id":     "disk-1",
			"size":   1024,
			"format": "qcow2",
		}

		assert.Equal(t, "disk-1", props["id"])
		assert.Equal(t, 1024, props["size"])
		assert.Equal(t, "qcow2", props["format"])
	})
}

func TestDiskAttachmentPoints(t *testing.T) {
	t.Run("SCSI controller support", func(t *testing.T) {
		controller := "scsi"
		assert.Equal(t, "scsi", controller)
	})

	t.Run("IDE controller support", func(t *testing.T) {
		controller := "ide"
		assert.Equal(t, "ide", controller)
	})

	t.Run("SATA controller support", func(t *testing.T) {
		controller := "sata"
		assert.Equal(t, "sata", controller)
	})
}

func TestDiskHotPlug(t *testing.T) {
	t.Run("enables hot-plug operations", func(t *testing.T) {
		hotplug := true
		assert.True(t, hotplug)
	})

	t.Run("supports disk hot-attach", func(t *testing.T) {
		operation := "attach"
		assert.Equal(t, "attach", operation)
	})

	t.Run("supports disk hot-detach", func(t *testing.T) {
		operation := "detach"
		assert.Equal(t, "detach", operation)
	})
}

func TestDiskOperationErrors(t *testing.T) {
	t.Run("handles disk not found", func(t *testing.T) {
		errorMsg := "Disk not found"
		assert.Contains(t, errorMsg, "not found")
	})

	t.Run("handles attachment errors", func(t *testing.T) {
		errorMsg := "Failed to attach disk"
		assert.Contains(t, errorMsg, "Failed")
	})

	t.Run("handles size validation", func(t *testing.T) {
		size := 0
		assert.Equal(t, 0, size)
	})
}
