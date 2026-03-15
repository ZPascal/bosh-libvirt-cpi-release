package testhelpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelper provides common test utilities
type TestHelper struct {
	t *testing.T
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

// Assert returns assertion helper
func (h *TestHelper) Assert() *assert.Assertions {
	return assert.New(h.t)
}

// Require returns requirement helper
func (h *TestHelper) Require() *require.Assertions {
	return require.New(h.t)
}

// TempDir creates a temporary directory for testing
func (h *TestHelper) TempDir() string {
	return h.t.TempDir()
}

// Cleanup registers a cleanup function
func (h *TestHelper) Cleanup(fn func()) {
	h.t.Cleanup(fn)
}
