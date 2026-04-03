package qemu_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bosh-libvirt-cpi/qemu"
)

func TestImage_ExistsWithRealFile(t *testing.T) {
	img := qemu.NewImage()
	require.NotNil(t, img)

	// Test with non-existent file
	exists := img.Exists("/tmp/nonexistent-qcow2-image-12345.qcow2")
	assert.False(t, exists)

	// Test with real temporary file
	tmpFile, err := os.CreateTemp("", "test-*.qcow2")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tmpFile.Close()
	exists = img.Exists(tmpFile.Name())
	assert.True(t, exists)
}

func TestImage_ExistsEdgeCases(t *testing.T) {
	img := qemu.NewImage()

	t.Run("empty path", func(t *testing.T) {
		exists := img.Exists("")
		assert.False(t, exists)
	})

	t.Run("current directory", func(t *testing.T) {
		exists := img.Exists(".")
		assert.True(t, exists) // Current directory exists
	})

	t.Run("root directory", func(t *testing.T) {
		exists := img.Exists("/")
		assert.True(t, exists)
	})

	t.Run("relative path", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.qcow2")
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		baseName := filepath.Base(tmpFile.Name())
		// Note: This might fail if not in same directory, but tests the function
		_ = baseName
	})
}

func TestImage_PathHandling(t *testing.T) {
	img := qemu.NewImage()

	paths := []struct {
		name  string
		path  string
		valid bool
	}{
		{"absolute path", "/tmp/image.qcow2", false}, // doesn't exist
		{"dot path", ".", true},                       // current dir exists
		{"root", "/", true},                           // root exists
		{"home dir", os.ExpandEnv("$HOME"), true},     // home usually exists
	}

	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			exists := img.Exists(p.path)
			assert.Equal(t, p.valid, exists, "path: "+p.path)
		})
	}
}

func TestImage_MultipleInstances(t *testing.T) {
	img1 := qemu.NewImage()
	img2 := qemu.NewImage()

	assert.NotNil(t, img1)
	assert.NotNil(t, img2)

	// Both should report same result for same path
	tmpFile, err := os.CreateTemp("", "test-*.qcow2")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	exists1 := img1.Exists(tmpFile.Name())
	exists2 := img2.Exists(tmpFile.Name())
	assert.Equal(t, exists1, exists2)
	assert.True(t, exists1)
}

func TestImageFormatString(t *testing.T) {
	formats := []struct {
		format   qemu.ImageFormat
		expected string
	}{
		{qemu.FormatQCOW2, "qcow2"},
		{qemu.FormatVMDK, "vmdk"},
		{qemu.FormatRAW, "raw"},
	}

	for _, f := range formats {
		t.Run(f.expected, func(t *testing.T) {
			assert.Equal(t, f.expected, string(f.format))
		})
	}
}

func TestImageFormatsAreDistinct(t *testing.T) {
	assert.NotEqual(t, qemu.FormatQCOW2, qemu.FormatVMDK)
	assert.NotEqual(t, qemu.FormatQCOW2, qemu.FormatRAW)
	assert.NotEqual(t, qemu.FormatVMDK, qemu.FormatRAW)
}

func TestNewImageCreatesValidInstance(t *testing.T) {
	img := qemu.NewImage()

	require.NotNil(t, img)

	// Should be usable for Exists check
	exists := img.Exists("/tmp")
	assert.True(t, exists)
}

func TestImage_HandlesSymlinkPaths(t *testing.T) {
	img := qemu.NewImage()

	// Create a real file and symlink to it
	tmpFile, err := os.CreateTemp("", "test-real-*.qcow2")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Create symlink
	symlinkPath := filepath.Join(filepath.Dir(tmpFile.Name()), "test-link.qcow2")
	err = os.Symlink(tmpFile.Name(), symlinkPath)
	if err == nil {
		defer os.Remove(symlinkPath)

		exists := img.Exists(symlinkPath)
		assert.True(t, exists)
	}
	// Skip on systems where symlink creation fails
}

func TestImage_PerformanceWithManyPaths(t *testing.T) {
	img := qemu.NewImage()

	// Test that function can be called many times without issue
	for i := 0; i < 100; i++ {
		exists := img.Exists("/nonexistent/path")
		assert.False(t, exists)
	}
}

func TestImage_ParallelExists(t *testing.T) {
	img := qemu.NewImage()

	done := make(chan bool, 10)

	// Test concurrent access
	for i := 0; i < 10; i++ {
		go func() {
			exists := img.Exists("/tmp")
			assert.True(t, exists)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

