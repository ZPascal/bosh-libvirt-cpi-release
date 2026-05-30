package fakes

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	bstem "bosh-libvirt-cpi/stemcell"
	bvm "bosh-libvirt-cpi/vm"
)

type FakeCreator struct {
	CreateAgentIDArg    apiv1.AgentID
	CreateStemcellArg   bstem.Stemcell
	CreateCloudPropsArg apiv1.VMCloudProps
	CreateNetworksArg   apiv1.Networks
	CreateEnvArg        apiv1.VMEnv

	CreateResult bvm.VM
	CreateErr    error
}

var _ bvm.Creator = &FakeCreator{}

func (c *FakeCreator) Create(
	agentID apiv1.AgentID,
	stemcell bstem.Stemcell,
	cloudProps apiv1.VMCloudProps,
	networks apiv1.Networks,
	env apiv1.VMEnv,
) (bvm.VM, error) {
	c.CreateAgentIDArg = agentID
	c.CreateStemcellArg = stemcell
	c.CreateCloudPropsArg = cloudProps
	c.CreateNetworksArg = networks
	c.CreateEnvArg = env
	return c.CreateResult, c.CreateErr
}
