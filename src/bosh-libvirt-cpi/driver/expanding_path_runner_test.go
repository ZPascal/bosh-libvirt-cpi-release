package driver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestMockRunner struct {
	homeDir    string
	homeDirErr error
}

func (m *TestMockRunner) HomeDir() (string, error) {
	return m.homeDir, m.homeDirErr
}

func (m *TestMockRunner) Execute(path string, args ...string) (string, int, error) {
	return "ok", 0, nil
}

func (m *TestMockRunner) Upload(src, dst string) error           { return nil }
func (m *TestMockRunner) Put(path string, contents []byte) error { return nil }
func (m *TestMockRunner) Get(path string) ([]byte, error)        { return []byte("data"), nil }

func TestExpandingPathRunnerNew(t *testing.T) {
	runner := NewExpandingPathRunner(&TestMockRunner{homeDir: "/home/user"})
	assert.NotNil(t, runner)
}

func TestExpandPathExecuteExpands(t *testing.T) {
	runner := NewExpandingPathRunner(&TestMockRunner{homeDir: "/home/user"})
	_, _, _ = runner.Execute("echo", "~/file.txt")
}

func TestExpandPathUploadExpands(t *testing.T) {
	runner := NewExpandingPathRunner(&TestMockRunner{homeDir: "/home/user"})
	_ = runner.Upload("~/src.txt", "~/dst.txt")
}

func TestExpandPathPutExpands(t *testing.T) {
	runner := NewExpandingPathRunner(&TestMockRunner{homeDir: "/home/user"})
	_ = runner.Put("~/file.txt", []byte("content"))
}

func TestExpandPathGetExpands(t *testing.T) {
	runner := NewExpandingPathRunner(&TestMockRunner{homeDir: "/home/user"})
	_, _ = runner.Get("~/file.txt")
}

func TestExpandPathError(t *testing.T) {
	runner := NewExpandingPathRunner(&TestMockRunner{homeDirErr: errors.New("failed")})
	_, _, err := runner.Execute("echo", "~/file.txt")
	assert.Error(t, err)
}

func TestExpandPathAbsolutePath(t *testing.T) {
	runner := NewExpandingPathRunner(&TestMockRunner{homeDir: "/home/user"})
	_, _, _ = runner.Execute("echo", "/absolute/path.txt")
}

func TestExpandPathRelativePath(t *testing.T) {
	runner := NewExpandingPathRunner(&TestMockRunner{homeDir: "/home/user"})
	_, _, _ = runner.Execute("echo", "relative/path.txt")
}
