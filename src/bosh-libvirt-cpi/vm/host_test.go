package vm

import (
	bnet "bosh-libvirt-cpi/vm/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHost_FindNetwork(t *testing.T) {
	host := Host{}
	t.Run("returns default for NAT type", func(t *testing.T) {
		net := createTestNetwork(bnet.NATType, "")
		network, err := host.FindNetwork(net)
		require.NoError(t, err)
		assert.Equal(t, "default", network.Name())
	})
	t.Run("returns named network for NATNetworkType", func(t *testing.T) {
		net := createTestNetwork(bnet.NATNetworkType, "custom-nat")
		network, err := host.FindNetwork(net)
		require.NoError(t, err)
		assert.Equal(t, "custom-nat", network.Name())
	})
	t.Run("returns default when name is empty", func(t *testing.T) {
		net := createTestNetwork(bnet.HostOnlyType, "")
		network, err := host.FindNetwork(net)
		require.NoError(t, err)
		assert.Equal(t, "default", network.Name())
	})
	t.Run("returns error for unknown type", func(t *testing.T) {
		net := createTestNetwork("unknown", "test")
		_, err := host.FindNetwork(net)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown network type")
	})
}
func TestHost_EnableNetworks(t *testing.T) {
	host := Host{}
	nets := Networks{
		"net1": createTestNetwork(bnet.NATType, ""),
		"net2": createTestNetwork(bnet.HostOnlyType, "test"),
	}
	err := host.EnableNetworks(nets)
	// For libvirt, this should be a no-op
	assert.NoError(t, err)
}
func TestSimpleNetwork(t *testing.T) {
	t.Run("implements all Network interface methods", func(t *testing.T) {
		net := &simpleNetwork{name: "test-network"}
		assert.Equal(t, "test-network", net.Name())
		assert.Contains(t, net.Description(), "test-network")
		assert.True(t, net.IsEnabled())
		assert.Equal(t, "enabled", net.EnabledDescription())
		assert.NoError(t, net.Enable())
		assert.True(t, net.IsDHCPEnabled())
		assert.Nil(t, net.IPNet())
	})
}

// Helper to create test networks
func createTestNetwork(netType, name string) Network {
	return Network{
		props: NetworkCloudProps{
			Name: name,
			Type: netType,
		},
	}
}
