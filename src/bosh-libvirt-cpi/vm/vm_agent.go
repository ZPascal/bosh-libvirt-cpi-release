package vm

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

func (vm VMImpl) ConfigureAgent(agentEnv apiv1.AgentEnv) error {
	_, err := vm.configureAgent(agentEnv)
	return err
}

func (vm VMImpl) configureAgent(agentEnv apiv1.AgentEnv) ([]byte, error) {
	bytes, err := agentEnv.AsBytes()
	if err != nil {
		return nil, bosherr.WrapError(err, "Marshalling agent env")
	}

	err = vm.store.Put("env.json", bytes)
	if err != nil {
		return nil, bosherr.WrapError(err, "Updating agent env")
	}

	return bytes, nil
}

func (vm VMImpl) reconfigureAgent(hotPlug bool, agentEnvFunc func(apiv1.AgentEnv)) error {
	prevContents, err := vm.store.Get("env.json")
	if err != nil {
		return bosherr.WrapError(err, "Fetching agent env")
	}

	agentEnv, err := apiv1.NewAgentEnvFactory().FromBytes(prevContents)
	if err != nil {
		return bosherr.WrapError(err, "Unmarshalling agent env")
	}

	agentEnvFunc(agentEnv)

	_, err = vm.configureAgent(agentEnv)
	if err != nil {
		return err
	}

	// For libvirt, we use cloud-init or config-drive
	// The agent env is written to env.json and picked up on VM restart
	// TODO: Implement config-drive mounting for libvirt if needed

	return nil
}
