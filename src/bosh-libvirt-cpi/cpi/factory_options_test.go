package cpi_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	"bosh-libvirt-cpi/cpi"
)

var _ = Describe("FactoryOpts", func() {
	var opts cpi.FactoryOpts

	BeforeEach(func() {
		opts = cpi.FactoryOpts{
			BackendURI: "qemu:///system",
			StoreDir:   "/var/vcap/store",
			Agent: apiv1.AgentOptions{
				Mbus: "https://user:pass@127.0.0.1:4321/agent",
			},
		}
	})

	Describe("Validate", func() {
		It("succeeds with valid qemu options", func() {
			Expect(opts.Validate()).ToNot(HaveOccurred())
		})

		It("succeeds with vbox scheme", func() {
			opts.BackendURI = "vbox:///session"
			Expect(opts.Validate()).ToNot(HaveOccurred())
		})

		It("succeeds with lxc scheme", func() {
			opts.BackendURI = "lxc:///"
			Expect(opts.Validate()).ToNot(HaveOccurred())
		})

		It("returns error when BackendURI is empty", func() {
			opts.BackendURI = ""

			err := opts.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("BackendURI"))
		})

		It("returns error for qemu+ssh URI scheme (not yet supported)", func() {
			opts.BackendURI = "qemu+ssh://user@remote/system"
			err := opts.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unsupported BackendURI scheme"))
		})

		It("returns error when scheme is unknown", func() {
			opts.BackendURI = "weird:///foo"

			err := opts.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unsupported BackendURI scheme"))
		})

		It("returns error when StoreDir is empty", func() {
			opts.StoreDir = ""

			err := opts.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("StoreDir"))
		})

		It("returns error when Agent options invalid", func() {
			opts.Agent = apiv1.AgentOptions{}

			err := opts.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Agent"))
		})

		Context("when Host is set", func() {
			BeforeEach(func() {
				opts.Host = "remote.example.com"
			})

			It("returns error when Username is empty", func() {
				opts.Username = ""
				opts.PrivateKey = "key"

				err := opts.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Username"))
			})

			It("returns error when PrivateKey is empty", func() {
				opts.Username = "user"
				opts.PrivateKey = ""

				err := opts.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("PrivateKey"))
			})

			It("returns error when HostKey is empty", func() {
				opts.Username = "user"
				opts.PrivateKey = "key"
				opts.HostKey = ""

				err := opts.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("HostKey"))
			})

			It("succeeds when Host, Username, PrivateKey, and HostKey are all set", func() {
				opts.Username = "user"
				opts.PrivateKey = "key"
				opts.HostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."

				Expect(opts.Validate()).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Directory helpers", func() {
		BeforeEach(func() {
			opts.StoreDir = "/store"
		})

		It("returns StemcellsDir under StoreDir", func() {
			Expect(opts.StemcellsDir()).To(Equal("/store/stemcells"))
		})

		It("returns VMsDir under StoreDir", func() {
			Expect(opts.VMsDir()).To(Equal("/store/vms"))
		})

		It("returns DisksDir under StoreDir", func() {
			Expect(opts.DisksDir()).To(Equal("/store/disks"))
		})
	})
})
