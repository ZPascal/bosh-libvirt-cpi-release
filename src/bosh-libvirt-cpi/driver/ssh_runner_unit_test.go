package driver_test

import (
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"bosh-libvirt-cpi/driver"
)

// TestSSHRunnerInitialization tests SSH runner initialization
func TestSSHRunnerInitialization(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	fs := boshsys.NewOsFileSystem(logger)

	tests := []struct {
		name string
		opts driver.SSHRunnerOpts
	}{
		{"localhost", driver.SSHRunnerOpts{Host: "localhost", Username: "user"}},
		{"ip address", driver.SSHRunnerOpts{Host: "192.168.1.100", Username: "ubuntu"}},
		{"hostname", driver.SSHRunnerOpts{Host: "example.com", Username: "root"}},
		{"with private key", driver.SSHRunnerOpts{Host: "host", Username: "user", PrivateKey: "key"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := driver.NewSSHRunner(tt.opts, fs, logger)
			if runner == nil {
				t.Fatal("expected non-nil runner")
			}
		})
	}
}

// TestSSHRunnerInitializesWithDifferentHosts tests various host formats
func TestSSHRunnerInitializesWithDifferentHosts(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	fs := boshsys.NewOsFileSystem(logger)

	hosts := []string{
		"localhost",
		"127.0.0.1",
		"10.0.0.1",
		"192.168.1.100",
		"example.com",
		"vm.local",
	}

	for _, host := range hosts {
		t.Run("host_"+host, func(t *testing.T) {
			opts := driver.SSHRunnerOpts{
				Host:     host,
				Username: "user",
			}
			runner := driver.NewSSHRunner(opts, fs, logger)
			if runner == nil {
				t.Fatalf("failed to create runner for host %s", host)
			}
		})
	}
}

// TestSSHRunnerInitializesWithDifferentUsernames tests various usernames
func TestSSHRunnerInitializesWithDifferentUsernames(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	fs := boshsys.NewOsFileSystem(logger)

	users := []string{"root", "ubuntu", "ec2-user", "admin", "bosh"}

	for _, user := range users {
		t.Run("user_"+user, func(t *testing.T) {
			opts := driver.SSHRunnerOpts{
				Host:     "localhost",
				Username: user,
			}
			runner := driver.NewSSHRunner(opts, fs, logger)
			if runner == nil {
				t.Fatalf("failed to create runner for user %s", user)
			}
		})
	}
}

// TestSSHRunnerHandlesPrivateKeys tests different private key formats
func TestSSHRunnerHandlesPrivateKeys(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	fs := boshsys.NewOsFileSystem(logger)

	tests := []struct {
		name       string
		privateKey string
	}{
		{"empty key", ""},
		{"rsa key", "-----BEGIN RSA PRIVATE KEY-----\ndata\n-----END RSA PRIVATE KEY-----"},
		{"openssh key", "-----BEGIN OPENSSH PRIVATE KEY-----\ndata\n-----END OPENSSH PRIVATE KEY-----"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := driver.SSHRunnerOpts{
				Host:       "localhost",
				Username:   "user",
				PrivateKey: tt.privateKey,
			}
			runner := driver.NewSSHRunner(opts, fs, logger)
			if runner == nil {
				t.Fatal("expected non-nil runner")
			}
		})
	}
}
