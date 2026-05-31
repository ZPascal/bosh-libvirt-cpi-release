package vm_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/driver/fakes"
	"bosh-libvirt-cpi/vm"
)

var _ = Describe("Store", func() {
	var (
		runner *fakes.FakeRunner
		store  vm.Store
	)

	BeforeEach(func() {
		runner = &fakes.FakeRunner{}
		store = vm.NewStore("/vms", runner)
	})

	Describe("List", func() {
		It("returns empty slice when directory is empty", func() {
			runner.ExecuteOutput = ""
			ids, err := store.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]string{}))
			Expect(len(ids)).To(Equal(0))
		})

		It("returns VM IDs without trailing empty strings when multiple VMs exist", func() {
			runner.ExecuteOutput = "vm-abc\nvm-def\n"
			ids, err := store.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]string{"vm-abc", "vm-def"}))
			Expect(len(ids)).To(Equal(2))
		})

		It("returns single VM ID without trailing empty string", func() {
			runner.ExecuteOutput = "vm-123\n"
			ids, err := store.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]string{"vm-123"}))
			Expect(len(ids)).To(Equal(1))
		})

		It("returns multiple VMs without empty strings in the middle", func() {
			runner.ExecuteOutput = "vm-1\nvm-2\nvm-3\n"
			ids, err := store.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]string{"vm-1", "vm-2", "vm-3"}))
			Expect(len(ids)).To(Equal(3))
		})

		It("propagates error from Execute", func() {
			runner.ExecuteErr = errors.New("execute failed")
			_, err := store.List()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Put", func() {
		It("rejects keys containing ..", func() {
			err := store.Put("../etc/passwd", []byte("data"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid key"))
		})

		It("rejects keys containing /", func() {
			err := store.Put("sub/dir", []byte("data"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid key"))
		})

		It("accepts normal keys", func() {
			err := store.Put("metadata.json", []byte("data"))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Get", func() {
		It("rejects keys containing ..", func() {
			_, err := store.Get("../etc/passwd")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid key"))
		})

		It("accepts normal keys", func() {
			runner.GetResult = []byte(`{"key":"value"}`)
			_, err := store.Get("agent.json")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("DeleteOne", func() {
		It("rejects keys containing ..", func() {
			err := store.DeleteOne("../../important")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid key"))
		})

		It("accepts normal keys", func() {
			err := store.DeleteOne("disk-abc-disk-attachment.json")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
