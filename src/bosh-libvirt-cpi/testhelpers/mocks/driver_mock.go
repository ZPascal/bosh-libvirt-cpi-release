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

// SimpleMockDriver is a simple mock implementation with callback functions
type SimpleMockDriver struct {
	ExecuteFunc        func(args ...string) (string, error)
	ExecuteComplexFunc func(args []string, opts driver.ExecuteOpts) (string, error)
	IsMissingVMErrFunc func(output string) bool
}

func NewSimpleMockDriver() *SimpleMockDriver {
	return &SimpleMockDriver{
		ExecuteFunc: func(args ...string) (string, error) { return "", nil },
		ExecuteComplexFunc: func(args []string, opts driver.ExecuteOpts) (string, error) { return "", nil },
		IsMissingVMErrFunc: func(output string) bool { return false },
	}
}

func (m *SimpleMockDriver) Execute(args ...string) (string, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(args...)
	}
	return "", nil
}

func (m *SimpleMockDriver) ExecuteComplex(args []string, opts driver.ExecuteOpts) (string, error) {
	if m.ExecuteComplexFunc != nil {
		return m.ExecuteComplexFunc(args, opts)
	}
	return "", nil
}

func (m *SimpleMockDriver) IsMissingVMErr(output string) bool {
	if m.IsMissingVMErrFunc != nil {
		return m.IsMissingVMErrFunc(output)
	}
	return false
}

// SimpleMockRunner is a simple mock implementation with callback functions
type SimpleMockRunner struct {
	ExecuteFunc func(path string, args ...string) (string, int, error)
	UploadFunc  func(srcDir, dstDir string) error
	PutFunc     func(path string, contents []byte) error
	GetFunc     func(path string) ([]byte, error)
}

func NewSimpleMockRunner() *SimpleMockRunner {
	return &SimpleMockRunner{
		ExecuteFunc: func(path string, args ...string) (string, int, error) { return "", 0, nil },
		UploadFunc:  func(srcDir, dstDir string) error { return nil },
		PutFunc:     func(path string, contents []byte) error { return nil },
		GetFunc:     func(path string) ([]byte, error) { return nil, nil },
	}
}

func (m *SimpleMockRunner) Execute(path string, args ...string) (string, int, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(path, args...)
	}
	return "", 0, nil
}

func (m *SimpleMockRunner) Upload(srcDir, dstDir string) error {
	if m.UploadFunc != nil {
		return m.UploadFunc(srcDir, dstDir)
	}
	return nil
}

func (m *SimpleMockRunner) Put(path string, contents []byte) error {
	if m.PutFunc != nil {
		return m.PutFunc(path, contents)
	}
	return nil
}

func (m *SimpleMockRunner) Get(path string) ([]byte, error) {
	if m.GetFunc != nil {
		return m.GetFunc(path)
	}
	return nil, nil
}
