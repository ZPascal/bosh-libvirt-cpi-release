package stemcell

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strings"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/qemu"
	bpds "bosh-libvirt-cpi/vm/portdevices"
)

type FactoryOpts struct {
	DirPath           string
	StorageController string // todo expose per stemcell
}

type Factory struct {
	opts FactoryOpts

	driver  driver.Driver
	runner  driver.Runner
	retrier driver.Retrier

	fs         boshsys.FileSystem
	uuidGen    boshuuid.Generator
	compressor boshcmd.Compressor

	logTag string
	logger boshlog.Logger
}

func NewFactory(
	opts FactoryOpts,
	driver driver.Driver,
	runner driver.Runner,
	retrier driver.Retrier,
	fs boshsys.FileSystem,
	uuidGen boshuuid.Generator,
	compressor boshcmd.Compressor,
	logger boshlog.Logger,
) Factory {
	return Factory{
		opts: opts,

		driver:  driver,
		runner:  runner,
		retrier: retrier,

		fs:         fs,
		uuidGen:    uuidGen,
		compressor: compressor,

		logTag: "stemcell.Factory",
		logger: logger,
	}
}

func (f Factory) ImportFromPath(imagePath string) (Stemcell, error) {
	id, err := f.uuidGen.Generate()
	if err != nil {
		return nil, bosherr.WrapError(err, "Generating stemcell id")
	}

	id = "sc-" + id

	stemcellPath := filepath.Join(f.opts.DirPath, id)

	err = f.upload(imagePath, stemcellPath)
	if err != nil {
		return nil, err
	}

	// For libvirt, we create a base domain from the stemcell disk
	err = f.createDomainFromStemcell(id, stemcellPath)
	if err != nil {
		return nil, err
	}

	stemcell := f.newStemcell(apiv1.NewStemcellCID(id))

	err = stemcell.Prepare()
	if err != nil {
		f.cleanUpPartialImport(id)
		return nil, bosherr.WrapErrorf(err, "Preparing stemcell")
	}

	return stemcell, err
}

func (f Factory) Find(cid apiv1.StemcellCID) (Stemcell, error) {
	return f.newStemcell(cid), nil
}

func (f Factory) newStemcell(cid apiv1.StemcellCID) StemcellImpl {
	path := filepath.Join(f.opts.DirPath, cid.AsString())
	return NewStemcellImpl(cid, path, f.driver, f.runner, f.logger)
}

func (f Factory) upload(imagePath, stemcellPath string) error {
	tmpDir, err := f.fs.TempDir("bosh-libvirt-cpi-stemcell-upload")
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating tmp stemcell directory")
	}

	defer f.fs.RemoveAll(tmpDir)

	err = f.compressor.DecompressFileToDir(imagePath, tmpDir, boshcmd.CompressorOptions{})
	if err != nil {
		return bosherr.WrapErrorf(err, "Unpacking stemcell '%s' to '%s'", imagePath, tmpDir)
	}

	_, _, err = f.runner.Execute("mkdir", "-p", stemcellPath)
	if err != nil {
		return bosherr.WrapError(err, "Creating stemcell parent")
	}

	switch f.opts.StorageController {
	case bpds.IDEController:
		err = f.switchRootDiskToIDEController(tmpDir)
		if err != nil {
			return bosherr.WrapError(err, "Switching root disk to IDE Controller")
		}
	case bpds.SATAController:
		err = f.switchRootDiskToSATAController(tmpDir)
		if err != nil {
			return bosherr.WrapError(err, "Switching root disk to SATA Controller")
		}
	default: // scsi
		// do nothing
	}

	for _, fileName := range []string{"image-disk1.vmdk", "image.mf", "image.ovf"} {
		err := f.runner.Upload(filepath.Join(tmpDir, fileName), filepath.Join(stemcellPath, fileName))
		if err != nil {
			return bosherr.WrapErrorf(err, "Uploading stemcell")
		}
	}

	return nil
}

func (f Factory) switchRootDiskToIDEController(tmpDir string) error {
	var beforeSHA1, afterSHA1 string

	{
		ovfPath := filepath.Join(tmpDir, "image.ovf")

		contents, err := f.fs.ReadFileString(ovfPath)
		if err != nil {
			return err
		}

		beforeSHA1 = fmt.Sprintf("%x", sha1.Sum([]byte(contents)))

		// http://blogs.vmware.com/vapp/2009/11/virtual-hardware-in-ovf-part-1.html
		// Parent=x references Item with InstanceID=x
		contents = strings.Replace(
			contents, "<rasd:Parent>3</rasd:Parent>", "<rasd:Parent>4</rasd:Parent>", 1)

		afterSHA1 = fmt.Sprintf("%x", sha1.Sum([]byte(contents)))

		err = f.fs.WriteFileString(ovfPath, contents)
		if err != nil {
			return err
		}
	}

	{
		mfPath := filepath.Join(tmpDir, "image.mf")

		mfContents, err := f.fs.ReadFileString(mfPath)
		if err != nil {
			return err
		}

		mfContents = strings.Replace(mfContents, beforeSHA1, afterSHA1, 1)

		err = f.fs.WriteFileString(mfPath, mfContents)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f Factory) switchRootDiskToSATAController(tmpDir string) error {
	var beforeSHA1, afterSHA1 string

	{
		ovfPath := filepath.Join(tmpDir, "image.ovf")

		contents, err := f.fs.ReadFileString(ovfPath)
		if err != nil {
			return err
		}

		beforeSHA1 = fmt.Sprintf("%x", sha1.Sum([]byte(contents)))

		// If it is still IDE, replace differently. New jammy stemcells are SATA and not IDE controller
		if strings.Contains(contents, "<rasd:Description>IDE Controller</rasd:Description>") {
			// Sata controller example found here:
			// https://communities.vmware.com/t5/ESXi-Discussions/How-to-add-SATA-controller-in-ESXi-5-5/m-p/935069/highlight/true#M80252
			contents = strings.Replace(
				contents, "<rasd:Parent>3</rasd:Parent>", "<rasd:Parent>4</rasd:Parent>", 1)

			contents = strings.Replace(
				contents, "<rasd:Description>IDE Controller</rasd:Description>", "<rasd:Description>SATA Controller</rasd:Description>", 1)

			contents = strings.Replace(
				contents, "<rasd:ElementName>ideController0</rasd:ElementName>", "<rasd:ElementName>sataController0</rasd:ElementName>", 1)

			contents = strings.Replace(
				contents, "<rasd:ResourceType>5</rasd:ResourceType>", "<rasd:ResourceSubType>AHCI</rasd:ResourceSubType><rasd:ResourceType>20</rasd:ResourceType>", 1)

		} else {
			contents = strings.Replace(
				contents, "<rasd:Parent>4</rasd:Parent>", "<rasd:Parent>3</rasd:Parent>", 1)
		}

		afterSHA1 = fmt.Sprintf("%x", sha1.Sum([]byte(contents)))

		err = f.fs.WriteFileString(ovfPath, contents)
		if err != nil {
			return err
		}
	}

	{
		mfPath := filepath.Join(tmpDir, "image.mf")

		mfContents, err := f.fs.ReadFileString(mfPath)
		if err != nil {
			return err
		}

		mfContents = strings.Replace(mfContents, beforeSHA1, afterSHA1, 1)

		err = f.fs.WriteFileString(mfPath, mfContents)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f Factory) createDomainFromStemcell(id, stemcellPath string) error {
	// Convert VMDK to qcow2 using native qemu package
	vmdkPath := filepath.Join(stemcellPath, "image-disk1.vmdk")
	qcow2Path := filepath.Join(stemcellPath, "image.qcow2")

	qemuImg := qemu.NewImage()
	err := qemuImg.Convert(vmdkPath, qcow2Path, qemu.FormatVMDK, qemu.FormatQCOW2)
	if err != nil {
		return bosherr.WrapError(err, "Converting VMDK to qcow2")
	}

	// Create a minimal domain XML for the stemcell
	domainXML := fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='MiB'>1024</memory>
  <vcpu>1</vcpu>
  <os>
    <type arch='x86_64'>hvm</type>
    <boot dev='hd'/>
  </os>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <interface type='network'>
      <source network='default'/>
      <model type='virtio'/>
    </interface>
  </devices>
</domain>`, id, qcow2Path)

	// Write domain XML to temp file
	xmlPath := filepath.Join(stemcellPath, "domain.xml")
	err = f.fs.WriteFileString(xmlPath, domainXML)
	if err != nil {
		return bosherr.WrapError(err, "Writing domain XML")
	}

	// Define the domain
	_, err = f.driver.Execute("define", xmlPath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Defining stemcell domain")
	}

	return nil
}

func (f Factory) cleanUpPartialImport(suggestedNameOrID string) {
	_, err := f.driver.Execute("undefine", suggestedNameOrID, "--remove-all-storage")
	if err != nil {
		f.logger.Error(f.logTag, "Failed to clean up partially imported stemcell: %s", err)
	}
}
