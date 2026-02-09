package vm

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
)

type VMProps struct {
	Memory        int `json:"memory"`
	CPUs          int `json:"cpus"`
	EphemeralDisk int `json:"ephemeral_disk"`
}

func NewVMProps(props apiv1.VMCloudProps) (VMProps, error) {
	vmProps := VMProps{
		Memory:        512,  // Default 512 MB
		CPUs:          1,    // Default 1 CPU
		EphemeralDisk: 5000, // Default 5 GB
	}

	err := props.As(&vmProps)
	if err != nil {
		return VMProps{}, err
	}

	return vmProps, nil
}
