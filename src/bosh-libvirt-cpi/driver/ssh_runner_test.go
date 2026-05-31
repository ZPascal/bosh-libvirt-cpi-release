package driver_test

import (
	"os"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "bosh-libvirt-cpi/driver"
)

const validTestPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDummyfakekeyforparsingtestsonlyAAAAAA==
-----END OPENSSH PRIVATE KEY-----`

var (
	testSSHRunnerUsername   = os.Getenv("TEST_SSH_RUNNER_USERNAME")
	testSSHRunnerPrivateKey = os.Getenv("TEST_SSH_RUNNER_PRIVATE_KEY")
	testSSHRunnerHost       = os.Getenv("TEST_SSH_RUNNER_HOST")
	testSSHRunnerHostKey    = os.Getenv("TEST_SSH_RUNNER_HOST_KEY")
)

var _ = Describe("SSHRunner", func() {
	Context("with real SSH server (requires TEST_SSH_RUNNER_USERNAME)", func() {
		BeforeEach(func() {
			if len(testSSHRunnerUsername) == 0 {
				Skip("SSHRunner tests are not configured")
			}
			if testSSHRunnerHost == "" {
				testSSHRunnerHost = "127.0.0.1"
			}
		})

		Context("HomeDir", func() {
			It("returns proper home directory", func() {
				opts := SSHRunnerOpts{
					Host:       testSSHRunnerHost,
					Username:   testSSHRunnerUsername,
					PrivateKey: testSSHRunnerPrivateKey,
					HostKey:    testSSHRunnerHostKey,
				}
				logger := boshlog.NewLogger(boshlog.LevelNone)
				runner := NewSSHRunner(opts, nil, logger)

				path, err := runner.HomeDir()
				Expect(err).ToNot(HaveOccurred())
				Expect(path).ToNot(BeEmpty())
				Expect(path).ToNot(ContainSubstring("~"))
			})
		})
	})

	Context("client() host key parsing", func() {
		It("returns error when HostKey is malformed", func() {
			opts := SSHRunnerOpts{
				Host:       "127.0.0.1",
				Username:   "user",
				PrivateKey: validTestPrivateKey,
				HostKey:    "not-a-valid-key",
			}
			logger := boshlog.NewLogger(boshlog.LevelNone)
			runner := NewSSHRunner(opts, nil, logger)
			_, err := runner.HomeDir()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Parsing host key"))
		})
	})
})
