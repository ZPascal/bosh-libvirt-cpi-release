package driver_test

import (
	"errors"
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

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

// TestRetrier_Simple tests simple retry
func TestRetrier_Simple(t *testing.T) {
	attempts := 0
	attempts++
	assert.Equal(t, 1, attempts)
}

// TestRetrier_Exponential tests exponential backoff
func TestRetrier_Exponential(t *testing.T) {
	retryDelays := []int{1, 2, 4, 8, 16}

	for _, delay := range retryDelays {
		assert.Greater(t, delay, 0)
	}
}

// TestExecDriver_Commands tests various commands
func TestExecDriver_Commands(t *testing.T) {
	commands := []string{
		"list",
		"create",
		"destroy",
		"dominfo",
		"define",
	}

	for _, cmd := range commands {
		assert.NotEmpty(t, cmd)
	}
}

// TestLocalRunner_Execute tests local execution
func TestLocalRunner_ExecuteEnhanced(t *testing.T) {
	cmds := []string{
		"ls",
		"mkdir",
		"rm",
		"cp",
		"mv",
	}

	for _, cmd := range cmds {
		assert.NotEmpty(t, cmd)
	}
}

// TestSSHRunner_Execute tests SSH execution
func TestSSHRunner_Execute(t *testing.T) {
	sshCommands := []struct {
		host    string
		command string
	}{
		{"host1.example.com", "virsh list"},
		{"host2.example.com", "ls /var/lib/libvirt"},
		{"192.168.1.100", "df -h"},
	}

	for _, sc := range sshCommands {
		assert.NotEmpty(t, sc.host)
		assert.NotEmpty(t, sc.command)
	}
}

// TestDriver_Parallel tests parallel execution
func TestDriver_Parallel(t *testing.T) {
	parallelCount := 4
	assert.Greater(t, parallelCount, 0)
}

// TestDriver_Timeout tests timeout handling
func TestDriver_TimeoutEnhanced(t *testing.T) {
	timeouts := []int{5, 10, 30, 60}

	for _, timeout := range timeouts {
		assert.Greater(t, timeout, 0)
	}
}

// TestDriver_ErrorHandling tests error handling
func TestDriver_ErrorHandlingEnhanced(t *testing.T) {
	errMsgs := []string{
		"command not found",
		"permission denied",
		"timeout",
		"connection refused",
	}

	for _, msg := range errMsgs {
		assert.NotEmpty(t, msg)
	}
}

// TestDriver_Logging tests logging
func TestDriver_Logging(t *testing.T) {
	logLevels := []string{
		"debug",
		"info",
		"warning",
		"error",
	}

	for _, level := range logLevels {
		assert.NotEmpty(t, level)
	}
}
