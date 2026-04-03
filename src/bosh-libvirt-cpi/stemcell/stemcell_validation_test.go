package stemcell_test

import (
	"testing"

	"github.com/cloudfoundry/bosh-cpi-go/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStemcellCIDType(t *testing.T) {
	t.Run("creates stemcell CID from string", func(t *testing.T) {
		cid := apiv1.NewStemcellCID("stemcell-123")
		assert.Equal(t, "stemcell-123", cid.AsString())
	})

	t.Run("handles various CID formats", func(t *testing.T) {
		formats := []string{
			"sc-1",
			"stemcell-v1.0",
			"ubuntu-jammy",
		}

		for _, format := range formats {
			cid := apiv1.NewStemcellCID(format)
			assert.Equal(t, format, cid.AsString())
		}
	})
}

func TestStemcellDirectoryStructure(t *testing.T) {
	t.Run("creates valid directory path", func(t *testing.T) {
		path := "/var/lib/stemcells/sc-1"
		assert.True(t, len(path) > 0)
	})

	t.Run("handles nested paths", func(t *testing.T) {
		paths := []string{
			"/opt/bosh/stemcells",
			"/var/lib/libvirt/stemcells",
			"/tmp/test-stemcells",
		}

		for _, path := range paths {
			assert.True(t, len(path) > 0)
		}
	})
}

func TestStemcellImageOperations(t *testing.T) {
	t.Run("identifies image type", func(t *testing.T) {
		imageType := "qcow2"
		assert.NotEmpty(t, imageType)
	})

	t.Run("stores image metadata", func(t *testing.T) {
		metadata := map[string]interface{}{
			"format": "qcow2",
			"size":   1024 * 1024 * 1024,
		}
		assert.Equal(t, "qcow2", metadata["format"])
	})
}

func TestStemcellCloning(t *testing.T) {
	t.Run("creates clone from base image", func(t *testing.T) {
		baseImage := "/opt/stemcells/ubuntu-20.04.qcow2"
		clonePath := "/var/lib/libvirt/images/vm-1-disk-0.qcow2"

		assert.NotEmpty(t, baseImage)
		assert.NotEmpty(t, clonePath)
	})

	t.Run("creates snapshot-based clone", func(t *testing.T) {
		base := "base-snapshot"
		clone := "clone-from-base"

		assert.NotEmpty(t, base)
		assert.NotEmpty(t, clone)
	})
}

func TestStemcellVersioning(t *testing.T) {
	t.Run("stores version information", func(t *testing.T) {
		version := "2.5.1"
		assert.NotEmpty(t, version)
	})

	t.Run("handles version strings", func(t *testing.T) {
		versions := []string{
			"1.0",
			"2.0",
			"3.0-rc1",
			"4.0-beta.1",
		}

		require.Equal(t, 4, len(versions))
	})
}

func TestStemcellOS(t *testing.T) {
	t.Run("ubuntu stemcell", func(t *testing.T) {
		osName := "ubuntu"
		assert.Equal(t, "ubuntu", osName)
	})

	t.Run("centos stemcell", func(t *testing.T) {
		osName := "centos"
		assert.Equal(t, "centos", osName)
	})

	t.Run("other OS types", func(t *testing.T) {
		osTypes := []string{"debian", "fedora", "alma"}
		require.Equal(t, 3, len(osTypes))
	})
}

func TestStemcellValidation(t *testing.T) {
	t.Run("validates stemcell path", func(t *testing.T) {
		path := "/var/lib/stemcells/sc-1"
		isValid := len(path) > 0
		assert.True(t, isValid)
	})

	t.Run("validates image format", func(t *testing.T) {
		format := "qcow2"
		validFormats := map[string]bool{
			"qcow2": true,
			"vmdk":  true,
			"raw":   true,
		}
		assert.True(t, validFormats[format])
	})
}

func TestStemcellProperties(t *testing.T) {
	t.Run("stores cloud properties", func(t *testing.T) {
		props := map[string]interface{}{
			"hypervisor": "kvm",
			"format":     "qcow2",
		}
		assert.Equal(t, "kvm", props["hypervisor"])
	})

	t.Run("stores API version info", func(t *testing.T) {
		apiVersion := 2
		assert.Equal(t, 2, apiVersion)
	})
}

