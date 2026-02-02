package qemu

import (
	"fmt"
	"os"
	"os/exec"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

// ImageFormat represents disk image format
type ImageFormat string

const (
	FormatQCOW2 ImageFormat = "qcow2"
	FormatVMDK  ImageFormat = "vmdk"
	FormatRAW   ImageFormat = "raw"
)

// Image provides qcow2 image operations
type Image struct{}

// NewImage creates a new Image instance
func NewImage() *Image {
	return &Image{}
}

// Create creates a new qcow2 image
func (i *Image) Create(path string, sizeMB int) error {
	sizeStr := fmt.Sprintf("%dM", sizeMB)

	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", path, sizeStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating qcow2 image: %s", string(output))
	}

	return nil
}

// Convert converts an image from one format to another
func (i *Image) Convert(sourcePath, destPath string, sourceFormat, destFormat ImageFormat) error {
	cmd := exec.Command("qemu-img", "convert",
		"-f", string(sourceFormat),
		"-O", string(destFormat),
		sourcePath,
		destPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return bosherr.WrapErrorf(err, "Converting image: %s", string(output))
	}

	return nil
}

// Info returns information about an image
func (i *Image) Info(path string) (map[string]string, error) {
	cmd := exec.Command("qemu-img", "info", "--output=json", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Getting image info: %s", string(output))
	}

	// For now, return basic info
	// TODO: Parse JSON output
	info := make(map[string]string)
	info["raw_output"] = string(output)

	return info, nil
}

// Resize resizes an image
func (i *Image) Resize(path string, newSizeMB int) error {
	sizeStr := fmt.Sprintf("%dM", newSizeMB)

	cmd := exec.Command("qemu-img", "resize", path, sizeStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return bosherr.WrapErrorf(err, "Resizing image: %s", string(output))
	}

	return nil
}

// Check checks an image for errors
func (i *Image) Check(path string) error {
	cmd := exec.Command("qemu-img", "check", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// qemu-img check returns non-zero for corrupted images
		return bosherr.Errorf("Image check failed: %s", string(output))
	}

	return nil
}

// Exists checks if an image file exists
func (i *Image) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
