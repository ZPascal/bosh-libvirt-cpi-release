package vm

import (
	"fmt"
	"math/rand"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	"bosh-libvirt-cpi/driver"
	bnet "bosh-libvirt-cpi/vm/network"
)

const (
	// Libvirt supports many NICs, but we'll keep a reasonable limit
	maxNICs = 8
)

type NICs struct {
	driver driver.Driver
	vmCID  apiv1.VMCID
}

func (n NICs) Configure(nets Networks, host Host) error {
	if len(nets) > maxNICs {
		return bosherr.Errorf("Exceeded maximum # of NICs (%d)", maxNICs)
	}

	for _, net := range nets {
		mac, err := n.addNIC(net, host)
		if err != nil {
			return err
		}

		net.SetMAC(mac)
	}

	return nil
}

func (n NICs) addNIC(net Network, host Host) (string, error) {
	// Generate MAC address
	mac, err := n.randomMAC()
	if err != nil {
		return "", err
	}
	macStr := n.userFriendly(mac)

	// Determine network type and source
	var networkType, networkSource string

	switch net.CloudPropertyType() {
	case bnet.NATType:
		// Use default NAT network
		networkType = "network"
		networkSource = "default"

	case bnet.NATNetworkType:
		actualNet, err := host.FindNetwork(net)
		if err != nil {
			return "", err
		}
		networkType = "network"
		networkSource = actualNet.Name()

	case bnet.HostOnlyType:
		actualNet, err := host.FindNetwork(net)
		if err != nil {
			return "", err
		}
		networkType = "network"
		networkSource = actualNet.Name()

	case bnet.BridgedType:
		actualNet, err := host.FindNetwork(net)
		if err != nil {
			return "", err
		}
		networkType = "bridge"
		networkSource = actualNet.Name()

	default:
		return "", bosherr.Errorf("Unknown network type: %s", net.CloudPropertyType())
	}

	// Attach network interface using virsh
	args := []string{
		"attach-interface", n.vmCID.AsString(),
		networkType,
		"--source", networkSource,
		"--mac", macStr,
		"--model", "virtio", // Use virtio for better performance
		"--config", // Make it persistent
	}

	_, err = n.driver.Execute(args...)
	if err != nil {
		return "", err
	}

	return macStr, nil
}

func (NICs) randomMAC() ([]byte, error) {
	buf := make([]byte, 6)

	_, err := rand.Read(buf)
	if err != nil {
		return nil, err
	}

	// Set locally administered bit (bit 1 of first byte) and ensure unicast (bit 0 = 0)
	// This creates a valid private MAC address
	buf[0] = (buf[0] & 0xfe) | 0x02

	return buf, nil
}

func (NICs) userFriendly(buf []byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}
