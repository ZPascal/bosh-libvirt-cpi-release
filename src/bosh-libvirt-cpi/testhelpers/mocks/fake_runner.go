package mocks

import (
	"os"
)

// FakeRunner is a mock implementation of driver.Runner for testing
type FakeRunner struct {
	lastCommand  string
	lastArgs     []string
	ExecuteFunc  func(path string, args ...string) (string, int, error)
	UploadFunc   func(srcDir, dstDir string) error
	PutFunc      func(path string, contents []byte) error
	GetFunc      func(path string) ([]byte, error)
}

// NewFakeRunner creates a new fake runner
func NewFakeRunner() *FakeRunner {
	return &FakeRunner{}
}

// HomeDir returns a fake home directory
func (r *FakeRunner) HomeDir() (string, error) {
	return os.ExpandEnv("$HOME"), nil
}

// Execute records the command and returns success
func (r *FakeRunner) Execute(path string, args ...string) (string, int, error) {
	r.lastCommand = path
	r.lastArgs = args
	if r.ExecuteFunc != nil {
		return r.ExecuteFunc(path, args...)
	}
	return "success", 0, nil
}

// Exists checks if a file exists
func (r *FakeRunner) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Upload copies a file
func (r *FakeRunner) Upload(srcPath, dstPath string) error {
	if r.UploadFunc != nil {
		return r.UploadFunc(srcPath, dstPath)
	}
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, content, 0644)
}

// Put writes contents to a file
func (r *FakeRunner) Put(path string, contents []byte) error {
	if r.PutFunc != nil {
		return r.PutFunc(path, contents)
	}
	return os.WriteFile(path, contents, 0644)
}

// Get reads contents from a file
func (r *FakeRunner) Get(path string) ([]byte, error) {
	if r.GetFunc != nil {
		return r.GetFunc(path)
	}
	return os.ReadFile(path)
}

// GetLastCommand returns the last command executed
func (r *FakeRunner) GetLastCommand() string {
	return r.lastCommand
}

// GetLastArgs returns the last arguments
func (r *FakeRunner) GetLastArgs() []string {
	return r.lastArgs
}
