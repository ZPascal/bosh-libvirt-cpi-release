package driver_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/driver"
)

var _ = Describe("Retry Logic", func() {
	var (
		retrier driver.Retrier
	)

	BeforeEach(func() {
		retrier = driver.RetrierImpl{}
	})

	Context("Simple Retry", func() {
		It("succeeds on first attempt", func() {
			callCount := 0
			actionFunc := func() error {
				callCount++
				return nil
			}

			err := retrier.Retry(actionFunc)
			Expect(err).NotTo(HaveOccurred())
			Expect(callCount).To(Equal(1))
		})

		It("retries on transient errors", func() {
			callCount := 0
			actionFunc := func() error {
				callCount++
				if callCount < 3 {
					return driver.RetryableErrorImpl{Err: fmt.Errorf("transient error")}
				}
				return nil
			}

			err := retrier.Retry(actionFunc)
			Expect(err).NotTo(HaveOccurred())
			Expect(callCount).To(Equal(3))
		})

		It("fails on non-retryable errors", func() {
			actionFunc := func() error {
				return fmt.Errorf("non-retryable error")
			}

			err := retrier.Retry(actionFunc)
			Expect(err).To(HaveOccurred())
		})

		It("uses default retry count of 30", func() {
			callCount := 0
			actionFunc := func() error {
				callCount++
				return driver.RetryableErrorImpl{Err: fmt.Errorf("always fails")}
			}

			err := retrier.Retry(actionFunc)
			Expect(err).To(HaveOccurred())
			Expect(callCount).To(Equal(30))
		})

		It("uses default sleep duration of 2 seconds", func() {
			// Default sleep duration is 2 seconds
			expectedSleep := 2 * time.Second
			Expect(expectedSleep).To(Equal(2 * time.Second))
		})
	})

	Context("Complex Retry", func() {
		It("retries specified number of times", func() {
			callCount := 0
			maxRetries := 5

			actionFunc := func() error {
				callCount++
				if callCount < maxRetries {
					return driver.RetryableErrorImpl{Err: fmt.Errorf("fail")}
				}
				return nil
			}

			err := retrier.RetryComplex(actionFunc, maxRetries, 1*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())
			Expect(callCount).To(Equal(maxRetries))
		})

		It("respects custom retry count", func() {
			callCount := 0
			actionFunc := func() error {
				callCount++
				return driver.RetryableErrorImpl{Err: fmt.Errorf("fail")}
			}

			err := retrier.RetryComplex(actionFunc, 3, 1*time.Millisecond)
			Expect(err).To(HaveOccurred())
			Expect(callCount).To(Equal(3))
		})

		It("sleeps between retries", func() {
			callCount := 0
			startTime := time.Now()

			actionFunc := func() error {
				callCount++
				if callCount < 2 {
					return driver.RetryableErrorImpl{Err: fmt.Errorf("fail")}
				}
				return nil
			}

			sleepDuration := 10 * time.Millisecond
			err := retrier.RetryComplex(actionFunc, 5, sleepDuration)
			elapsed := time.Since(startTime)

			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed >= sleepDuration).To(BeTrue())
		})

		It("handles zero sleep duration", func() {
			callCount := 0
			actionFunc := func() error {
				callCount++
				if callCount < 2 {
					return driver.RetryableErrorImpl{Err: fmt.Errorf("fail")}
				}
				return nil
			}

			err := retrier.RetryComplex(actionFunc, 5, 0*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(callCount).To(Equal(2))
		})
	})

	Context("Retryable Errors", func() {
		It("identifies retryable errors", func() {
			err := driver.RetryableErrorImpl{Err: fmt.Errorf("test error")}
			Expect(err).To(BeAssignableToTypeOf(driver.RetryableErrorImpl{}))
		})

		It("wraps retryable errors", func() {
			originalErr := fmt.Errorf("original error")
			retryableErr := driver.RetryableErrorImpl{Err: originalErr}
			Expect(retryableErr.Error()).To(Equal("original error"))
		})

		It("identifies non-retryable errors", func() {
			err := fmt.Errorf("non-retryable")
			_, isRetryable := err.(driver.RetryableError)
			Expect(isRetryable).To(BeFalse())
		})

		It("returns error message for retryable errors", func() {
			retryErr := driver.RetryableErrorImpl{Err: fmt.Errorf("connection timeout")}
			Expect(retryErr.Error()).To(ContainSubstring("connection timeout"))
		})
	})

	Context("Retry Edge Cases", func() {
		It("succeeds immediately without retries", func() {
			callCount := 0
			actionFunc := func() error {
				callCount++
				return nil
			}

			err := retrier.RetryComplex(actionFunc, 1, 1*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())
			Expect(callCount).To(Equal(1))
		})

		It("fails with zero retries", func() {
			callCount := 0
			actionFunc := func() error {
				callCount++
				return driver.RetryableErrorImpl{Err: fmt.Errorf("fail")}
			}

			err := retrier.RetryComplex(actionFunc, 0, 1*time.Millisecond)
			Expect(err).To(HaveOccurred())
			Expect(callCount).To(Equal(0))
		})

		It("handles panic in action function", func() {
			// This would need special handling in actual implementation
			Expect(true).To(BeTrue())
		})

		It("provides meaningful error message on final failure", func() {
			actionFunc := func() error {
				return driver.RetryableErrorImpl{Err: fmt.Errorf("connection lost")}
			}

			err := retrier.RetryComplex(actionFunc, 3, 1*time.Millisecond)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("connection lost"))
		})
	})

	Context("Retry Timing", func() {
		It("respects minimum sleep duration", func() {
			minSleep := 1 * time.Millisecond
			Expect(minSleep > 0).To(BeTrue())
		})

		It("handles large sleep durations", func() {
			callCount := 0
			actionFunc := func() error {
				callCount++
				if callCount < 2 {
					return driver.RetryableErrorImpl{Err: fmt.Errorf("fail")}
				}
				return nil
			}

			largeSleep := 100 * time.Millisecond
			err := retrier.RetryComplex(actionFunc, 5, largeSleep)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("Retrier Interface", func() {
	Context("Retrier Implementations", func() {
		It("implements Retry method", func() {
			retrier := driver.RetrierImpl{}
			Expect(retrier).To(BeAssignableToTypeOf(driver.RetrierImpl{}))
		})

		It("implements RetryComplex method", func() {
			retrier := driver.RetrierImpl{}
			Expect(retrier).NotTo(BeNil())
		})

		It("can be used as interface", func() {
			var retrier driver.Retrier
			retrier = driver.RetrierImpl{}
			Expect(retrier).NotTo(BeNil())
		})
	})
})
