package vm

import (
	"fmt"
	gonet "net"

	bnet "bosh-libvirt-cpi/vm/network"
)

// Host represents the libvirt host
// For libvirt, network management is handled differently than VirtualBox
type Host struct{}

// FindNetwork finds a network by name
// For libvirt, networks are managed via virsh and are simpler
func (h Host) FindNetwork(net Network) (bnet.Network, error) {
	// For libvirt, we use a simple network lookup
	// Networks are pre-configured in libvirt (virsh net-list)
	switch net.CloudPropertyType() {
	case bnet.NATType:
		// NAT network - use default libvirt network
		return &simpleNetwork{name: "default"}, nil

	case bnet.NATNetworkType, bnet.HostOnlyType, bnet.BridgedType:
		// Named network - use the configured name
		name := net.CloudPropertyName()
		if name == "" {
			name = "default"
		}
		return &simpleNetwork{name: name}, nil

	default:
		return nil, fmt.Errorf("unknown network type: %s", net.CloudPropertyType())
	}
}

// EnableNetworks enables the specified networks
// For libvirt, networks should be pre-configured
func (h Host) EnableNetworks(nets Networks) error {
	// For libvirt, we assume networks are already configured
	// Networks should be created via: virsh net-define/net-start
	// This is a no-op for libvirt
	return nil
}

// simpleNetwork is a simple network implementation for libvirt
type simpleNetwork struct {
	name string
}

func (n *simpleNetwork) Name() string {
	return n.name
}

func (n *simpleNetwork) Description() string {
	return fmt.Sprintf("libvirt network '%s'", n.name)
}

func (n *simpleNetwork) IsEnabled() bool {
	return true
}

func (n *simpleNetwork) EnabledDescription() string {
	return "enabled"
}

func (n *simpleNetwork) Enable() error {
	// Networks should be pre-configured in libvirt
	return nil
}

func (n *simpleNetwork) IsDHCPEnabled() bool {
	return true
}

func (n *simpleNetwork) IPNet() *gonet.IPNet {
	// Return nil - network details are managed by libvirt
	return nil
}
