package stemcell

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
)

// Importer imports a stemcell image from a local path.
type Importer interface {
	// ImportFromPath imports the stemcell image and records its guest
	// architecture (see NormalizeArch).
	ImportFromPath(imagePath, arch string) (Stemcell, error)
}

var _ Importer = Factory{}

// Finder looks up a stemcell by CID.
type Finder interface {
	Find(apiv1.StemcellCID) (Stemcell, error)
}

var _ Finder = Factory{}

// Stemcell represents an imported stemcell managed by the CPI.
type Stemcell interface {
	ID() apiv1.StemcellCID

	ImagePath() string
	// Architecture is the guest architecture ("x86_64" or "aarch64").
	Architecture() string

	Exists() (bool, error)
	Delete() error
}

var _ Stemcell = StemcellImpl{}
