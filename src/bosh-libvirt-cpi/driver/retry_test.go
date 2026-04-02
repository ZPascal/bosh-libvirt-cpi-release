package driver_test

import (
	"errors"
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/driver"
)

func TestDriver_Retry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Driver Retry Suite")
}

var _ = Describe("Retry Logic", func() {
	var logger boshlog.Logger

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	Context("AttemptsWithDelay", func() {
		It("succeeds immediately on first attempt", func() {
			attempts := 0
			fn := func() error {
				attempts++
				return nil
			}

			err := driver.AttemptsWithDelay(3, 0, fn, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(attempts).To(Equal(1))
		})

		It("retries until success", func() {
			attempts := 0
			fn := func() error {
				attempts++
				if attempts < 3 {
					return errors.New("temporary error")
				}
				return nil
			}

			err := driver.AttemptsWithDelay(5, 0, fn, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(attempts).To(Equal(3))
		})

		It("fails after max attempts", func() {
			attempts := 0
			fn := func() error {
				attempts++
				return errors.New("persistent error")
			}

			err := driver.AttemptsWithDelay(3, 0, fn, logger)
			Expect(err).To(HaveOccurred())
			Expect(attempts).To(Equal(3))
		})

		It("respects single attempt", func() {
			attempts := 0
			fn := func() error {
				attempts++
				return errors.New("error")
			}

			err := driver.AttemptsWithDelay(1, 0, fn, logger)
			Expect(err).To(HaveOccurred())
			Expect(attempts).To(Equal(1))
		})

		It("succeeds on last attempt", func() {
			attempts := 0
			fn := func() error {
				attempts++
				if attempts == 3 {
					return nil
				}
				return errors.New("error")
			}

			err := driver.AttemptsWithDelay(3, 0, fn, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(attempts).To(Equal(3))
		})

		It("fails on attempt one before success", func() {
			attempts := 0
			fn := func() error {
				attempts++
				if attempts == 3 {
					return nil
				}
				return errors.New("error")
			}

			err := driver.AttemptsWithDelay(2, 0, fn, logger)
			Expect(err).To(HaveOccurred())
			Expect(attempts).To(Equal(2))
		})

		It("handles zero delay", func() {
			attempts := 0
			fn := func() error {
				attempts++
				return nil
			}

			err := driver.AttemptsWithDelay(3, 0, fn, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(attempts).To(Equal(1))
		})

		It("returns nil when all attempts fail with different errors", func() {
			fn := func() error {
				return errors.New("different error each time")
			}

			err := driver.AttemptsWithDelay(2, 0, fn, logger)
			Expect(err).To(HaveOccurred())
		})

		It("stops retrying after success", func() {
			attempts := 0
			fn := func() error {
				attempts++
				if attempts <= 2 {
					return errors.New("error")
				}
				return nil
			}

			err := driver.AttemptsWithDelay(5, 0, fn, logger)
			Expect(err).ToNot(HaveOccurred())
			// Should only try 3 times, not all 5
			Expect(attempts).To(Equal(3))
		})
	})

	Context("Retry Robustness", func() {
		It("handles many retries", func() {
			attempts := 0
			fn := func() error {
				attempts++
				if attempts > 20 {
					return nil
				}
				return errors.New("retry")
			}

			err := driver.AttemptsWithDelay(30, 0, fn, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(attempts).To(Equal(21))
		})

		It("continues until max attempts", func() {
			fn := func() error {
				return errors.New("always fails")
			}

			err := driver.AttemptsWithDelay(5, 0, fn, logger)
			Expect(err).To(HaveOccurred())
		})
	})
})
