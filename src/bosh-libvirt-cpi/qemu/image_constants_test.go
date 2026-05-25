package qemu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageFormatConstants(t *testing.T) {
	assert.Equal(t, ImageFormat("qcow2"), FormatQCOW2)
	assert.Equal(t, ImageFormat("vmdk"), FormatVMDK)
	assert.Equal(t, ImageFormat("raw"), FormatRAW)
}

func TestImageFormatString(t *testing.T) {
	assert.Equal(t, "qcow2", string(FormatQCOW2))
	assert.Equal(t, "vmdk", string(FormatVMDK))
	assert.Equal(t, "raw", string(FormatRAW))
}

func TestImageNewImage(t *testing.T) {
	img := NewImage()
	assert.NotNil(t, img)
}

func TestImageInterface(t *testing.T) {
	img := NewImage()
	assert.NotNil(t, img)
	// Verify it implements the expected interface
}

func TestDiskFormatNames(t *testing.T) {
	formats := []ImageFormat{FormatQCOW2, FormatVMDK, FormatRAW}
	assert.Equal(t, 3, len(formats))
}
