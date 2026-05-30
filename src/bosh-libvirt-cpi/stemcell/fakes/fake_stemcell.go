package fakes

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	bstem "bosh-libvirt-cpi/stemcell"
)

type FakeStemcell struct {
	cid apiv1.StemcellCID

	ImagePathResult string

	ExistsResult bool
	ExistsErr    error
	DeleteErr    error
}

var _ bstem.Stemcell = &FakeStemcell{}

func NewFakeStemcell(cid string) *FakeStemcell {
	return &FakeStemcell{cid: apiv1.NewStemcellCID(cid)}
}

func (s *FakeStemcell) ID() apiv1.StemcellCID { return s.cid }
func (s *FakeStemcell) ImagePath() string     { return s.ImagePathResult }
func (s *FakeStemcell) Exists() (bool, error) { return s.ExistsResult, s.ExistsErr }
func (s *FakeStemcell) Delete() error         { return s.DeleteErr }
