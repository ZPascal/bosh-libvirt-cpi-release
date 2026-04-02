package network_test

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/vm/network"
)

var _ = Describe("Network Types", func() {
	Context("Network Type Constants", func() {
		It("defines NAT network type", func() {
			Expect(network.NATType).To(Equal("nat"))
		})

		It("defines NAT network type with name", func() {
			Expect(network.NATNetworkType).To(Equal("natnetwork"))
		})

		It("defines host-only network type", func() {
			Expect(network.HostOnlyType).To(Equal("hostonly"))
		})

		It("defines bridged network type", func() {
			Expect(network.BridgedType).To(Equal("bridged"))
		})
	})

	Context("Network Constants", func() {
		It("has all required network types", func() {
			types := []string{
				network.NATType,
				network.NATNetworkType,
				network.HostOnlyType,
				network.BridgedType,
			}

			for _, t := range types {
				Expect(t).NotTo(BeEmpty())
			}
		})

		It("network types are unique", func() {
			types := map[string]bool{
				network.NATType:        true,
				network.NATNetworkType: true,
				network.HostOnlyType:   true,
				network.BridgedType:    true,
			}

			Expect(len(types)).To(Equal(4))
		})
	})
})

var _ = Describe("Network Interface", func() {
	var (
		mockNet *testNetwork
	)

	BeforeEach(func() {
		mockNet = &testNetwork{
			name:        "test-network",
			description: "Test Network",
			enabled:     true,
			dhcp:        true,
			ipnet: &net.IPNet{
				IP:   net.ParseIP("192.168.1.0"),
				Mask: net.IPv4Mask(255, 255, 255, 0),
			},
		}
	})

	Context("Network Information", func() {
		It("returns network name", func() {
			Expect(mockNet.Name()).To(Equal("test-network"))
		})

		It("returns network description", func() {
			Expect(mockNet.Description()).To(Equal("Test Network"))
		})

		It("checks if network is enabled", func() {
			Expect(mockNet.IsEnabled()).To(BeTrue())
		})

		It("returns enabled description", func() {
			desc := mockNet.EnabledDescription()
			Expect(desc).NotTo(BeEmpty())
		})

		It("checks DHCP status", func() {
			Expect(mockNet.IsDHCPEnabled()).To(BeTrue())
		})

		It("returns network IP range", func() {
			ipnet := mockNet.IPNet()
			Expect(ipnet).NotTo(BeNil())
			Expect(ipnet.IP).To(Equal(net.ParseIP("192.168.1.0")))
		})
	})

	Context("Network Operations", func() {
		It("enables network", func() {
			err := mockNet.Enable()
			Expect(err).NotTo(HaveOccurred())
		})

		It("handles disabled networks", func() {
			mockNet.enabled = false
			Expect(mockNet.IsEnabled()).To(BeFalse())
		})

		It("handles networks with DHCP disabled", func() {
			mockNet.dhcp = false
			Expect(mockNet.IsDHCPEnabled()).To(BeFalse())
		})
	})
})

var _ = Describe("Network IP Configuration", func() {
	Context("IPv4 Networks", func() {
		It("creates IPv4 network", func() {
			_, ipnet, err := net.ParseCIDR("192.168.1.0/24")
			Expect(err).NotTo(HaveOccurred())
			Expect(ipnet).NotTo(BeNil())
		})

		It("validates private IP ranges", func() {
			privateRanges := []string{
				"10.0.0.0/8",
				"172.16.0.0/12",
				"192.168.0.0/16",
			}

			for _, cidr := range privateRanges {
				_, ipnet, err := net.ParseCIDR(cidr)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipnet).NotTo(BeNil())
			}
		})
	})

	Context("IPv6 Networks", func() {
		It("creates IPv6 network", func() {
			_, ipnet, err := net.ParseCIDR("fe80::/10")
			Expect(err).NotTo(HaveOccurred())
			Expect(ipnet).NotTo(BeNil())
		})

		It("validates link-local addresses", func() {
			ip := net.ParseIP("fe80::1")
			Expect(ip).NotTo(BeNil())
		})
	})

	Context("Network Masks", func() {
		It("creates /24 network mask", func() {
			mask := net.IPv4Mask(255, 255, 255, 0)
			Expect(mask).NotTo(BeNil())
			Expect(mask.String()).To(Equal("ffffff00"))
		})

		It("creates /16 network mask", func() {
			mask := net.IPv4Mask(255, 255, 0, 0)
			Expect(mask).NotTo(BeNil())
		})

		It("creates /8 network mask", func() {
			mask := net.IPv4Mask(255, 0, 0, 0)
			Expect(mask).NotTo(BeNil())
		})
	})
})

// testNetwork is a test implementation of network.Network
type testNetwork struct {
	name        string
	description string
	enabled     bool
	dhcp        bool
	ipnet       *net.IPNet
}

func (n *testNetwork) Name() string {
	return n.name
}

func (n *testNetwork) Description() string {
	return n.description
}

func (n *testNetwork) IsEnabled() bool {
	return n.enabled
}

func (n *testNetwork) EnabledDescription() string {
	if n.enabled {
		return "Enabled"
	}
	return "Disabled"
}

func (n *testNetwork) Enable() error {
	n.enabled = true
	return nil
}

func (n *testNetwork) IsDHCPEnabled() bool {
	return n.dhcp
}

func (n *testNetwork) IPNet() *net.IPNet {
	return n.ipnet
}

