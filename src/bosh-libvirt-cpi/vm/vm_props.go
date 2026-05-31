package vm

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
)

type VMProps struct {
	Memory        int
	CPUs          int
	EphemeralDisk int `json:"ephemeral_disk"`
}

func NewVMProps(props apiv1.VMCloudProps) (VMProps, error) {
	vmProps := VMProps{
		Memory:        512,
		CPUs:          1,
		EphemeralDisk: 5000,
	}

	err := props.As(&vmProps)
	if err != nil {
		return VMProps{}, err
	}

	return vmProps, nil
}
