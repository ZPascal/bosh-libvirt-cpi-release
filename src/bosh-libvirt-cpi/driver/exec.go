package driver

import (
	"regexp"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var (
	// Generic error patterns for command execution
	execDriverNotReadyErr = regexp.MustCompile("error:")
	execDriverGenericErr  = regexp.MustCompile("error:")
)

type ExecDriver struct {
	runner  Runner
	retrier Retrier
	binPath string

	logTag string
	logger boshlog.Logger
}

func NewExecDriver(runner Runner, retrier Retrier, binPath string, logger boshlog.Logger) ExecDriver {
	return ExecDriver{
		runner:  runner,
		retrier: retrier,
		binPath: binPath,

		logTag: "driver.ExecDriver",
		logger: logger,
	}
}

func (d ExecDriver) Execute(args ...string) (string, error) {
	return d.ExecuteComplex(args, ExecuteOpts{})
}

func (d ExecDriver) ExecuteComplex(args []string, opts ExecuteOpts) (string, error) {
	var output string
	var status int

	execFunc := func() error {
		var err error

		output, status, err = d.runner.Execute(d.binPath, args...)
		if err != nil {
			return RetryableErrorImpl{Err: err}
		}

		if status != 0 && execDriverNotReadyErr.MatchString(output) {
			return RetryableErrorImpl{Err: bosherr.Errorf("Command not ready")}
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
		if status == 126 {
			// Binary executable but failed to execute
			return output, bosherr.Errorf("Most likely corrupted installation or missing dependencies")
		} else {
			errored = !opts.IgnoreNonZeroExitStatus
		}
	} else {
		// Sometimes commands fail but don't return non-zero exit code
		if execDriverGenericErr.MatchString(output) {
			d.logger.Debug(d.logTag, "Error text found in output, assuming error.")
			errored = true
		}
	}

	if errored {
		return output, bosherr.Errorf("Error executing command:\nCommand: '%v'\nExit code: %d\nOutput: '%s'", args, status, output)
	}

	return output, nil
}

func (d ExecDriver) IsMissingVMErr(output string) bool {
	// Check for common libvirt error messages indicating missing domain
	return strings.Contains(output, "Domain not found") ||
		strings.Contains(output, "failed to get domain") ||
		strings.Contains(output, "no domain with matching")
}
