package cpi_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	"bosh-libvirt-cpi/cpi"
	stemcellfakes "bosh-libvirt-cpi/stemcell/fakes"
)

var _ = Describe("Stemcells", func() {
	var (
		importer  *stemcellfakes.FakeImporter
		finder    *stemcellfakes.FakeStemcellFinder
		stemcells cpi.Stemcells
	)

	BeforeEach(func() {
		importer = &stemcellfakes.FakeImporter{}
		finder = &stemcellfakes.FakeStemcellFinder{}
		stemcells = cpi.NewStemcells(importer, finder)
	})

	Describe("CreateStemcell", func() {
		It("imports and returns stemcell ID", func() {
			fakeStemcell := stemcellfakes.NewFakeStemcell("sc-123")
			importer.ImportResult = fakeStemcell

			cid, err := stemcells.CreateStemcell("/path/to/image", apiv1.CloudPropsImpl{})
			Expect(err).ToNot(HaveOccurred())
			Expect(cid.AsString()).To(Equal("sc-123"))
			Expect(importer.ImportFromPathArg).To(Equal("/path/to/image"))
		})

		It("returns error when import fails", func() {
			importer.ImportErr = errors.New("import failed")

			_, err := stemcells.CreateStemcell("/path/to/image", apiv1.CloudPropsImpl{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("import failed"))
		})
	})

	Describe("DeleteStemcell", func() {
		It("finds and deletes stemcell", func() {
			fakeStemcell := stemcellfakes.NewFakeStemcell("sc-abc")
			finder.FindResult = fakeStemcell

			err := stemcells.DeleteStemcell(apiv1.NewStemcellCID("sc-abc"))
			Expect(err).ToNot(HaveOccurred())
			Expect(finder.FindArg.AsString()).To(Equal("sc-abc"))
		})

		It("returns error when finder fails", func() {
			finder.FindErr = errors.New("not found")

			err := stemcells.DeleteStemcell(apiv1.NewStemcellCID("sc-missing"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("returns error when delete fails", func() {
			fakeStemcell := stemcellfakes.NewFakeStemcell("sc-abc")
			fakeStemcell.DeleteErr = errors.New("delete failed")
			finder.FindResult = fakeStemcell

			err := stemcells.DeleteStemcell(apiv1.NewStemcellCID("sc-abc"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("delete failed"))
		})
	})
})
