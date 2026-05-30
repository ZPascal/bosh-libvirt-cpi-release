package domains_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDomains(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Domains Suite")
}
