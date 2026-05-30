//go:build integration

package stemcell_test

import (
	"os"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Stemcell (integration)", func() {
	BeforeEach(func() {
		if os.Getenv("LIBVIRT_URI") == "" {
			Skip("LIBVIRT_URI not set")
		}
	})

	It("imports a stemcell image and creates a domain", func() {
		// Full flow: stemcell import → domain defined
		Skip("requires real stemcell tarball — set STEMCELL_PATH to enable")
	})
})
