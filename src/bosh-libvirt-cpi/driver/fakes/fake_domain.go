package fakes

import "bosh-libvirt-cpi/driver"

type FakeDomain struct {
	GetNameResult string
	GetNameErr    error

	GetStateState  int
	GetStateReason int
	GetStateErr    error

	IsActiveResult bool
	IsActiveErr    error
}

var _ driver.Domain = &FakeDomain{}

func (d *FakeDomain) GetName() (string, error) {
	return d.GetNameResult, d.GetNameErr
}

func (d *FakeDomain) GetState() (int, int, error) {
	return d.GetStateState, d.GetStateReason, d.GetStateErr
}

func (d *FakeDomain) IsActive() (bool, error) {
	return d.IsActiveResult, d.IsActiveErr
}

func (d *FakeDomain) Free() error { return nil }
