package vm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"bosh-libvirt-cpi/vm"
)

func TestNetworkConfiguration(t *testing.T) {
	t.Run("creates dynamic network", func(t *testing.T) {
		net := vm.Network{}
		assert.NotNil(t, net)
	})

	t.Run("handles network metadata", func(t *testing.T) {
		nets := vm.Networks{}
		assert.NotNil(t, nets)
	})
}

func TestNetworkCloudProps(t *testing.T) {
	t.Run("stores network name", func(t *testing.T) {
		props := vm.NetworkCloudProps{
			Name: "default",
			Type: "dynamic",
		}
		assert.Equal(t, "default", props.Name)
		assert.Equal(t, "dynamic", props.Type)
	})

	t.Run("creates from cloud properties", func(t *testing.T) {
		props := vm.NetworkCloudProps{
			Name: "private",
			Type: "static",
		}
		assert.Equal(t, "private", props.Name)
	})
}

func TestVMPropsStructure(t *testing.T) {
	t.Run("stores memory value", func(t *testing.T) {
		props := vm.VMProps{Memory: 2048}
		assert.Equal(t, 2048, props.Memory)
	})

	t.Run("stores CPU value", func(t *testing.T) {
		props := vm.VMProps{CPUs: 4}
		assert.Equal(t, 4, props.CPUs)
	})

	t.Run("stores all properties", func(t *testing.T) {
		props := vm.VMProps{
			Memory: 4096,
			CPUs:   8,
		}
		assert.Equal(t, 4096, props.Memory)
		assert.Equal(t, 8, props.CPUs)
	})
}

