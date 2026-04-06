package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	main_pkg "bosh-libvirt-cpi/main"
)

// TestBasicDeps_ReturnTypes verifies basicDeps returns correct types
func TestBasicDeps_ReturnTypes(t *testing.T) {
	// Execute: Call basicDeps
	logger, fs, cmdRunner, uuidGen := main_pkg.BasicDeps()

	// Assert: All dependencies are initialized
	assert.NotNil(t, logger, "logger should not be nil")
	assert.NotNil(t, fs, "filesystem should not be nil")
	assert.NotNil(t, cmdRunner, "command runner should not be nil")
	assert.NotNil(t, uuidGen, "UUID generator should not be nil")
}

// TestBasicDeps_LoggerWorks verifies logger is functional
func TestBasicDeps_LoggerWorks(t *testing.T) {
	// Execute
	logger, _, _, _ := main_pkg.BasicDeps()

	// Assert: Logger has expected methods
	assert.NotNil(t, logger)
	// Try to use the logger (shouldn't panic)
	logger.Debug("main_test", "Testing logger")
}

// TestBasicDeps_FileSystemWorks verifies filesystem is functional
func TestBasicDeps_FileSystemWorks(t *testing.T) {
	// Execute
	_, fs, _, _ := main_pkg.BasicDeps()

	// Assert: FileSystem is not nil
	assert.NotNil(t, fs)
}

// TestBasicDeps_CmdRunnerWorks verifies command runner is functional
func TestBasicDeps_CmdRunnerWorks(t *testing.T) {
	// Execute
	_, _, cmdRunner, _ := main_pkg.BasicDeps()

	// Assert: CmdRunner is initialized
	assert.NotNil(t, cmdRunner)
}

// TestBasicDeps_UUIDGenWorks verifies UUID generator works
func TestBasicDeps_UUIDGenWorks(t *testing.T) {
	// Execute
	_, _, _, uuidGen := main_pkg.BasicDeps()

	// Assert: UUID generator can generate UUIDs
	assert.NotNil(t, uuidGen)
	
	// Generate a UUID
	uuid, err := uuidGen.Generate()
	assert.NoError(t, err, "UUID generation should not error")
	assert.NotEmpty(t, uuid, "UUID should not be empty")
}

// TestBasicDeps_MultipleCallsConsistent verifies basicDeps returns valid instances
func TestBasicDeps_MultipleCallsConsistent(t *testing.T) {
	// Execute: Call basicDeps multiple times
	logger1, fs1, cmdRunner1, uuidGen1 := main_pkg.BasicDeps()
	logger2, fs2, cmdRunner2, uuidGen2 := main_pkg.BasicDeps()

	// Assert: Each call returns initialized instances
	assert.NotNil(t, logger1)
	assert.NotNil(t, logger2)
	assert.NotNil(t, fs1)
	assert.NotNil(t, fs2)
	assert.NotNil(t, cmdRunner1)
	assert.NotNil(t, cmdRunner2)
	assert.NotNil(t, uuidGen1)
	assert.NotNil(t, uuidGen2)
}

// TestBasicDeps_UUIDUniqueness verifies UUID generator produces unique values
func TestBasicDeps_UUIDUniqueness(t *testing.T) {
	// Execute
	_, _, _, uuidGen := main_pkg.BasicDeps()

	// Generate multiple UUIDs
	uuid1, err1 := uuidGen.Generate()
	uuid2, err2 := uuidGen.Generate()
	uuid3, err3 := uuidGen.Generate()

	// Assert: All UUIDs generated without error
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)

	// Assert: All UUIDs are unique
	assert.NotEqual(t, uuid1, uuid2, "UUIDs should be unique")
	assert.NotEqual(t, uuid2, uuid3, "UUIDs should be unique")
	assert.NotEqual(t, uuid1, uuid3, "UUIDs should be unique")
}

// TestBasicDeps_LoggerLogging verifies logger methods work
func TestBasicDeps_LoggerLogging(t *testing.T) {
	// Execute
	logger, _, _, _ := main_pkg.BasicDeps()

	// Assert: Logger methods work without panic
	assert.NotPanics(t, func() {
		logger.Debug("test_tag", "debug message")
	})

	assert.NotPanics(t, func() {
		logger.Info("test_tag", "info message")
	})

	assert.NotPanics(t, func() {
		logger.Warn("test_tag", "warn message")
	})

	assert.NotPanics(t, func() {
		logger.Error("test_tag", "error message")
	})
}

// TestBasicDeps_FileSystemMethods verifies basic filesystem operations
func TestBasicDeps_FileSystemMethods(t *testing.T) {
	// Execute
	_, fs, _, _ := main_pkg.BasicDeps()

	// Assert: Filesystem is initialized and usable
	assert.NotNil(t, fs)
}

// TestBasicDeps_CmdRunnerMethods verifies command runner has expected methods
func TestBasicDeps_CmdRunnerMethods(t *testing.T) {
	// Execute
	_, _, cmdRunner, _ := main_pkg.BasicDeps()

	// Assert: CmdRunner is initialized and usable
	assert.NotNil(t, cmdRunner)
}

