package stemcell

import (
	"testing"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bosh-libvirt-cpi/driver"
)

// Mock Driver for testing
type mockDriver struct {
	executeOutput string
	executeErr    error
	executeCalls  int
	missingVMErr  bool
	isMissingVMResult bool
}

func (m *mockDriver) Execute(args ...string) (string, error) {
	m.executeCalls++
	if m.executeErr != nil {
		return "", m.executeErr
	}
	return m.executeOutput, nil
}

func (m *mockDriver) ExecuteComplex(args []string, opts driver.ExecuteOpts) (string, error) {
	return m.Execute(args...)
}

func (m *mockDriver) IsMissingVMErr(output string) bool {
	return m.isMissingVMResult
}


// Mock Runner for testing
type mockRunnerStemcell struct {
	executeOutput string
	exitCode      int
	executeErr    error
	executeCalls  int
}

func (m *mockRunnerStemcell) Execute(path string, args ...string) (string, int, error) {
	m.executeCalls++
	if m.executeErr != nil {
		return "", 1, m.executeErr
	}
	return m.executeOutput, m.exitCode, nil
}

func (m *mockRunnerStemcell) Upload(srcDir, dstDir string) error {
	return nil
}

func (m *mockRunnerStemcell) Put(path string, contents []byte) error {
	return nil
}

func (m *mockRunnerStemcell) Get(path string) ([]byte, error) {
	return nil, nil
}

func (m *mockRunnerStemcell) HomeDir() (string, error) {
	return "/home/test", nil
}

func TestStemcellImpl_ID(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewStemcellCID("stemcell-123")
	driver := &mockDriver{}
	runner := &mockRunnerStemcell{}

	stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

	assert.Equal(t, cid, stemcell.ID())
}

func TestStemcellImpl_SnapshotName(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewStemcellCID("stemcell-123")
	driver := &mockDriver{}
	runner := &mockRunnerStemcell{}

	stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

	expectedName := "prepared-clone"
	assert.Equal(t, expectedName, stemcell.SnapshotName())
}

func TestStemcellImpl_Prepare(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewStemcellCID("stemcell-123")

	t.Run("successfully prepares stemcell", func(t *testing.T) {
		driver := &mockDriver{executeOutput: "Snapshot created"}
		runner := &mockRunnerStemcell{}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		err := stemcell.Prepare()

		require.NoError(t, err)
		assert.Equal(t, 1, driver.executeCalls)
	})

	t.Run("returns error when snapshot creation fails", func(t *testing.T) {
		driver := &mockDriver{executeErr: assert.AnError}
		runner := &mockRunnerStemcell{}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		err := stemcell.Prepare()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Preparing stemcell")
	})

	t.Run("passes correct arguments to driver", func(t *testing.T) {
		driver := &mockDriver{executeOutput: ""}
		runner := &mockRunnerStemcell{}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)
		err := stemcell.Prepare()

		require.NoError(t, err)
		assert.Equal(t, 1, driver.executeCalls)
	})
}

func TestStemcellImpl_Exists(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewStemcellCID("stemcell-123")

	t.Run("returns true when stemcell exists", func(t *testing.T) {
		driver := &mockDriver{
			executeOutput: "Name:        stemcell-123\nState: running",
			isMissingVMResult: false,
		}
		runner := &mockRunnerStemcell{}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		exists, err := stemcell.Exists()

		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, 1, driver.executeCalls)
	})

	t.Run("returns false when stemcell does not exist", func(t *testing.T) {
		driver := &mockDriver{
			executeOutput: "error: Domain not found",
			executeErr: assert.AnError,
			isMissingVMResult: true,
		}
		runner := &mockRunnerStemcell{}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		exists, err := stemcell.Exists()

		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns error for other execute errors", func(t *testing.T) {
		driver := &mockDriver{
			executeErr: assert.AnError,
			isMissingVMResult: false,
		}
		runner := &mockRunnerStemcell{}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		exists, err := stemcell.Exists()

		assert.Error(t, err)
		assert.False(t, exists)
	})
}

func TestStemcellImpl_Delete(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewStemcellCID("stemcell-123")

	t.Run("successfully deletes stemcell", func(t *testing.T) {
		driver := &mockDriver{executeOutput: ""}
		runner := &mockRunnerStemcell{executeOutput: ""}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		err := stemcell.Delete()

		require.NoError(t, err)
		assert.Equal(t, 1, driver.executeCalls)
		assert.Equal(t, 1, runner.executeCalls)
	})

	t.Run("handles driver undefine error when domain not found", func(t *testing.T) {
		driver := &mockDriver{
			executeErr: assert.AnError,
			isMissingVMResult: true,
		}
		runner := &mockRunnerStemcell{executeOutput: ""}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		err := stemcell.Delete()

		require.NoError(t, err)
		assert.Equal(t, 1, runner.executeCalls)
	})

	t.Run("returns error when driver undefine fails", func(t *testing.T) {
		driver := &mockDriver{
			executeErr: assert.AnError,
			isMissingVMResult: false,
		}
		runner := &mockRunnerStemcell{}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		err := stemcell.Delete()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Undefining stemcell")
	})

	t.Run("returns error when directory deletion fails", func(t *testing.T) {
		driver := &mockDriver{executeOutput: ""}
		runner := &mockRunnerStemcell{executeErr: assert.AnError}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		err := stemcell.Delete()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Deleting stemcell directory")
	})

	t.Run("passes correct arguments to undefine command", func(t *testing.T) {
		driver := &mockDriver{executeOutput: ""}
		runner := &mockRunnerStemcell{executeOutput: ""}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)
		err := stemcell.Delete()

		require.NoError(t, err)
		assert.Equal(t, 1, driver.executeCalls)
	})
}

func TestStemcellImpl_ComprehensiveLifecycle(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cid := apiv1.NewStemcellCID("stemcell-full-lifecycle")

	t.Run("complete stemcell lifecycle", func(t *testing.T) {
		driver := &mockDriver{executeOutput: ""}
		runner := &mockRunnerStemcell{executeOutput: ""}

		stemcell := NewStemcellImpl(cid, "/tmp/stemcell", driver, runner, logger)

		// Prepare
		err := stemcell.Prepare()
		require.NoError(t, err)

		// Check if exists
		exists, err := stemcell.Exists()
		require.NoError(t, err)
		assert.True(t, exists)

		// Delete
		err = stemcell.Delete()
		require.NoError(t, err)

		// Verify total calls (Prepare, Exists, Delete)
		assert.Equal(t, 3, driver.executeCalls)
		assert.Equal(t, 1, runner.executeCalls)
	})
}

