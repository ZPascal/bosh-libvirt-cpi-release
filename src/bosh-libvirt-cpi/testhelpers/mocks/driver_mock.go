package mocks

import (
	"bosh-libvirt-cpi/driver"

	"github.com/stretchr/testify/mock"
)

// MockDriver is a mock implementation of driver.Driver
type MockDriver struct {
	mock.Mock
}

func NewMockDriver() *MockDriver {
	return &MockDriver{}
}

func (m *MockDriver) Execute(args ...string) (string, error) {
	argsCalled := m.Called(args)
	return argsCalled.String(0), argsCalled.Error(1)
}

func (m *MockDriver) ExecuteComplex(args []string, opts driver.ExecuteOpts) (string, error) {
	argsCalled := m.Called(args, opts)
	return argsCalled.String(0), argsCalled.Error(1)
}

func (m *MockDriver) IsMissingVMErr(output string) bool {
	args := m.Called(output)
	return args.Bool(0)
}

// MockRunner is a mock implementation of driver.Runner
type MockRunner struct {
	mock.Mock
}

func NewMockRunner() *MockRunner {
	return &MockRunner{}
}

func (m *MockRunner) Execute(path string, args ...string) (string, int, error) {
	argsCalled := m.Called(path, args)
	return argsCalled.String(0), argsCalled.Int(1), argsCalled.Error(2)
}

func (m *MockRunner) Upload(srcDir, dstDir string) error {
	args := m.Called(srcDir, dstDir)
	return args.Error(0)
}

func (m *MockRunner) Put(path string, contents []byte) error {
	args := m.Called(path, contents)
	return args.Error(0)
}

func (m *MockRunner) Get(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

// MockRetrier is a mock implementation of driver.Retrier
type MockRetrier struct{}

func (m *MockRetrier) Retry(fn func() error) error {
	return fn()
}
func (m *MockRetrier) RetryComplex(fn func() error, attempts uint, sleep uint) error {
	return fn()
}
