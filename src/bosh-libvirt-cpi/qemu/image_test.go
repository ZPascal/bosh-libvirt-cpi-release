package qemu

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
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
	f.Close()
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
