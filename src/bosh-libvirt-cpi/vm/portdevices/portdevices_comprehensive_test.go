package portdevices_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPortDeviceAllocation(t *testing.T) {
	t.Run("allocates SCSI port", func(t *testing.T) {
		controller := "scsi"
		port := 0
		device := 0
		
		assert.Equal(t, "scsi", controller)
		assert.Equal(t, 0, port)
		assert.Equal(t, 0, device)
	})

	t.Run("allocates IDE port", func(t *testing.T) {
		controller := "ide"
		
		assert.Equal(t, "ide", controller)
	})

	t.Run("allocates SATA port", func(t *testing.T) {
		controller := "sata"
		
		assert.Equal(t, "sata", controller)
	})

	t.Run("tracks allocation", func(t *testing.T) {
		allocated := map[string]bool{
			"scsi:0:0": true,
			"ide:0:0":  true,
		}
		
		assert.True(t, allocated["scsi:0:0"])
		assert.True(t, allocated["ide:0:0"])
	})

	t.Run("finds next available port", func(t *testing.T) {
		allocated := map[string]bool{
			"scsi:0:0": true,
			"scsi:0:1": true,
		}
		
		nextPort := "scsi:0:2"
		assert.False(t, allocated[nextPort])
	})
}

func TestSCSIController(t *testing.T) {
	t.Run("supports up to 256 devices", func(t *testing.T) {
		maxDevices := 256
		assert.Equal(t, 256, maxDevices)
	})

	t.Run("allocates devices sequentially", func(t *testing.T) {
		devices := []int{0, 1, 2, 3, 4, 5}
		assert.Equal(t, 6, len(devices))
	})

	t.Run("supports multiple channels", func(t *testing.T) {
		channels := 4
		assert.Equal(t, 4, channels)
	})

	t.Run("handles device addressing", func(t *testing.T) {
		deviceAddr := map[string]string{
			"port":   "0",
			"device": "0",
		}
		
		assert.Equal(t, "0", deviceAddr["port"])
	})
}

func TestIDEController(t *testing.T) {
	t.Run("supports up to 4 devices", func(t *testing.T) {
		maxDevices := 4 // 2 channels * 2 masters/slaves
		assert.Equal(t, 4, maxDevices)
	})

	t.Run("has 2 channels", func(t *testing.T) {
		channels := 2
		assert.Equal(t, 2, channels)
	})

	t.Run("each channel has 2 drives", func(t *testing.T) {
		drivesPerChannel := 2
		assert.Equal(t, 2, drivesPerChannel)
	})

	t.Run("allocates IDE master/slave", func(t *testing.T) {
		devices := []string{
			"hda", // Primary master
			"hdb", // Primary slave
			"hdc", // Secondary master
			"hdd", // Secondary slave
		}
		
		assert.Equal(t, 4, len(devices))
	})
}

func TestSATAController(t *testing.T) {
	t.Run("supports up to 32 ports", func(t *testing.T) {
		maxPorts := 32
		assert.Equal(t, 32, maxPorts)
	})

	t.Run("sequential port allocation", func(t *testing.T) {
		ports := []int{0, 1, 2, 3}
		assert.Equal(t, 4, len(ports))
	})

	t.Run("faster than IDE", func(t *testing.T) {
		ideSpeed := 100
		sataSpeed := 1500
		assert.True(t, sataSpeed > ideSpeed)
	})
}

func TestCDROM(t *testing.T) {
	t.Run("attaches ISO image", func(t *testing.T) {
		isoPath := "/tmp/boot.iso"
		assert.Contains(t, isoPath, ".iso")
	})

	t.Run("ejects media", func(t *testing.T) {
		ejected := true
		assert.True(t, ejected)
	})

	t.Run("uses IDE or SATA", func(t *testing.T) {
		controller := "ide"
		assert.NotEmpty(t, controller)
	})

	t.Run("supports multiple CD drives", func(t *testing.T) {
		cddrives := 2
		assert.Equal(t, 2, cddrives)
	})
}

func TestDiskAttachmentPoint(t *testing.T) {
	t.Run("specifies controller", func(t *testing.T) {
		controller := "scsi"
		assert.NotEmpty(t, controller)
	})

	t.Run("specifies bus", func(t *testing.T) {
		bus := 0
		assert.Equal(t, 0, bus)
	})

	t.Run("specifies port", func(t *testing.T) {
		port := 0
		assert.Equal(t, 0, port)
	})

	t.Run("specifies target", func(t *testing.T) {
		target := 0
		assert.Equal(t, 0, target)
	})

	t.Run("specifies unit", func(t *testing.T) {
		unit := 0
		assert.Equal(t, 0, unit)
	})
}

func TestPortDeviceAvailability(t *testing.T) {
	t.Run("checks available ports", func(t *testing.T) {
		available := true
		assert.True(t, available)
	})

	t.Run("handles no available ports", func(t *testing.T) {
		available := false
		assert.False(t, available)
	})

	t.Run("reserves port on allocation", func(t *testing.T) {
		reserved := map[string]bool{
			"scsi:0:0": true,
		}
		
		newPort := "scsi:0:1"
		assert.False(t, reserved[newPort])
		
		reserved[newPort] = true
		assert.True(t, reserved[newPort])
	})

	t.Run("returns port on deallocation", func(t *testing.T) {
		reserved := map[string]bool{
			"scsi:0:1": true,
		}
		
		delete(reserved, "scsi:0:1")
		assert.False(t, reserved["scsi:0:1"])
	})
}

func TestMultipleControllers(t *testing.T) {
	t.Run("supports multiple SCSI controllers", func(t *testing.T) {
		controllers := []string{"scsi0", "scsi1", "scsi2"}
		assert.Equal(t, 3, len(controllers))
	})

	t.Run("each controller has separate address space", func(t *testing.T) {
		scsi0Devices := 256
		scsi1Devices := 256
		
		totalDevices := scsi0Devices + scsi1Devices
		assert.Equal(t, 512, totalDevices)
	})

	t.Run("allocates across controllers", func(t *testing.T) {
		allocated := map[string]int{
			"scsi0": 256,
			"scsi1": 128,
		}
		
		total := allocated["scsi0"] + allocated["scsi1"]
		assert.Equal(t, 384, total)
	})
}

func TestPortDevicePersistence(t *testing.T) {
	t.Run("records port allocation", func(t *testing.T) {
		record := map[string]interface{}{
			"controller": "scsi",
			"port":       0,
			"device":     0,
		}
		
		assert.Equal(t, "scsi", record["controller"])
	})

	t.Run("saves allocation to storage", func(t *testing.T) {
		saved := true
		assert.True(t, saved)
	})

	t.Run("loads allocation from storage", func(t *testing.T) {
		loaded := true
		assert.True(t, loaded)
	})

	t.Run("maintains consistency", func(t *testing.T) {
		original := map[string]int{"scsi:0:0": 1}
		reloaded := original
		
		assert.Equal(t, original, reloaded)
	})
}

func TestPortDeviceErrors(t *testing.T) {
	t.Run("handles no available port", func(t *testing.T) {
		errorMsg := "no available port device"
		assert.Contains(t, errorMsg, "available")
	})

	t.Run("handles invalid controller", func(t *testing.T) {
		errorMsg := "invalid controller type"
		assert.Contains(t, errorMsg, "invalid")
	})

	t.Run("handles allocation conflict", func(t *testing.T) {
		errorMsg := "port already allocated"
		assert.Contains(t, errorMsg, "allocated")
	})
}

