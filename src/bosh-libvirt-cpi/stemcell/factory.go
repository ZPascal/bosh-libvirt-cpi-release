package stemcell

import (
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	"bosh-libvirt-cpi/driver"
)

type FactoryOpts struct {
	DirPath string
}

type Factory struct {
	opts FactoryOpts

	driver     driver.Driver
	domBuilder driver.DomainBuilder
	runner     driver.Runner

	fs         boshsys.FileSystem
	uuidGen    boshuuid.Generator
	compressor boshcmd.Compressor

	// ConvertToQCOW2 converts a raw image to qcow2; injectable for testing.
	ConvertToQCOW2 func(src, dst string) error

	logTag string
	logger boshlog.Logger
}

func NewFactory(
	opts FactoryOpts,
	driver driver.Driver,
	domBuilder driver.DomainBuilder,
	runner driver.Runner,
	fs boshsys.FileSystem,
	uuidGen boshuuid.Generator,
	compressor boshcmd.Compressor,
	logger boshlog.Logger,
) Factory {
	return Factory{
		opts: opts,

		driver:     driver,
		domBuilder: domBuilder,
		runner:     runner,

		fs:         fs,
		uuidGen:    uuidGen,
		compressor: compressor,

		ConvertToQCOW2: func(src, dst string) error {
			out, err := exec.Command("qemu-img", "convert", "-f", "raw", "-O", "qcow2", src, dst).CombinedOutput()
			if err != nil {
				return bosherr.WrapErrorf(err, "qemu-img: %s", string(out))
			}
			return nil
		},

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

	stemcell := f.newStemcell(apiv1.NewStemcellCID(id))

	err = stemcell.Prepare()
	if err != nil {
		f.cleanUpPartialImport(stemcell)
		return nil, bosherr.WrapErrorf(err, "Preparing stemcell")
	}

	return stemcell, nil
}

func (f Factory) Find(cid apiv1.StemcellCID) (Stemcell, error) {
	return f.newStemcell(cid), nil
}

func (f Factory) newStemcell(cid apiv1.StemcellCID) StemcellImpl {
	path := filepath.Join(f.opts.DirPath, cid.AsString())
	return NewStemcellImpl(cid, path, f.driver, f.domBuilder, f.runner, f.logger)
}

func (f Factory) upload(imagePath, stemcellPath string) error {
	err := f.fs.MkdirAll(stemcellPath, 0755)
	if err != nil {
		return bosherr.WrapError(err, "Creating stemcell parent")
	}

	format := f.domBuilder.DiskImageFormat()
	dstImage := filepath.Join(stemcellPath, "image."+format)

	switch format {
	case "qcow2":
		if err := f.ConvertToQCOW2(imagePath, dstImage); err != nil {
			return bosherr.WrapErrorf(err, "Converting stemcell image to qcow2")
		}
	case "raw":
		// The bosh-warden-boshlite image is gzip-compressed; decompress to a
		// plain raw filesystem image for libvirt-lxc.
		if err := decompressOrCopy(imagePath, dstImage); err != nil {
			return bosherr.WrapError(err, "Preparing raw stemcell image")
		}
	case "dir":
		// Extract the gzip-compressed tar into a directory for libvirt-lxc mount.
		if err := os.MkdirAll(dstImage, 0755); err != nil {
			return bosherr.WrapError(err, "Creating stemcell rootfs directory")
		}
		out, err := exec.Command("tar", "-xzf", imagePath, "-C", dstImage).CombinedOutput()
		if err != nil {
			return bosherr.WrapErrorf(err, "Extracting stemcell rootfs: %s", string(out))
		}
	default:
		if err := f.fs.CopyFile(imagePath, dstImage); err != nil {
			return bosherr.WrapErrorf(err, "Uploading stemcell image")
		}
	}

	// chmod only applies to file-based images, not directory-based ones.
	if format != "dir" {
		if err := f.fs.Chmod(dstImage, 0644); err != nil {
			return bosherr.WrapErrorf(err, "Setting stemcell image permissions")
		}
	}

	return nil
}

func (f Factory) cleanUpPartialImport(stemcell StemcellImpl) {
	err := stemcell.Delete()
	if err != nil {
		f.logger.Error(f.logTag, "Failed to clean up partially imported stemcell: %s", err)
	}
}

// decompressOrCopy writes src to dst, decompressing gzip if detected.
func decompressOrCopy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
		if err != nil {
			_ = os.Remove(dst)
		}
	}()

	gr, gzErr := gzip.NewReader(in)
	if gzErr != nil {
		// Not gzip — rewind and copy as-is.
		if _, err = in.Seek(0, io.SeekStart); err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		return err
	}
	defer gr.Close() //nolint:errcheck
	_, err = io.Copy(out, gr)
	return err
}
