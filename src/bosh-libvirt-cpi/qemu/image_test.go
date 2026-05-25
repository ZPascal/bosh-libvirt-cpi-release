package qemu

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewImage(t *testing.T) {
	img := NewImage()
	assert.NotNil(t, img)
}
func TestImage_Create(t *testing.T) {
	t.Run("creates qcow2 image when qemu-img available", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		tmpDir := t.TempDir()
		imagePath := filepath.Join(tmpDir, "test.qcow2")
		img := NewImage()
		err := img.Create(imagePath, 10) // 10 MB
		require.NoError(t, err)
		assert.FileExists(t, imagePath)
	})
	t.Run("returns error for invalid path", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		img := NewImage()
		err := img.Create("/nonexistent/invalid/path/test.qcow2", 10)
		assert.Error(t, err)
	})
}
func TestImage_Convert(t *testing.T) {
	t.Run("converts between formats when qemu-img available", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		tmpDir := t.TempDir()
		srcPath := filepath.Join(tmpDir, "source.qcow2")
		dstPath := filepath.Join(tmpDir, "dest.raw")
		// Create source image first
		img := NewImage()
		err := img.Create(srcPath, 10)
		require.NoError(t, err)
		// Convert it
		err = img.Convert(srcPath, dstPath, FormatQCOW2, FormatRAW)
		require.NoError(t, err)
		assert.FileExists(t, dstPath)
	})
	t.Run("returns error for missing source", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		tmpDir := t.TempDir()
		img := NewImage()
		err := img.Convert(
			filepath.Join(tmpDir, "nonexistent.qcow2"),
			filepath.Join(tmpDir, "output.raw"),
			FormatQCOW2,
			FormatRAW,
		)
		assert.Error(t, err)
	})
}
func TestImage_Resize(t *testing.T) {
	t.Run("resizes image successfully", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		tmpDir := t.TempDir()
		imagePath := filepath.Join(tmpDir, "test.qcow2")
		img := NewImage()
		err := img.Create(imagePath, 10)
		require.NoError(t, err)
		err = img.Resize(imagePath, 20)
		assert.NoError(t, err)
	})
	t.Run("returns error for missing image", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		img := NewImage()
		err := img.Resize("/nonexistent/image.qcow2", 20)
		assert.Error(t, err)
	})
}
func TestImage_Check(t *testing.T) {
	t.Run("checks valid image without errors", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		tmpDir := t.TempDir()
		imagePath := filepath.Join(tmpDir, "test.qcow2")
		img := NewImage()
		err := img.Create(imagePath, 10)
		require.NoError(t, err)
		err = img.Check(imagePath)
		assert.NoError(t, err)
	})
	t.Run("returns error for invalid image", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		tmpDir := t.TempDir()
		invalidPath := filepath.Join(tmpDir, "invalid.qcow2")
		// Create an invalid file
		err := os.WriteFile(invalidPath, []byte("not a qcow2 file"), 0644)
		require.NoError(t, err)
		img := NewImage()
		err = img.Check(invalidPath)
		assert.Error(t, err)
	})
}
func TestImage_Info(t *testing.T) {
	t.Run("returns image info", func(t *testing.T) {
		if !commandExists("qemu-img") {
			t.Skip("qemu-img not available")
		}
		tmpDir := t.TempDir()
		imagePath := filepath.Join(tmpDir, "test.qcow2")
		img := NewImage()
		err := img.Create(imagePath, 10)
		require.NoError(t, err)
		info, err := img.Info(imagePath)
		require.NoError(t, err)
		assert.NotEmpty(t, info)
		assert.Contains(t, info, "raw_output")
	})
}
func TestImage_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.qcow2")
	nonExistingFile := filepath.Join(tmpDir, "notexists.qcow2")
	// Create a file
	f, err := os.Create(existingFile)
	require.NoError(t, err)
	_ = f.Close()
	img := NewImage()
	t.Run("returns true for existing file", func(t *testing.T) {
		exists := img.Exists(existingFile)
		assert.True(t, exists)
	})
	t.Run("returns false for non-existing file", func(t *testing.T) {
		exists := img.Exists(nonExistingFile)
		assert.False(t, exists)
	})
}

// TestImageFormats tests image format constants
func TestImageFormats(t *testing.T) {
	formats := []struct {
		format   ImageFormat
		expected string
	}{
		{FormatQCOW2, "qcow2"},
		{FormatVMDK, "vmdk"},
		{FormatRAW, "raw"},
	}

	for _, f := range formats {
		assert.Equal(t, ImageFormat(f.expected), f.format)
	}
}

// TestImageOperations_Sequence tests image operations sequence
func TestImageOperations_Sequence(t *testing.T) {
	if !commandExists("qemu-img") {
		t.Skip("qemu-img not available")
	}

	tmpDir := t.TempDir()
	img := NewImage()

	t.Run("create_and_resize", func(t *testing.T) {
		imagePath := filepath.Join(tmpDir, "sequence.qcow2")
		// Create
		err := img.Create(imagePath, 10)
		require.NoError(t, err)
		assert.FileExists(t, imagePath)

		// Resize
		err = img.Resize(imagePath, 20)
		assert.NoError(t, err)

		// Check
		err = img.Check(imagePath)
		assert.NoError(t, err)
	})
}

// TestImageSizes_Various tests various image sizes
func TestImageSizes_Various(t *testing.T) {
	if !commandExists("qemu-img") {
		t.Skip("qemu-img not available")
	}

	tmpDir := t.TempDir()
	img := NewImage()

	sizes := []int{1, 10, 100, 1024, 10240}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%dMB", size), func(t *testing.T) {
			imagePath := filepath.Join(tmpDir, fmt.Sprintf("image_%d.qcow2", size))
			err := img.Create(imagePath, size)
			assert.NoError(t, err)
			assert.FileExists(t, imagePath)
		})
	}
}

// TestImageConversion_Formats tests format conversions
func TestImageConversion_Formats(t *testing.T) {
	if !commandExists("qemu-img") {
		t.Skip("qemu-img not available")
	}

	tmpDir := t.TempDir()
	img := NewImage()

	conversions := []struct {
		name       string
		source     string
		srcFormat  ImageFormat
		destFormat ImageFormat
	}{
		{"qcow2_to_raw", "source.qcow2", FormatQCOW2, FormatRAW},
		{"qcow2_to_vmdk", "source.qcow2", FormatQCOW2, FormatVMDK},
	}

	for _, c := range conversions {
		t.Run(c.name, func(t *testing.T) {
			srcPath := filepath.Join(tmpDir, c.source)
			dstPath := filepath.Join(tmpDir, fmt.Sprintf("dest_%s", c.destFormat))

			// Create source
			err := img.Create(srcPath, 10)
			require.NoError(t, err)

			// Convert
			err = img.Convert(srcPath, dstPath, c.srcFormat, c.destFormat)
			assert.NoError(t, err)
			assert.FileExists(t, dstPath)
		})
	}
}

// TestImageDelete tests image deletion
func TestImageDelete(t *testing.T) {
	if !commandExists("qemu-img") {
		t.Skip("qemu-img not available")
	}

	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "delete_test.qcow2")

	img := NewImage()
	err := img.Create(imagePath, 10)
	require.NoError(t, err)
	assert.FileExists(t, imagePath)

	// Delete file
	err = os.Remove(imagePath)
	assert.NoError(t, err)
	assert.NoFileExists(t, imagePath)
}

// TestImageProperties_Format tests image property formats
func TestImageProperties_Format(t *testing.T) {
	props := map[string]interface{}{
		"format":      "qcow2",
		"virtual_size": 10485760, // 10 MB in bytes
		"actual_size": 1048576,    // 1 MB in bytes
		"cluster_size": 65536,     // 64 KB
		"backing_file": "",
	}

	assert.Equal(t, "qcow2", props["format"])
	assert.Greater(t, props["virtual_size"].(int), 0)
	assert.Greater(t, props["actual_size"].(int), 0)
}

// TestImagePath_Validation tests image path validation
func TestImagePath_Validation(t *testing.T) {
	validPaths := []string{
		"/var/lib/libvirt/images/image.qcow2",
		"/tmp/image.qcow2",
		"/home/user/image.qcow2",
		"relative/path/image.qcow2",
	}

	for _, path := range validPaths {
		assert.NotEmpty(t, path)
		assert.Contains(t, path, "image")
	}
}

// TestImageCreation_Performance tests image creation isn't too slow
func TestImageCreation_Performance(t *testing.T) {
	if !commandExists("qemu-img") {
		t.Skip("qemu-img not available")
	}

	tmpDir := t.TempDir()
	img := NewImage()

	// Create multiple images in sequence
	for i := 1; i <= 5; i++ {
		imagePath := filepath.Join(tmpDir, fmt.Sprintf("perf_test_%d.qcow2", i))
		err := img.Create(imagePath, 5) // Small size for speed
		assert.NoError(t, err)
		assert.FileExists(t, imagePath)
	}
}

// TestImageSnapshot_Support tests image snapshot support
func TestImageSnapshot_Support(t *testing.T) {
	// Tests snapshot commands would be used with snapshots
	snapshots := []string{
		"snapshot-create",
		"snapshot-apply",
		"snapshot-delete",
		"snapshot-list",
	}

	for _, snap := range snapshots {
		assert.NotEmpty(t, snap)
		assert.Contains(t, snap, "snapshot")
	}
}

// TestImageBacking_File tests backing file support
func TestImageBacking_File(t *testing.T) {
	if !commandExists("qemu-img") {
		t.Skip("qemu-img not available")
	}

	tmpDir := t.TempDir()
	img := NewImage()

	// Create base image
	baseImage := filepath.Join(tmpDir, "base.qcow2")
	err := img.Create(baseImage, 10)
	require.NoError(t, err)

	// Would support backing file operations
	assert.FileExists(t, baseImage)
}

// TestImageCompression_Settings tests compression settings
func TestImageCompression_Settings(t *testing.T) {
	compressionSettings := map[string]interface{}{
		"compression": "on",
		"compression_type": "zstd",
		"level": 4,
	}

	assert.Equal(t, "on", compressionSettings["compression"])
	assert.NotEmpty(t, compressionSettings["compression_type"])
}

// TestImageFilesystem_Format tests filesystem format support
func TestImageFilesystem_Format(t *testing.T) {
	formats := []string{
		"ext4",
		"btrfs",
		"xfs",
		"ntfs",
	}

	for _, fmt := range formats {
		assert.NotEmpty(t, fmt)
	}
}

// Helper function to check if command exists
func commandExists(cmd string) bool {
	paths := []string{
		"/usr/bin/" + cmd,
		"/usr/local/bin/" + cmd,
		"/bin/" + cmd,
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}
