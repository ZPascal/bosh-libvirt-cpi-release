package vm

import (
	"encoding/json"
	"strings"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	bdisk "bosh-libvirt-cpi/disk"
)

func (vm VMImpl) DiskIDs() ([]apiv1.DiskCID, error) {
	ids, err := diskAttachmentRecords{vm.store}.List()
	if err != nil {
		return nil, err
	}

	var persistentIDs []apiv1.DiskCID

	for _, id := range ids {
		rec, err := diskAttachmentRecords{vm.store}.Get(id)
		if err != nil {
			return nil, err
		} else if !rec.Ephemeral {
			persistentIDs = append(persistentIDs, id)
		}
	}

	return persistentIDs, nil
}

func (vm VMImpl) AttachDisk(disk bdisk.Disk) (apiv1.DiskHint, error) {
	return vm.attachDisk(disk, false)
}

func (vm VMImpl) AttachEphemeralDisk(disk bdisk.Disk) error {
	_, err := vm.attachDisk(disk, true)
	return err
}

func (vm VMImpl) attachDisk(disk bdisk.Disk, ephemeral bool) (apiv1.DiskHint, error) {
	hint := apiv1.NewDiskHintFromString(disk.ImagePath())

	rec := diskAttachmentRecord{
		ID:        disk.ID().AsString(),
		Ephemeral: ephemeral,
		Path:      disk.ImagePath(),
	}

	err := diskAttachmentRecords{vm.store}.Save(disk.ID(), rec)
	if err != nil {
		return apiv1.DiskHint{}, err
	}

	stemVer, err := vm.stemcellAPIVersion.Value()
	if err != nil {
		return apiv1.DiskHint{}, bosherr.WrapErrorf(err, "Obtaining stemcell API version")
	}

	// Update agent env for stemcells that do not support mount_diskV2
	if ephemeral || stemVer < 2 {
		vm.logger.Debug("VMImpl", "Reconfiguring agent")

		agentUpdateFunc := func(agentEnv apiv1.AgentEnv) {
			if ephemeral {
				agentEnv.AttachEphemeralDisk(hint)
			} else {
				agentEnv.AttachPersistentDisk(disk.ID(), hint)
			}
		}

		err = vm.reconfigureAgent(agentUpdateFunc)
		if err != nil {
			return apiv1.DiskHint{}, bosherr.WrapErrorf(err, "Reconfiguring agent after attaching disk")
		}
	} else {
		vm.logger.Debug("VMImpl", "Skipping agent reconfiguration")
	}

	return hint, nil
}

func (vm VMImpl) DetachDisk(disk bdisk.Disk) error {
	err := diskAttachmentRecords{vm.store}.Delete(disk.ID())
	if err != nil {
		return err
	}

	agentUpdateFunc := func(agentEnv apiv1.AgentEnv) {
		agentEnv.DetachPersistentDisk(disk.ID())
	}

	err = vm.reconfigureAgent(agentUpdateFunc)
	if err != nil {
		return bosherr.WrapErrorf(err, "Reconfiguring agent after detaching disk")
	}

	return nil
}

type diskAttachmentRecord struct {
	ID        string
	Ephemeral bool
	Path      string
}

type diskAttachmentRecords struct {
	store Store
}

const (
	diskAttachmentRecordsSuffix = "-disk-attachment.json"
)

func (r diskAttachmentRecords) List() ([]apiv1.DiskCID, error) {
	keys, err := r.store.List()
	if err != nil {
		return nil, bosherr.WrapError(err, "Listing disk attachments")
	}

	var ids []apiv1.DiskCID

	for _, key := range keys {
		if !strings.HasSuffix(key, diskAttachmentRecordsSuffix) {
			continue
		}

		ids = append(ids, apiv1.NewDiskCID(strings.TrimSuffix(key, diskAttachmentRecordsSuffix)))
	}

	return ids, nil
}

func (r diskAttachmentRecords) Get(cid apiv1.DiskCID) (diskAttachmentRecord, error) {
	var rec diskAttachmentRecord

	bytes, err := r.store.Get(cid.AsString() + diskAttachmentRecordsSuffix)
	if err != nil {
		return rec, bosherr.WrapError(err, "Getting disk attachment")
	}

	err = json.Unmarshal(bytes, &rec)
	if err != nil {
		return rec, bosherr.WrapError(err, "Deserializing disk attachment")
	}

	return rec, nil
}

func (r diskAttachmentRecords) Save(cid apiv1.DiskCID, rec diskAttachmentRecord) error {
	bytes, err := json.Marshal(rec)
	if err != nil {
		return bosherr.WrapError(err, "Serializing disk attachment")
	}

	err = r.store.Put(cid.AsString()+diskAttachmentRecordsSuffix, bytes)
	if err != nil {
		return bosherr.WrapError(err, "Saving disk attachment")
	}

	return nil
}

func (r diskAttachmentRecords) Delete(cid apiv1.DiskCID) error {
	return r.store.DeleteOne(cid.AsString() + diskAttachmentRecordsSuffix)
}
