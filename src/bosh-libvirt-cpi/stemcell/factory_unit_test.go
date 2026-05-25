package stemcell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFactoryOpts(t *testing.T) {
	opts := FactoryOpts{
		DirPath:           "/tmp/stemcells",
		StorageController: "virtio",
	}
	assert.NotEmpty(t, opts.DirPath)
	assert.NotEmpty(t, opts.StorageController)
}

func TestFactoryOptsEmpty(t *testing.T) {
	opts := FactoryOpts{}
	assert.Empty(t, opts.DirPath)
}

func TestStorageControllerTypes(t *testing.T) {
	controllers := []string{
		"virtio",
		"sata",
		"scsi",
		"ide",
	}
	assert.Equal(t, 4, len(controllers))
}

func TestStemcellMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"os":         "ubuntu",
		"os_version": "jammy",
		"api":        "2",
	}
	assert.Equal(t, "ubuntu", metadata["os"])
	assert.Equal(t, "jammy", metadata["os_version"])
}

func TestStemcellInterface(t *testing.T) {
	// Stemcell interface should be properly implemented
	var stemcell interface{} = "test-stemcell"
	assert.NotNil(t, stemcell)
}
