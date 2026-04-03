package provider_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"bosh-libvirt-cpi/provider"
)

func TestProviderOptions(t *testing.T) {
	t.Run("stores binary path", func(t *testing.T) {
		opts := provider.ProviderOptions{
			BinPath: "virsh",
		}
		assert.Equal(t, "virsh", opts.BinPath)
	})

	t.Run("stores hypervisor type", func(t *testing.T) {
		opts := provider.ProviderOptions{
			Hypervisor: provider.HypervisorType("qemu"),
		}
		assert.NotEmpty(t, opts.Hypervisor)
	})
}

func TestConnectionURIGeneration(t *testing.T) {
	t.Run("generates URI", func(t *testing.T) {
		opts := provider.ProviderOptions{
			Hypervisor: provider.HypervisorType("qemu"),
		}
		uri := opts.GetConnectionURI()
		assert.NotEmpty(t, uri)
	})
}



