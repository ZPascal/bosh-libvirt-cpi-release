package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger()
	assert.NotNil(t, logger)
	assert.Implements(t, (*Logger)(nil), logger)
}

func TestSimpleLoggerDebug(t *testing.T) {
	logger := NewLogger()
	// Should not panic or error
	logger.Debug("Test debug message")
	logger.Debug("Test with args: %s", "value")
	logger.Debug("Multiple args", "arg1", "arg2", "arg3")
}

func TestSimpleLoggerInfo(t *testing.T) {
	logger := NewLogger()
	// Should not panic or error
	logger.Info("Test info message")
	logger.Info("Test with args: %s", "value")
	logger.Info("Multiple args", "arg1", "arg2", "arg3")
}

func TestSimpleLoggerWarn(t *testing.T) {
	logger := NewLogger()
	// Should not panic or error
	logger.Warn("Test warn message")
	logger.Warn("Test with args: %s", "value")
	logger.Warn("Multiple args", "arg1", "arg2", "arg3")
}

func TestSimpleLoggerError(t *testing.T) {
	logger := NewLogger()
	// Should not panic or error
	logger.Error("Test error message")
	logger.Error("Test with args: %s", "value")
	logger.Error("Multiple args", "arg1", "arg2", "arg3")
}

func TestSimpleLoggerFatal(t *testing.T) {
	logger := NewLogger()
	// Note: Fatal is typically used to exit the process, but the simpleLogger
	// implementation doesn't actually exit, so we can test it here
	logger.Fatal("Test fatal message")
	logger.Fatal("Test with args: %s", "value")
	logger.Fatal("Multiple args", "arg1", "arg2", "arg3")
}

func TestLoggerInterface(t *testing.T) {
	var logger Logger = NewLogger()

	// Verify all methods exist and are callable
	assert.NotNil(t, logger)

	// Call all interface methods
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.Fatal("fatal")
}

func TestMultipleLoggersIndependent(t *testing.T) {
	logger1 := NewLogger()
	logger2 := NewLogger()

	// Both should be different instances
	assert.NotSame(t, logger1, logger2)

	// Both should work independently
	logger1.Debug("logger1 debug")
	logger2.Info("logger2 info")
}

func TestEmptyMessage(t *testing.T) {
	logger := NewLogger()
	logger.Debug("")
	logger.Info("")
	logger.Warn("")
	logger.Error("")
	logger.Fatal("")
}

func TestNoArgs(t *testing.T) {
	logger := NewLogger()
	logger.Debug("message without args")
	logger.Info("message without args")
	logger.Warn("message without args")
	logger.Error("message without args")
	logger.Fatal("message without args")
}
