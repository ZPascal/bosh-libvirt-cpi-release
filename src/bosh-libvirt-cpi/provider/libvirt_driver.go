package provider

import (
	"regexp"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/driver"
)

var (
	libvirtNotReadyErr       = regexp.MustCompile("error: failed to connect")
	libvirtGenericErr        = regexp.MustCompile("error:")
	libvirtDomainNotFoundErr = regexp.MustCompile("error: failed to get domain")
)

// LibvirtDriver is a driver implementation for libvirt/virsh
type LibvirtDriver struct {
	runner  driver.Runner
	retrier driver.Retrier
	binPath string
	uri     string

	logTag string
	logger boshlog.Logger
}

// NewLibvirtDriver creates a new libvirt driver
func NewLibvirtDriver(
	runner driver.Runner,
	retrier driver.Retrier,
	binPath string,
	uri string,
	logger boshlog.Logger,
) LibvirtDriver {
	return LibvirtDriver{
		runner:  runner,
		retrier: retrier,
		binPath: binPath,
		uri:     uri,
		logTag:  "driver.LibvirtDriver",
		logger:  logger,
	}
}

func (d LibvirtDriver) Execute(args ...string) (string, error) {
	return d.ExecuteComplex(args, driver.ExecuteOpts{})
}

func (d LibvirtDriver) ExecuteComplex(args []string, opts driver.ExecuteOpts) (string, error) {
	var output string
	var status int

	// Prepend connection URI to args
	fullArgs := []string{"-c", d.uri}
	fullArgs = append(fullArgs, args...)

	execFunc := func() error {
		var err error

		output, status, err = d.runner.Execute(d.binPath, fullArgs...)
		if err != nil {
			return driver.RetryableErrorImpl{Err: err}
		}

		if status != 0 && libvirtNotReadyErr.MatchString(output) {
			return driver.RetryableErrorImpl{Err: bosherr.Errorf("Libvirt not ready")}
		}

		return nil
	}

	err := d.retrier.Retry(execFunc)
	output = strings.Replace(output, "\r\n", "\n", -1)
	if err != nil {
		return output, err
	}

	var errored bool

	if status != 0 {
		errored = !opts.IgnoreNonZeroExitStatus
	} else {
		// Check for error messages in output even with zero exit code
		if libvirtGenericErr.MatchString(output) {
			d.logger.Debug(d.logTag, "Libvirt error text found, assuming error.")
			errored = true
		}
	}

	if errored {
		return output, bosherr.Errorf("Error executing command:\nCommand: '%v'\nExit code: %d\nOutput: '%s'", args, status, output)
	}

	return output, nil
}

func (d LibvirtDriver) IsMissingVMErr(output string) bool {
	return libvirtDomainNotFoundErr.MatchString(output) ||
		strings.Contains(output, "Domain not found") ||
		strings.Contains(output, "failed to get domain")
}
