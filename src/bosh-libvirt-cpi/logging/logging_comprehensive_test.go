package logging

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoggerCreation tests logger initialization
func TestLoggerCreation(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		{
			name: "creates logger successfully",
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotNil(t, logger)
				assert.Implements(t, (*Logger)(nil), logger)
			},
		},
		{
			name: "logger implements interface",
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				var _ Logger = logger
				assert.NotNil(t, logger)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.testFunc)
	}
}

// TestDebugLogging tests Debug method
func TestDebugLogging(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		args     []interface{}
		testFunc func(*testing.T)
	}{
		{
			name: "logs debug message without args",
			msg:  "Debug message",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Debug("Test debug")
				})
			},
		},
		{
			name: "logs debug message with single arg",
			msg:  "Debug %v",
			args: []interface{}{"value"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Debug("Debug %v", "test")
				})
			},
		},
		{
			name: "logs debug message with multiple args",
			msg:  "Debug %v %v %v",
			args: []interface{}{"a", "b", "c"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Debug("Debug %v %v %v", "a", "b", "c")
				})
			},
		},
		{
			name: "logs debug with empty message",
			msg:  "",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Debug("")
				})
			},
		},
		{
			name: "logs debug with special characters",
			msg:  "Debug with special: !@#$%",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Debug("Debug with special: !@#$%")
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.testFunc)
	}
}

// TestInfoLogging tests Info method
func TestInfoLogging(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		args     []interface{}
		testFunc func(*testing.T)
	}{
		{
			name: "logs info message without args",
			msg:  "Info message",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Info("Test info")
				})
			},
		},
		{
			name: "logs info message with single arg",
			msg:  "Info %v",
			args: []interface{}{"value"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Info("Info %v", "test")
				})
			},
		},
		{
			name: "logs info message with multiple args",
			msg:  "Info %v %v %v",
			args: []interface{}{"a", "b", "c"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Info("Info %v %v %v", "a", "b", "c")
				})
			},
		},
		{
			name: "logs info with long message",
			msg:  "Info with a very long message that spans multiple lines and contains lots of information about the system",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Info("Info with a very long message that spans multiple lines and contains lots of information about the system")
				})
			},
		},
		{
			name: "logs info with numeric args",
			msg:  "Info: count=%d, value=%f",
			args: []interface{}{42, 3.14},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Info("Info: count=%d, value=%f", 42, 3.14)
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.testFunc)
	}
}

// TestWarnLogging tests Warn method
func TestWarnLogging(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		args     []interface{}
		testFunc func(*testing.T)
	}{
		{
			name: "logs warn message without args",
			msg:  "Warning message",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Warn("Test warning")
				})
			},
		},
		{
			name: "logs warn message with single arg",
			msg:  "Warn %v",
			args: []interface{}{"value"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Warn("Warn %v", "test")
				})
			},
		},
		{
			name: "logs warn message with multiple args",
			msg:  "Warn %v %v %v",
			args: []interface{}{"a", "b", "c"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Warn("Warn %v %v %v", "a", "b", "c")
				})
			},
		},
		{
			name: "logs warn about resource exhaustion",
			msg:  "Warn: resource=%v, remaining=%d",
			args: []interface{}{"memory", 100},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Warn("Warn: resource=%v, remaining=%d", "memory", 100)
				})
			},
		},
		{
			name: "logs warn with empty args",
			msg:  "Warn %v %v",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Warn("Warn message")
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.testFunc)
	}
}

// TestErrorLogging tests Error method
func TestErrorLogging(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		args     []interface{}
		testFunc func(*testing.T)
	}{
		{
			name: "logs error message without args",
			msg:  "Error message",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Error("Test error")
				})
			},
		},
		{
			name: "logs error message with single arg",
			msg:  "Error %v",
			args: []interface{}{"value"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Error("Error %v", "test")
				})
			},
		},
		{
			name: "logs error message with multiple args",
			msg:  "Error %v %v %v",
			args: []interface{}{"a", "b", "c"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Error("Error %v %v %v", "a", "b", "c")
				})
			},
		},
		{
			name: "logs error with error description",
			msg:  "Error: operation=%v, reason=%v",
			args: []interface{}{"create", "permission denied"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Error("Error: operation=%v, reason=%v", "create", "permission denied")
				})
			},
		},
		{
			name: "logs error with error code",
			msg:  "Error code: %d - %v",
			args: []interface{}{500, "internal server error"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Error("Error code: %d - %v", 500, "internal server error")
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.testFunc)
	}
}

// TestFatalLogging tests Fatal method
func TestFatalLogging(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		args     []interface{}
		testFunc func(*testing.T)
	}{
		{
			name: "handles fatal message without args",
			msg:  "Fatal error",
			args: []interface{}{},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Fatal("Fatal error")
				})
			},
		},
		{
			name: "handles fatal message with single arg",
			msg:  "Fatal %v",
			args: []interface{}{"value"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Fatal("Fatal %v", "test")
				})
			},
		},
		{
			name: "handles fatal message with multiple args",
			msg:  "Fatal %v %v %v",
			args: []interface{}{"a", "b", "c"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Fatal("Fatal %v %v %v", "a", "b", "c")
				})
			},
		},
		{
			name: "handles fatal shutdown message",
			msg:  "Fatal: shutdown=%v, reason=%v",
			args: []interface{}{"true", "critical error"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Fatal("Fatal: shutdown=%v, reason=%v", "true", "critical error")
				})
			},
		},
		{
			name: "handles fatal with panic info",
			msg:  "Fatal panic in %v: %v",
			args: []interface{}{"goroutine", "nil pointer"},
			testFunc: func(t *testing.T) {
				logger := NewLogger()
				assert.NotPanics(t, func() {
					logger.Fatal("Fatal panic in %v: %v", "goroutine", "nil pointer")
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.testFunc)
	}
}

// TestLoggerMultipleCalls tests multiple logger calls in sequence
func TestLoggerMultipleCalls(t *testing.T) {
	logger := NewLogger()
	require.NotNil(t, logger)

	assert.NotPanics(t, func() {
		logger.Debug("Debug message")
		logger.Info("Info message")
		logger.Warn("Warning message")
		logger.Error("Error message")
		logger.Fatal("Fatal message")
	})
}

// TestLoggerWithDifferentArgTypes tests logging with various argument types
func TestLoggerWithDifferentArgTypes(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name string
		call func()
	}{
		{
			name: "with string arg",
			call: func() {
				logger.Info("String: %s", "value")
			},
		},
		{
			name: "with int arg",
			call: func() {
				logger.Info("Int: %d", 42)
			},
		},
		{
			name: "with float arg",
			call: func() {
				logger.Info("Float: %f", 3.14)
			},
		},
		{
			name: "with bool arg",
			call: func() {
				logger.Info("Bool: %v", true)
			},
		},
		{
			name: "with struct arg",
			call: func() {
				type testStruct struct {
					name  string
					value int
				}
				logger.Info("Struct: %v", testStruct{name: "test", value: 123})
			},
		},
		{
			name: "with nil arg",
			call: func() {
				logger.Info("Nil: %v", nil)
			},
		},
		{
			name: "with slice arg",
			call: func() {
				logger.Info("Slice: %v", []int{1, 2, 3})
			},
		},
		{
			name: "with map arg",
			call: func() {
				logger.Info("Map: %v", map[string]int{"a": 1, "b": 2})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NotPanics(t, test.call)
		})
	}
}

// TestLoggerConcurrentAccess tests concurrent logging calls
func TestLoggerConcurrentAccess(t *testing.T) {
	logger := NewLogger()
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(id int) {
			logger.Info("Goroutine %d", id)
			logger.Debug("Debug from goroutine %d", id)
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

// TestLoggerMessageFormatting tests message formatting scenarios
func TestLoggerMessageFormatting(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name string
		call func()
	}{
		{
			name: "format with percent sign",
			call: func() {
				logger.Info("Progress: 100%%")
			},
		},
		{
			name: "format with escaped newline",
			call: func() {
				logger.Info("Line 1\\nLine 2")
			},
		},
		{
			name: "format with tabs",
			call: func() {
				logger.Info("Key:\tValue")
			},
		},
		{
			name: "format with unicode",
			call: func() {
				logger.Info("Unicode: 你好世界 🚀")
			},
		},
		{
			name: "format with very long message",
			call: func() {
				longMsg := "This is a very long message that contains a lot of information about system operations and various parameters that need to be logged and tracked for debugging and monitoring purposes. It should handle long messages gracefully without truncation or errors."
				logger.Info(longMsg)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NotPanics(t, test.call)
		})
	}
}

// TestLoggerNilPointerHandling verifies behavior with nil references
func TestLoggerNilPointerHandling(t *testing.T) {
	logger := NewLogger()
	require.NotNil(t, logger)

	// Test with nil values in args
	assert.NotPanics(t, func() {
		var nilVal *string
		logger.Info("Nil pointer: %v", nilVal)
	})
}

// TestLoggerVariadicHandling tests variadic argument handling
func TestLoggerVariadicHandling(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name string
		call func()
	}{
		{
			name: "no args",
			call: func() {
				logger.Info("Message")
			},
		},
		{
			name: "one arg",
			call: func() {
				logger.Info("Message %v", 1)
			},
		},
		{
			name: "two args",
			call: func() {
				logger.Info("Message %v %v", 1, 2)
			},
		},
		{
			name: "three args",
			call: func() {
				logger.Info("Message %v %v %v", 1, 2, 3)
			},
		},
		{
			name: "five args",
			call: func() {
				logger.Info("Message %v %v %v %v %v", 1, 2, 3, 4, 5)
			},
		},
		{
			name: "ten args",
			call: func() {
				logger.Info("Message %v %v %v %v %v %v %v %v %v %v",
					1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NotPanics(t, test.call)
		})
	}
}

// TestLoggerFormatMismatch tests mismatched format and args
func TestLoggerFormatMismatch(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name string
		call func()
	}{
		{
			name: "format string with no args",
			call: func() {
				logger.Info("Message with %v placeholder")
			},
		},
		{
			name: "more args than format specifiers",
			call: func() {
				logger.Info("Message %v", 1, 2, 3)
			},
		},
		{
			name: "format specifier mismatch",
			call: func() {
				logger.Info("Int: %d, String: %s", "wrong")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NotPanics(t, test.call)
		})
	}
}

// TestLoggerBoundaryConditions tests edge cases and boundary conditions
func TestLoggerBoundaryConditions(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name string
		call func()
	}{
		{
			name: "extremely long message",
			call: func() {
				longMsg := strings.Repeat("x", 10000)
				logger.Info(longMsg)
			},
		},
		{
			name: "message with many newlines",
			call: func() {
				logger.Info("Line1\nLine2\nLine3\nLine4\nLine5")
			},
		},
		{
			name: "message with control characters",
			call: func() {
				logger.Info("Control: \t\n\r")
			},
		},
		{
			name: "message with quotes",
			call: func() {
				logger.Info("Quote: 'single' and \"double\"")
			},
		},
		{
			name: "message with backslashes",
			call: func() {
				logger.Info("Path: C:\\Windows\\System32")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NotPanics(t, test.call)
		})
	}
}

// TestLoggingLevels tests all log level methods
func TestLoggingLevels(t *testing.T) {
	logger := NewLogger()
	require.NotNil(t, logger)

	logLevels := []struct {
		name string
		call func(Logger, string)
	}{
		{name: "Debug", call: func(l Logger, m string) { l.Debug(m) }},
		{name: "Info", call: func(l Logger, m string) { l.Info(m) }},
		{name: "Warn", call: func(l Logger, m string) { l.Warn(m) }},
		{name: "Error", call: func(l Logger, m string) { l.Error(m) }},
		{name: "Fatal", call: func(l Logger, m string) { l.Fatal(m) }},
	}

	for _, level := range logLevels {
		t.Run(level.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				level.call(logger, "Test message for "+level.name)
			})
		})
	}
}

// TestLoggerBehaviorConsistency verifies consistent behavior
func TestLoggerBehaviorConsistency(t *testing.T) {
	logger1 := NewLogger()
	logger2 := NewLogger()

	// Both loggers should handle the same message
	msg := "Consistency test message"

	assert.NotPanics(t, func() {
		logger1.Info(msg)
		logger2.Info(msg)
	})
}

