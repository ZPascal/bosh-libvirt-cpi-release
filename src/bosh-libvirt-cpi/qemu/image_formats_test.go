package qemu_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"bosh-libvirt-cpi/qemu"
)

func TestImageFormat_String(t *testing.T) {
	t.Run("returns QCOW2", func(t *testing.T) {
		format := qemu.FormatQCOW2
		assert.Equal(t, "qcow2", string(format))
	})

	t.Run("returns VMDK", func(t *testing.T) {
		format := qemu.FormatVMDK
		assert.Equal(t, "vmdk", string(format))
	})

	t.Run("returns RAW", func(t *testing.T) {
		format := qemu.FormatRAW
		assert.Equal(t, "raw", string(format))
	})
}

func TestImageFormats(t *testing.T) {
	formats := []qemu.ImageFormat{
		qemu.FormatQCOW2,
		qemu.FormatVMDK,
		qemu.FormatRAW,
	}

	assert.Equal(t, 3, len(formats))
	assert.Equal(t, qemu.FormatQCOW2, formats[0])
	assert.Equal(t, qemu.FormatVMDK, formats[1])
	assert.Equal(t, qemu.FormatRAW, formats[2])
}

func TestNewImage(t *testing.T) {
	img := qemu.NewImage()
	assert.NotNil(t, img)
}

func TestImage_FileHandling(t *testing.T) {
	img := qemu.NewImage()

	t.Run("Exists returns false for non-existent file", func(t *testing.T) {
		exists := img.Exists("/non/existent/path/image.qcow2")
		assert.False(t, exists)
	})

	t.Run("Exists handles empty paths", func(t *testing.T) {
		exists := img.Exists("")
		assert.False(t, exists)
	})
}

func TestImagePathHandling(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"simple path", "/tmp/image.qcow2"},
		{"nested path", "/var/lib/libvirt/images/disk.qcow2"},
		{"relative path", "./disk.qcow2"},
		{"path with spaces", "/tmp/my image.qcow2"},
		{"path with special chars", "/tmp/image-v1.0.qcow2"},
	}

	img := qemu.NewImage()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exists := img.Exists(tc.path)
			// These files don't exist, so should return false
			assert.False(t, exists)
		})
	}
}

func TestImageFormatConstants(t *testing.T) {
	t.Run("QCOW2 format is correct", func(t *testing.T) {
		assert.Equal(t, qemu.ImageFormat("qcow2"), qemu.FormatQCOW2)
	})

	t.Run("VMDK format is correct", func(t *testing.T) {
		assert.Equal(t, qemu.ImageFormat("vmdk"), qemu.FormatVMDK)
	})

	t.Run("RAW format is correct", func(t *testing.T) {
		assert.Equal(t, qemu.ImageFormat("raw"), qemu.FormatRAW)
	})
}

func TestImageFormatsNotEmpty(t *testing.T) {
	assert.NotEmpty(t, qemu.FormatQCOW2)
	assert.NotEmpty(t, qemu.FormatVMDK)
	assert.NotEmpty(t, qemu.FormatRAW)
}

func TestImageFormatsAreUnique(t *testing.T) {
	formats := map[string]bool{
		string(qemu.FormatQCOW2): false,
		string(qemu.FormatVMDK):  false,
		string(qemu.FormatRAW):   false,
	}

	count := 0
	for _, seen := range formats {
		if !seen {
			count++
		}
	}

	assert.Equal(t, 3, count)
}

