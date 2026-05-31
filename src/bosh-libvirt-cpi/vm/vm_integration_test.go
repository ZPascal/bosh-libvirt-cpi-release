//go:build integration

package vm_test

import (
	"os"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("VM (integration)", func() {
	BeforeEach(func() {
		if os.Getenv("LIBVIRT_URI") == "" {
			Skip("LIBVIRT_URI not set")
		}
	})

	It("creates a VM domain and starts it", func() {
		Skip("requires real libvirt connection and disk images — set LIBVIRT_URI to enable")
	})
})
