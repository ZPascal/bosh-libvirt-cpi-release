package cpi

import (
	"github.com/cloudfoundry/bosh-cpi-go/apiv1"
)

// Snapshots implements the CPI snapshot methods (no-ops: libvirt snapshots are not supported).
type Snapshots struct{}

// NewSnapshots returns a Snapshots.
func NewSnapshots() Snapshots {
	return Snapshots{}
}

func (s Snapshots) SnapshotDisk(cid apiv1.DiskCID, meta apiv1.DiskMeta) (apiv1.SnapshotCID, error) {
	return apiv1.SnapshotCID{}, nil
}

func (s Snapshots) DeleteSnapshot(cid apiv1.SnapshotCID) error {
	return nil
}
