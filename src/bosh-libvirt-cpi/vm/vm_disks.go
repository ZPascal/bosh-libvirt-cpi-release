package vm

import (
	"encoding/json"
	"errors"
	"strings"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	bdisk "bosh-libvirt-cpi/disk"
	"bosh-libvirt-cpi/driver"
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

// persistentDiskTargets are the guest device names for hot-plugged persistent
// disks; vda (root) and vdb (ephemeral) are defined with the domain.
var persistentDiskTargets = func() []string {
	var targets []string
	for c := 'c'; c <= 'z'; c++ {
		targets = append(targets, "vd"+string(c))
	}
	return targets
}()

// virtioSerialMaxLen is the longest serial virtio-blk exposes to the guest.
const virtioSerialMaxLen = 20

func diskSerial(cid apiv1.DiskCID) string {
	id := cid.AsString()
	if len(id) > virtioSerialMaxLen {
		return id[:virtioSerialMaxLen]
	}
	return id
}

func (vm VMImpl) attachDisk(disk bdisk.Disk, ephemeral bool) (apiv1.DiskHint, error) {
	if !ephemeral {
		hint, handled, err := vm.hotplugPersistentDisk(disk)
		if handled || err != nil {
			return hint, err
		}
	}

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

// hotplugPersistentDisk attaches the disk image to the domain as a virtio disk
// and returns a hint the agent resolves via /dev/disk/by-id/*<serial>, falling
// back to /dev/<target>. handled is false when the backend cannot hot-plug, in
// which case the caller keeps the legacy image-path hint.
func (vm VMImpl) hotplugPersistentDisk(disk bdisk.Disk) (apiv1.DiskHint, bool, error) {
	records := diskAttachmentRecords{vm.store}
	serial := diskSerial(disk.ID())

	// Backends that can't hot-plug keep the legacy path; check before touching
	// any attachment records.
	if _, err := vm.domBuilder.BuildPersistentDiskXML(disk.ImagePath(), "qcow2", persistentDiskTargets[0], serial); errors.Is(err, driver.ErrPersistentDiskUnsupported) {
		return apiv1.DiskHint{}, false, nil
	}

	// Already hot-plugged (e.g. bosh cck "reattach disk"): attaching the same
	// image again would fail on QEMU's image lock, so return the same hint.
	if rec, err := records.Get(disk.ID()); err == nil && rec.Target != "" {
		return persistentDiskHint(rec.Serial, rec.Target), true, nil
	}

	format, err := disk.Format()
	if err != nil {
		return apiv1.DiskHint{}, true, err
	}

	target, err := vm.freePersistentDiskTarget()
	if err != nil {
		return apiv1.DiskHint{}, true, err
	}

	xml, err := vm.domBuilder.BuildPersistentDiskXML(disk.ImagePath(), format, target, serial)
	if err != nil {
		return apiv1.DiskHint{}, true, bosherr.WrapError(err, "Building persistent disk device")
	}

	// Save first so the target is reserved; drop the record if the attach fails.
	rec := diskAttachmentRecord{
		ID:     disk.ID().AsString(),
		Path:   disk.ImagePath(),
		Format: format,
		Target: target,
		Serial: serial,
	}
	if err := records.Save(disk.ID(), rec); err != nil {
		return apiv1.DiskHint{}, true, err
	}

	if err := vm.driver.AttachDevice(vm.cid.AsString(), xml); err != nil {
		_ = records.Delete(disk.ID())
		return apiv1.DiskHint{}, true, bosherr.WrapErrorf(err, "Attaching persistent disk '%s' as %s", disk.ID().AsString(), target)
	}

	return persistentDiskHint(serial, target), true, nil
}

func persistentDiskHint(serial, target string) apiv1.DiskHint {
	return apiv1.NewDiskHintFromMap(map[string]interface{}{
		"id":        serial,
		"volume_id": serial,
		"path":      "/dev/" + target,
	})
}

func (vm VMImpl) freePersistentDiskTarget() (string, error) {
	records := diskAttachmentRecords{vm.store}
	ids, err := records.List()
	if err != nil {
		return "", err
	}

	used := map[string]bool{}
	for _, id := range ids {
		rec, err := records.Get(id)
		if err != nil {
			return "", err
		}
		used[rec.Target] = true
	}

	for _, t := range persistentDiskTargets {
		if !used[t] {
			return t, nil
		}
	}
	return "", bosherr.Errorf("No free device target for another persistent disk (all of vdc-vdz in use)")
}

func (vm VMImpl) DetachDisk(disk bdisk.Disk) error {
	records := diskAttachmentRecords{vm.store}

	// Hot-plugged disks have a Target in their record; unplug them first. A
	// domain that is already gone has nothing left to unplug. A record that
	// exists but can't be read must not be dropped: the device would stay
	// attached with nothing left pointing at it.
	attached, err := records.List()
	if err != nil {
		return err
	}
	var rec diskAttachmentRecord
	for _, id := range attached {
		if id == disk.ID() {
			if rec, err = records.Get(disk.ID()); err != nil {
				return err
			}
			break
		}
	}
	if rec.Target != "" {
		xml, err := vm.domBuilder.BuildPersistentDiskXML(rec.Path, rec.Format, rec.Target, rec.Serial)
		if err != nil {
			return bosherr.WrapError(err, "Building persistent disk device")
		}
		if err := vm.driver.DetachDevice(vm.cid.AsString(), xml); err != nil && !vm.driver.IsMissingDomainErr(err) {
			return bosherr.WrapErrorf(err, "Detaching persistent disk '%s'", disk.ID().AsString())
		}
	}

	if err := records.Delete(disk.ID()); err != nil {
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
	// Format, Target and Serial are set for hot-plugged (QEMU) persistent disks.
	Format string `json:",omitempty"`
	Target string `json:",omitempty"`
	Serial string `json:",omitempty"`
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
