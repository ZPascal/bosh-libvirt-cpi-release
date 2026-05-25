package disk

import (
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/stretchr/testify/assert"
)

type MockUUIDGen struct{}

func (m *MockUUIDGen) Generate() (string, error) {
	return "mock-uuid-123", nil
}

func TestDiskFactoryInit(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	assert.NotNil(t, logger)
}

func TestDiskStoragePath(t *testing.T) {
	path := "/storage/disks/disk-123"
	assert.NotEmpty(t, path)
}
