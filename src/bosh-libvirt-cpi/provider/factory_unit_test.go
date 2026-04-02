package provider

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

func TestNewProviderFactory(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	factory := NewProviderFactory(logger)

	assert.NotNil(t, factory)
}

func TestProviderFactory_Create(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	factory := NewProviderFactory(logger)

	runner := &mockRunner{}
	retrier := &mockRetrier{}
	fs := boshsys.NewOsFileSystem(logger)
	opts := ProviderOptions{
		BinPath:    "virsh",
		StoreDir:   "/var/vcap/store",
		Hypervisor: HypervisorTypeQEMU,
	}

	provider, err := factory.Create(HypervisorTypeQEMU, runner, retrier, fs, opts)

	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestProviderFactory_CreateWithDifferentHypervisors(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	factory := NewProviderFactory(logger)

	runner := &mockRunner{}
	retrier := &mockRetrier{}
	fs := boshsys.NewOsFileSystem(logger)

	testCases := []struct {
		name       string
		hypervisor HypervisorType
	}{
		{"QEMU", HypervisorTypeQEMU},
		{"VirtualBox", HypervisorTypeVBox},
		{"LXC", HypervisorTypeLXC},
		{"Xen", HypervisorTypeXen},
		{"VMware", HypervisorTypeVMware},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := ProviderOptions{
				BinPath:    "virsh",
				StoreDir:   "/var/vcap/store",
				Hypervisor: tc.hypervisor,
			}

			provider, err := factory.Create(tc.hypervisor, runner, retrier, fs, opts)
			assert.NoError(t, err)
			assert.NotNil(t, provider)
		})
	}
}

func TestProviderOptions_GetConnectionURI(t *testing.T) {
	testCases := []struct {
		name       string
		hypervisor HypervisorType
		uri        string
		expected   string
	}{
		{"QEMU system default", HypervisorTypeQEMU, "", "qemu:///system"},
		{"VirtualBox session default", HypervisorTypeVBox, "", "vbox:///session"},
		{"LXC default", HypervisorTypeLXC, "", "lxc:///"},
		{"Xen default", HypervisorTypeXen, "", "xen:///"},
		{"VMware session default", HypervisorTypeVMware, "", "vmware:///session"},
		{"Custom URI overrides hypervisor", HypervisorTypeQEMU, "qemu+ssh://remote/system", "qemu+ssh://remote/system"},
		{"Custom URI for VBox", HypervisorTypeVBox, "vbox+ssh://remote/session", "vbox+ssh://remote/session"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := ProviderOptions{
				Hypervisor: tc.hypervisor,
				URI:        tc.uri,
			}

			uri := opts.GetConnectionURI()
			assert.Equal(t, tc.expected, uri)
		})
	}
}

func TestProviderOptions_GetConnectionURIDefaultsToQEMU(t *testing.T) {
	opts := ProviderOptions{}
	uri := opts.GetConnectionURI()
	assert.Equal(t, "qemu:///system", uri)
}

// Mock implementations for testing
type mockRetrier struct {
	retryCalled bool
}

func (r *mockRetrier) Retry(fn func() error) error {
	r.retryCalled = true
	return fn()
}

func (r *mockRetrier) RetryComplex(fn func() error, attempts int, delay time.Duration) error {
	return fn()
}

type mockRunner struct{}

func (r *mockRunner) Execute(path string, args ...string) (string, int, error) {
	return "", 0, nil
}

func (r *mockRunner) Upload(srcDir, dstDir string) error {
	return nil
}

func (r *mockRunner) Put(path string, contents []byte) error {
	return nil
}

func (r *mockRunner) Get(path string) ([]byte, error) {
	return nil, nil
}


