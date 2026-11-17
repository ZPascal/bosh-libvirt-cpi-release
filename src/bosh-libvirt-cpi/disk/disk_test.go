package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	bdisk "bosh-libvirt-cpi/disk"
	driverfakes "bosh-libvirt-cpi/driver/fakes"
)

var _ = Describe("DiskImpl.Format", func() {
	var (
		runner *driverfakes.FakeRunner
		d      bdisk.DiskImpl
		probed []string
	)

	BeforeEach(func() {
		runner = &driverfakes.FakeRunner{}
		d = bdisk.NewDiskImpl(apiv1.NewDiskCID("disk-1"), "/disks/disk-1", runner, boshlog.NewLogger(boshlog.LevelNone))
	})

	respond := func(out string, err error) {
		runner.ExecuteFunc = func(name string, args ...string) (string, int, error) {
			probed = append([]string{name}, args...)
			return out, 0, err
		}
	}

	It("reports qcow2 for images with the QCOW magic (qemu-img create)", func() {
		respond("QFI\xfb\n", nil)
		Expect(d.Format()).To(Equal("qcow2"))
		Expect(probed).To(Equal([]string{"head", "-c", "4", "/disks/disk-1/disk.img"}))
	})

	It("reports raw for anything else (dd fallback: sparse zeros)", func() {
		respond("\x00\x00\x00\x00\n", nil)
		Expect(d.Format()).To(Equal("raw"))
	})

	It("returns the probe error", func() {
		respond("", errors.New("no such file"))
		_, err := d.Format()
		Expect(err).To(MatchError(ContainSubstring("no such file")))
	})
})
