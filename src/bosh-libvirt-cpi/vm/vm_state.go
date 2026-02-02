package vm

import (
	"strings"
)

func (vm VMImpl) Exists() (bool, error) {
	output, err := vm.driver.Execute("dominfo", vm.cid.AsString())
	if err != nil {
		if vm.driver.IsMissingVMErr(output) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (vm VMImpl) Start(gui bool) error {
	// For libvirt, we start with virsh start
	// GUI parameter is not relevant for libvirt (handled by graphics device in domain XML)
	_, err := vm.driver.Execute("start", vm.cid.AsString())
	if err != nil {
		// Check if already running
		if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "is already running") {
			return nil
		}
		return err
	}

	return nil
}

func (vm VMImpl) Reboot() error {
	_, err := vm.driver.Execute("reboot", vm.cid.AsString())
	return err
}

func (vm VMImpl) HaltIfRunning() error {
	running, err := vm.IsRunning()
	if err != nil {
		return err
	}

	if running {
		// Use destroy for immediate shutdown (like VirtualBox poweroff)
		_, err = vm.driver.Execute("destroy", vm.cid.AsString())
	}

	return err
}

func (vm VMImpl) IsRunning() (bool, error) {
	state, err := vm.State()
	if err != nil {
		return false, err
	}

	return state == "running", nil
}

func (vm VMImpl) State() (string, error) {
	output, err := vm.driver.Execute("domstate", vm.cid.AsString())
	if err != nil {
		if vm.driver.IsMissingVMErr(output) {
			return "missing", nil
		}
		return "", err
	}

	// virsh domstate returns: running, shut off, paused, etc.
	state := strings.TrimSpace(strings.ToLower(output))

	// Normalize state names to match expected values
	switch state {
	case "shut off", "shutoff":
		return "poweroff", nil
	case "running":
		return "running", nil
	case "paused":
		return "paused", nil
	case "crashed":
		return "aborted", nil
	default:
		return state, nil
	}
}
