package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

func TestNewConfigFromPath(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)

	t.Run("loads valid config from file", func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "config-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "config.json")
		configData := map[string]interface{}{
			"hypervisor":            "qemu",
			"bin_path":              "virsh",
			"store_dir":             "/var/vcap/store",
			"storage_controller":    "ide",
			"auto_enable_networks":  false,
			"agent": map[string]interface{}{
				"mbus": "nats://nats:nats@localhost:4222",
			},
		}

		jsonData, err := json.Marshal(configData)
		require.NoError(t, err)

		err = ioutil.WriteFile(configPath, jsonData, 0644)
		require.NoError(t, err)

		fs := boshsys.NewOsFileSystem(logger)
		config, err := NewConfigFromPath(configPath, fs)

		require.NoError(t, err)
		assert.NotNil(t, config)
	})

	t.Run("returns error when file not found", func(t *testing.T) {
		fs := boshsys.NewOsFileSystem(logger)
		_, err := NewConfigFromPath("/nonexistent/config.json", fs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Reading config")
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "config-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "config.json")
		err = ioutil.WriteFile(configPath, []byte("{ invalid json"), 0644)
		require.NoError(t, err)

		fs := boshsys.NewOsFileSystem(logger)
		_, err = NewConfigFromPath(configPath, fs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Unmarshalling config")
	})

	t.Run("applies default hypervisor", func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "config-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "config.json")
		configData := map[string]interface{}{
			"bin_path":           "virsh",
			"store_dir":          "/var/vcap/store",
			"storage_controller": "ide",
			"agent": map[string]interface{}{
				"mbus": "nats://nats:nats@localhost:4222",
			},
		}

		jsonData, err := json.Marshal(configData)
		require.NoError(t, err)

		err = ioutil.WriteFile(configPath, jsonData, 0644)
		require.NoError(t, err)

		fs := boshsys.NewOsFileSystem(logger)
		config, err := NewConfigFromPath(configPath, fs)

		require.NoError(t, err)
		assert.Equal(t, "qemu", config.Hypervisor)
	})

	t.Run("applies default bin_path", func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "config-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "config.json")
		configData := map[string]interface{}{
			"hypervisor":         "qemu",
			"store_dir":          "/var/vcap/store",
			"storage_controller": "ide",
			"agent": map[string]interface{}{
				"mbus": "nats://nats:nats@localhost:4222",
			},
		}

		jsonData, err := json.Marshal(configData)
		require.NoError(t, err)

		err = ioutil.WriteFile(configPath, jsonData, 0644)
		require.NoError(t, err)

		fs := boshsys.NewOsFileSystem(logger)
		config, err := NewConfigFromPath(configPath, fs)

		require.NoError(t, err)
		assert.Equal(t, "virsh", config.BinPath)
	})

	t.Run("validates invalid hypervisor", func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "config-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "config.json")
		configData := map[string]interface{}{
			"hypervisor":         "invalid_hypervisor",
			"bin_path":           "virsh",
			"store_dir":          "/var/vcap/store",
			"storage_controller": "ide",
			"agent": map[string]interface{}{
				"mbus": "nats://nats:nats@localhost:4222",
			},
		}

		jsonData, err := json.Marshal(configData)
		require.NoError(t, err)

		err = ioutil.WriteFile(configPath, jsonData, 0644)
		require.NoError(t, err)

		fs := boshsys.NewOsFileSystem(logger)
		_, err = NewConfigFromPath(configPath, fs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid hypervisor")
	})
}

// Mock filesystem for testing error cases
type mockFileSystem struct {
	err error
}

func (fs *mockFileSystem) HomeDir() (string, error) {
	return "", nil
}

func (fs *mockFileSystem) TempDir(prefix string) (string, error) {
	return "", nil
}

func (fs *mockFileSystem) RemoveAll(path string) error {
	return nil
}

func (fs *mockFileSystem) MkdirAll(path string, mode os.FileMode) error {
	return nil
}

func (fs *mockFileSystem) Chown(path, user, group string) error {
	return nil
}

func (fs *mockFileSystem) Chmod(path string, mode os.FileMode) error {
	return nil
}

func (fs *mockFileSystem) OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	return nil, errors.New("mocked error")
}

func (fs *mockFileSystem) ReadFile(path string) ([]byte, error) {
	if fs.err != nil {
		return nil, fs.err
	}
	return nil, errors.New("file not found")
}

func (fs *mockFileSystem) WriteFile(path string, content []byte) error {
	return nil
}

func (fs *mockFileSystem) SymlinkBash(oldPath, newPath string) error {
	return nil
}

func (fs *mockFileSystem) Symlink(oldPath, newPath string) error {
	return nil
}

func (fs *mockFileSystem) ReadFileWithOpts(path string, opts boshsys.ReadOpts) ([]byte, error) {
	return nil, errors.New("file not found")
}

func (fs *mockFileSystem) FileExists(path string) bool {
	return false
}

func (fs *mockFileSystem) Glob(pattern string) ([]string, error) {
	return nil, nil
}

func (fs *mockFileSystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return nil
}

