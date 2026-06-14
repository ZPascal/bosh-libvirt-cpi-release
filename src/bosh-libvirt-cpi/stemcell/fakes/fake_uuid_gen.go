package fakes

import boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

type FakeUUIDGen struct {
	GeneratedUUID string
	GenerateErr   error
}

var _ boshuuid.Generator = &FakeUUIDGen{}

func (g *FakeUUIDGen) Generate() (string, error) {
	return g.GeneratedUUID, g.GenerateErr
}
