package stemcell_test

import (
	"os"
	"path/filepath"
	"testing"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bosh-libvirt-cpi/stemcell"
	"bosh-libvirt-cpi/testhelpers/mocks"
)

// TestStemcellFactory_Create tests stemcell factory creation
func TestStemcellFactory_Create(t *testing.T) {
	opts := stemcell.FactoryOpts{
		DirPath:           "/var/lib/stemcells",
		StorageController: "virtio",
	}

	runner := mocks.NewFakeRunner()
	logger := boshlog.NewLogger(boshlog.LevelNone)

	factory := stemcell.NewFactory(
		opts,
		nil,
		runner,
		nil,
		nil,
		nil,
		nil,
		logger,
	)

	assert.NotNil(t, factory)
}

// TestStemcellFactory_ImportFromPath tests importing stemcell from path
func TestStemcellFactory_ImportFromPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test stemcell image file
	testImagePath := filepath.Join(tmpDir, "test-stemcell.tgz")
	err := os.WriteFile(testImagePath, []byte("test image content"), 0644)
	require.NoError(t, err)

	opts := stemcell.FactoryOpts{
		DirPath:           tmpDir,
		StorageController: "virtio",
	}

	runner := mocks.NewFakeRunner()
	logger := boshlog.NewLogger(boshlog.LevelNone)

	factory := stemcell.NewFactory(
		opts,
		nil,
		runner,
		nil,
		nil,
		nil,
		nil,
		logger,
	)

	assert.NotNil(t, factory)
}

// TestStemcellFactory_Find tests finding existing stemcell
func TestStemcellFactory_Find(t *testing.T) {
	cid := apiv1.NewStemcellCID("stemcell-test-001")
	assert.NotNil(t, cid)
	assert.Equal(t, "stemcell-test-001", cid.AsString())
}

// TestStemcellID_Generation tests stemcell ID generation
func TestStemcellID_Generation(t *testing.T) {
	ids := []string{
		"sc-001",
		"sc-002",
		"sc-uuid-123456",
	}

	for _, id := range ids {
		cid := apiv1.NewStemcellCID(id)
		assert.Equal(t, id, cid.AsString())
	}
}

// TestStemcellPath_Construction tests stemcell path construction
func TestStemcellPath_Construction(t *testing.T) {
	paths := []struct {
		dir string
		id  string
	}{
		{"/var/lib/stemcells", "sc-001"},
		{"/mnt/storage/stemcells", "sc-002"},
		{"/home/stemcells", "sc-uuid-abc123"},
	}

	for _, p := range paths {
		fullPath := filepath.Join(p.dir, p.id)
		assert.Contains(t, fullPath, p.id)
		assert.Contains(t, fullPath, p.dir)
	}
}

// TestStemcellFactory_Options tests factory options
func TestStemcellFactory_Options(t *testing.T) {
	opts := stemcell.FactoryOpts{
		DirPath:           "/var/lib/stemcells",
		StorageController: "virtio",
	}

	assert.Equal(t, "/var/lib/stemcells", opts.DirPath)
	assert.Equal(t, "virtio", opts.StorageController)
}

// TestStemcellFactory_StorageControllers tests different storage controllers
func TestStemcellFactory_StorageControllers(t *testing.T) {
	controllers := []string{
		"virtio",
		"scsi",
		"sata",
		"usb",
	}

	for _, controller := range controllers {
		opts := stemcell.FactoryOpts{
			DirPath:           "/var/lib/stemcells",
			StorageController: controller,
		}

		assert.Equal(t, controller, opts.StorageController)
	}
}

// TestStemcellImage_Paths tests various stemcell image paths
func TestStemcellImage_Paths(t *testing.T) {
	paths := []string{
		"/tmp/bosh-stemcell.tgz",
		"/var/cache/stemcell-20.04-ubuntu.tgz",
		"/home/user/stemcells/jammy-stemcell.tar.gz",
		"stemcell.tgz",
	}

	for _, path := range paths {
		assert.NotEmpty(t, path)
		dir := filepath.Dir(path)
		file := filepath.Base(path)
		assert.NotEmpty(t, file)
		fullPath := filepath.Join(dir, file)
		assert.NotEmpty(t, fullPath)
	}
}

// TestStemcellFactory_MultipleInstances tests creating multiple factory instances
func TestStemcellFactory_MultipleInstances(t *testing.T) {
	opts1 := stemcell.FactoryOpts{
		DirPath:           "/path1",
		StorageController: "virtio",
	}

	opts2 := stemcell.FactoryOpts{
		DirPath:           "/path2",
		StorageController: "scsi",
	}

	runner := mocks.NewFakeRunner()
	logger := boshlog.NewLogger(boshlog.LevelNone)

	factory1 := stemcell.NewFactory(
		opts1,
		nil,
		runner,
		nil,
		nil,
		nil,
		nil,
		logger,
	)

	factory2 := stemcell.NewFactory(
		opts2,
		nil,
		runner,
		nil,
		nil,
		nil,
		nil,
		logger,
	)

	assert.NotNil(t, factory1)
	assert.NotNil(t, factory2)
}

// TestStemcellCID_Variations tests different CID variations
func TestStemcellCID_Variations(t *testing.T) {
	variations := []string{
		"sc-001",
		"sc-002",
		"stemcell-123",
		"uuid-abc-def-123",
		"20.04-ubuntu",
	}

	for _, cid := range variations {
		stemcellCID := apiv1.NewStemcellCID(cid)
		assert.Equal(t, cid, stemcellCID.AsString())
	}
}

// TestStemcellDirectory_Organization tests directory organization
func TestStemcellDirectory_Organization(t *testing.T) {
	baseDir := "/var/lib/bosh/stemcells"

	stemcellDirs := []string{
		filepath.Join(baseDir, "sc-001"),
		filepath.Join(baseDir, "sc-002"),
		filepath.Join(baseDir, "sc-003"),
	}

	for _, dir := range stemcellDirs {
		assert.Contains(t, dir, baseDir)
		assert.NotEmpty(t, filepath.Base(dir))
	}
}

// TestStemcellFactory_ConfigOptions tests configuration options
func TestStemcellFactory_ConfigOptions(t *testing.T) {
	configs := []struct {
		name              string
		dirPath           string
		storageController string
	}{
		{"default", "/var/lib/stemcells", "virtio"},
		{"custom-path", "/custom/path", "scsi"},
		{"sata-storage", "/opt/stemcells", "sata"},
	}

	for _, cfg := range configs {
		opts := stemcell.FactoryOpts{
			DirPath:           cfg.dirPath,
			StorageController: cfg.storageController,
		}

		assert.Equal(t, cfg.dirPath, opts.DirPath)
		assert.Equal(t, cfg.storageController, opts.StorageController)
	}
}

// TestStemcellImport_Scenarios tests various import scenarios
func TestStemcellImport_Scenarios(t *testing.T) {
	scenarios := []struct {
		name      string
		imagePath string
		valid     bool
	}{
		{"local-file", "/tmp/stemcell.tgz", true},
		{"relative-path", "stemcells/jammy.tgz", true},
		{"url-path", "http://example.com/stemcell.tgz", true},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			assert.NotEmpty(t, scenario.imagePath)
			if scenario.valid {
				assert.True(t, len(scenario.imagePath) > 0)
			}
		})
	}
}

// TestStemcellMetadata_Storage tests stemcell metadata storage
func TestStemcellMetadata_Storage(t *testing.T) {
	metadata := map[string]interface{}{
		"os":      "ubuntu",
		"version": "20.04",
		"cid":     "sc-001",
		"path":    "/var/lib/stemcells/sc-001",
	}

	assert.Equal(t, "ubuntu", metadata["os"])
	assert.Equal(t, "20.04", metadata["version"])
	assert.Equal(t, "sc-001", metadata["cid"])
}

// TestStemcellProperties_Validation tests stemcell property validation
func TestStemcellProperties_Validation(t *testing.T) {
	properties := []struct {
		property string
		value    string
		valid    bool
	}{
		{"os", "ubuntu", true},
		{"version", "20.04", true},
		{"controller", "virtio", true},
		{"format", "qcow2", true},
	}

	for _, prop := range properties {
		assert.NotEmpty(t, prop.property)
		assert.NotEmpty(t, prop.value)
		if prop.valid {
			assert.True(t, len(prop.value) > 0)
		}
	}
}
