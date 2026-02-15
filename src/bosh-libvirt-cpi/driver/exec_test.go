package driver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func TestExecDriver_Execute(t *testing.T) {
	t.Run("executes command successfully", func(t *testing.T) {
		mockRunner := &mockRunner{
			output: "success",
			status: 0,
		}
		mockRetrier := &mockRetrier{}
		logger := boshlog.NewLogger(boshlog.LevelNone)

		driver := NewExecDriver(mockRunner, mockRetrier, "virsh", logger)
		output, err := driver.Execute("list", "--all")

		require.NoError(t, err)
		assert.Equal(t, "success", output)
		assert.True(t, mockRunner.executeCalled)
	})

	t.Run("handles command error", func(t *testing.T) {
		mockRunner := &mockRunner{
			output: "error output",
			status: 1,
			err:    assert.AnError,
		}
		mockRetrier := &mockRetrier{}
		logger := boshlog.NewLogger(boshlog.LevelNone)

		driver := NewExecDriver(mockRunner, mockRetrier, "virsh", logger)
		output, err := driver.Execute("invalid", "command")

		assert.Error(t, err)
		assert.Contains(t, output, "error output")
	})

	t.Run("normalizes line endings", func(t *testing.T) {
		mockRunner := &mockRunner{
			output: "line1\r\nline2\r\nline3",
			status: 0,
		}
		mockRetrier := &mockRetrier{}
		logger := boshlog.NewLogger(boshlog.LevelNone)

		driver := NewExecDriver(mockRunner, mockRetrier, "virsh", logger)
		output, err := driver.Execute("test")

		require.NoError(t, err)
		assert.Equal(t, "line1\nline2\nline3", output)
	})
}

func TestExecDriver_ExecuteComplex(t *testing.T) {
	t.Run("ignores non-zero exit status when configured", func(t *testing.T) {
		mockRunner := &mockRunner{
			output: "warning output",
			status: 1,
		}
		mockRetrier := &mockRetrier{}
		logger := boshlog.NewLogger(boshlog.LevelNone)

		driver := NewExecDriver(mockRunner, mockRetrier, "virsh", logger)
		output, err := driver.ExecuteComplex(
			[]string{"test"},
			ExecuteOpts{IgnoreNonZeroExitStatus: true},
		)

		require.NoError(t, err)
		assert.Equal(t, "warning output", output)
	})

	t.Run("returns error for non-zero exit status by default", func(t *testing.T) {
		mockRunner := &mockRunner{
			output: "error output",
			status: 1,
		}
		mockRetrier := &mockRetrier{}
		logger := boshlog.NewLogger(boshlog.LevelNone)

		driver := NewExecDriver(mockRunner, mockRetrier, "virsh", logger)
		_, err := driver.ExecuteComplex(
			[]string{"test"},
			ExecuteOpts{},
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Error executing command")
	})
}

func TestExecDriver_IsMissingVMErr(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	driver := NewExecDriver(&mockRunner{}, &mockRetrier{}, "virsh", logger)

	t.Run("detects Domain not found", func(t *testing.T) {
		output := "error: Domain not found: no domain with matching name 'test-vm'"
		assert.True(t, driver.IsMissingVMErr(output))
	})

	t.Run("detects failed to get domain", func(t *testing.T) {
		output := "error: failed to get domain 'test-vm'"
		assert.True(t, driver.IsMissingVMErr(output))
	})

	t.Run("detects no domain with matching", func(t *testing.T) {
		output := "error: no domain with matching uuid 'abc-123'"
		assert.True(t, driver.IsMissingVMErr(output))
	})

	t.Run("does not detect false positives", func(t *testing.T) {
		output := "VM is running successfully"
		assert.False(t, driver.IsMissingVMErr(output))
	})

	t.Run("does not detect other errors", func(t *testing.T) {
		output := "error: permission denied"
		assert.False(t, driver.IsMissingVMErr(output))
	})
}

// Mock implementations

type mockRunner struct {
	executeCalled bool
	path          string
	args          []string
	output        string
	status        int
	err           error
}

func (r *mockRunner) Execute(path string, args ...string) (string, int, error) {
	r.executeCalled = true
	r.path = path
	r.args = args
	return r.output, r.status, r.err
}

func (r *mockRunner) Upload(srcDir, dstDir string) error {
	return nil
}

func (r *mockRunner) Put(path string, contents []byte) error {
	return nil
}

func (r *mockRunner) Get(path string) ([]byte, error) {
	return nil, nil
}

type mockRetrier struct {
	retryCalled bool
}

func (r *mockRetrier) Retry(fn func() error) error {
	r.retryCalled = true
	return fn()
}

func (r *mockRetrier) RetryComplex(fn func() error, attempts int, delay time.Duration) error {
	return fn()
}
