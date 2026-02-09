package provider

import (
	"bosh-libvirt-cpi/testhelpers/mocks"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewLibvirtProvider(t *testing.T) {
	t.Run("creates provider with QEMU hypervisor", func(t *testing.T) {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		runner := mocks.NewMockRunner()
		retrier := &mocks.MockRetrier{}
		fs := fakesys.NewFakeFileSystem()
		opts := ProviderOptions{
			BinPath:    "virsh",
			StoreDir:   "/tmp/test",
			Hypervisor: HypervisorTypeQEMU,
		}
		provider, err := NewLibvirtProvider(HypervisorTypeQEMU, runner, retrier, fs, opts, logger)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, HypervisorTypeQEMU, provider.GetHypervisor())
	})
	t.Run("uses default virsh path if not specified", func(t *testing.T) {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		runner := mocks.NewMockRunner()
		retrier := &mocks.MockRetrier{}
		fs := fakesys.NewFakeFileSystem()
		opts := ProviderOptions{
			StoreDir:   "/tmp/test",
			Hypervisor: HypervisorTypeQEMU,
		}
		provider, err := NewLibvirtProvider(HypervisorTypeQEMU, runner, retrier, fs, opts, logger)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})
}
func TestProviderOptions_GetConnectionURI(t *testing.T) {
	tests := []struct {
		name        string
		hypervisor  HypervisorType
		customURI   string
		expectedURI string
	}{
		{"QEMU auto-generated", HypervisorTypeQEMU, "", "qemu:///system"},
		{"VBox auto-generated", HypervisorTypeVBox, "", "vbox:///session"},
		{"LXC auto-generated", HypervisorTypeLXC, "", "lxc:///"},
		{"Custom URI", HypervisorTypeQEMU, "qemu+ssh://host/system", "qemu+ssh://host/system"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ProviderOptions{
				Hypervisor: tt.hypervisor,
				URI:        tt.customURI,
			}
			uri := opts.GetConnectionURI()
			assert.Equal(t, tt.expectedURI, uri)
		})
	}
}

// MockRetrier for testing
type MockRetrier struct{}

func (m *MockRetrier) Retry(fn func() error) error {
	return fn()
}
