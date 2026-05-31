package vm

import (
	"path/filepath"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	"bosh-libvirt-cpi/driver"
)

type Store struct {
	path   string
	runner driver.Runner
}

func NewStore(path string, runner driver.Runner) Store {
	return Store{path, runner}
}

func sanitizeKey(key string) error {
	if strings.Contains(key, "..") || strings.Contains(key, "/") {
		return bosherr.Errorf("invalid key '%s': must not contain '..' or '/'", key)
	}
	return nil
}

func (m Store) List() ([]string, error) {
	_, _, err := m.runner.Execute("mkdir", "-p", m.path)
	if err != nil {
		return nil, err
	}

	out, _, err := m.runner.Execute("ls", "-1", m.path)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(out, "\n")
	ids := []string{}
	for _, p := range parts {
		if p != "" {
			ids = append(ids, p)
		}
	}
	return ids, nil
}

func (m Store) Path(key string) string {
	return filepath.Join(m.path, key)
}

func (m Store) Put(key string, contents []byte) error {
	if err := sanitizeKey(key); err != nil {
		return err
	}

	_, _, err := m.runner.Execute("mkdir", "-p", m.path)
	if err != nil {
		return err
	}

	return m.runner.Put(filepath.Join(m.path, key), contents)
}

func (m Store) Get(key string) ([]byte, error) {
	if err := sanitizeKey(key); err != nil {
		return nil, err
	}

	return m.runner.Get(filepath.Join(m.path, key))
}

func (m Store) DeleteOne(key string) error {
	if err := sanitizeKey(key); err != nil {
		return err
	}

	_, _, err := m.runner.Execute("rm", "-rf", filepath.Join(m.path, key))
	return err
}

func (m Store) Delete() error {
	_, _, err := m.runner.Execute("rm", "-rf", m.path)
	return err
}
