package stemcell

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	// DecompressImage decompresses a gzip image to dst; injectable for testing.
	DecompressImage func(src, dst string) error

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
			out, _, err := runner.Execute("qemu-img", "convert", "-f", "raw", "-O", "qcow2", src, dst)
			if err != nil {
				return bosherr.WrapErrorf(err, "qemu-img: %s", out)
			}
			return nil
		},
		DecompressImage: decompressOrCopy,

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
		// The stemcell image is gzip-compressed raw disk; decompress then convert to qcow2.
		// ConvertToQCOW2 runs qemu-img via runner.Execute which may be SSH (remote host).
		// Decompress locally first, then upload the raw file to the remote host before
		// converting, so qemu-img can open it. Clean up the remote raw file afterwards.
		rawTmp := dstImage + ".raw"
		if err := f.DecompressImage(imagePath, rawTmp); err != nil {
			return bosherr.WrapError(err, "Decompressing stemcell image")
		}
		defer func() { _ = os.Remove(rawTmp) }()
		if isSSHRunner(f.runner) {
			if _, _, mkdirErr := f.runner.Execute("mkdir", "-p", filepath.Dir(rawTmp)); mkdirErr == nil {
				if uploadErr := f.runner.Upload(rawTmp, rawTmp); uploadErr == nil {
					defer func() { _, _, _ = f.runner.Execute("rm", "-f", rawTmp) }()
				}
			}
		}
		if err := f.ConvertToQCOW2(rawTmp, dstImage); err != nil {
			return bosherr.WrapErrorf(err, "Converting stemcell image to qcow2")
		}
	case "raw":
		// The bosh-warden-boshlite image is gzip-compressed; decompress to a
		// plain raw filesystem image for libvirt-lxc.
		if err := decompressOrCopy(imagePath, dstImage); err != nil {
			return bosherr.WrapError(err, "Preparing raw stemcell image")
		}
	case "ext4":
		// Build an ext4 raw image from the stemcell.
		// The bosh-warden stemcell tarball contains an `image` file which is
		// itself a gzip-compressed tar of the rootfs (warden-tar format).
		// Steps:
		//   1. Extract outer stemcell tgz → get `image` (gzipped rootfs tar).
		//   2. Extract `image` (the inner gzip+tar) → actual rootfs tree.
		//   3. Size the ext4 image, format, mount, copy rootfs in, unmount.
		//   4. If SSH runner: upload to remote host.
		outerDir := dstImage + ".outer"
		if err := os.MkdirAll(outerDir, 0755); err != nil {
			return bosherr.WrapError(err, "Creating temp outer stemcell dir")
		}
		defer func() { _ = os.RemoveAll(outerDir) }()
		if out, err := exec.Command("tar", "-xzf", imagePath, "-C", outerDir).CombinedOutput(); err != nil {
			return bosherr.WrapErrorf(err, "Extracting outer stemcell tgz: %s", string(out))
		}
		// The inner rootfs archive is always named `image` in BOSH stemcells.
		innerImage := filepath.Join(outerDir, "image")
		tmpDir := dstImage + ".rootfs"
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return bosherr.WrapError(err, "Creating temp rootfs dir")
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()
		if _, statErr := os.Stat(innerImage); statErr == nil {
			// Extract the inner gzipped rootfs tar.
			if out, err := exec.Command("tar", "-xzf", innerImage, "-C", tmpDir).CombinedOutput(); err != nil {
				return bosherr.WrapErrorf(err, "Extracting inner rootfs image: %s", string(out))
			}
		} else {
			// Fallback: outer tgz IS the rootfs tar (non-warden stemcell format).
			if out, err := exec.Command("tar", "-xzf", imagePath, "-C", tmpDir).CombinedOutput(); err != nil {
				return bosherr.WrapErrorf(err, "Extracting stemcell rootfs (fallback): %s", string(out))
			}
		}
		// Calculate size: du -sm gives MiB; add 20% headroom
		duOut, _ := exec.Command("du", "-sm", tmpDir).Output()
		sizeMB := 2048 // default 2 GiB
		if len(duOut) > 0 {
			var n int
			if _, err := fmt.Sscanf(string(duOut), "%d", &n); err == nil && n > 0 {
				sizeMB = n*120/100 + 64 // 20% headroom + 64 MB
			}
		}
		if out, err := exec.Command("dd", "if=/dev/zero", "of="+dstImage, "bs=1M",
			"count=0", "seek="+fmt.Sprintf("%d", sizeMB)).CombinedOutput(); err != nil {
			return bosherr.WrapErrorf(err, "Creating ext4 image file: %s", string(out))
		}
		if out, err := exec.Command("mkfs.ext4", "-F", dstImage).CombinedOutput(); err != nil {
			return bosherr.WrapErrorf(err, "Formatting ext4 image: %s", string(out))
		}
		mntDir := dstImage + ".mnt"
		if err := os.MkdirAll(mntDir, 0755); err != nil {
			return bosherr.WrapError(err, "Creating mount point")
		}
		defer func() { _ = exec.Command("umount", mntDir).Run(); _ = os.RemoveAll(mntDir) }()
		if out, err := exec.Command("mount", "-o", "loop", dstImage, mntDir).CombinedOutput(); err != nil {
			return bosherr.WrapErrorf(err, "Mounting ext4 image: %s", string(out))
		}
		if out, err := exec.Command("cp", "-a", tmpDir+"/.", mntDir+"/").CombinedOutput(); err != nil {
			return bosherr.WrapErrorf(err, "Copying rootfs into ext4 image: %s", string(out))
		}
		// Unmount before uploading so the image is fully flushed.
		_ = exec.Command("umount", mntDir).Run()
		// When the CPI is running inside a VM (SSH runner to a remote libvirt host),
		// upload the locally-created ext4 image to the remote host so create_vm can
		// access it there. Use the absolute local path as the upload source to avoid
		// the tilde being expanded to the *remote* home dir by ExpandingPathRunner.
		if isSSHRunner(f.runner) {
			localAbs := dstImage
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				localAbs = strings.Replace(dstImage, "~", home, 1)
			}
			if _, _, mkdirErr := f.runner.Execute("mkdir", "-p", filepath.Dir(dstImage)); mkdirErr == nil {
				_ = f.runner.Upload(localAbs, dstImage)
			}
		}
	case "dir":
		remoteTar := dstImage + ".tgz"
		if _, _, mkErr := f.runner.Execute("mkdir", "-p", dstImage); mkErr != nil {
			return bosherr.WrapError(mkErr, "Creating stemcell rootfs directory on remote")
		}
		// SSHRunner.Upload copies imagePath to remoteTar on the remote host; LocalRunner.Upload
		// moves it (rename). After this call, extraction runs from remoteTar on the remote side.
		if uploadErr := f.runner.Upload(imagePath, remoteTar); uploadErr != nil {
			return bosherr.WrapError(uploadErr, "Uploading stemcell tarball to remote host")
		}
		defer func() { _, _, _ = f.runner.Execute("rm", "-f", remoteTar) }()
		out, exitCode, err := f.runner.Execute("tar", "--no-same-devices", "-xzf", remoteTar, "-C", dstImage)
		if err != nil {
			if strings.Contains(out, "unrecognized option") || strings.Contains(out, "unknown option") {
				out2, exitCode2, err2 := f.runner.Execute("tar", "-xzf", remoteTar, "-C", dstImage)
				if err2 != nil && exitCode2 != 2 {
					return bosherr.WrapErrorf(err2, "Extracting stemcell rootfs on remote: %s", out2)
				}
			} else if exitCode != 2 {
				return bosherr.WrapErrorf(err, "Extracting stemcell rootfs on remote: %s", out)
			}
		}
	default:
		if err := f.fs.CopyFile(imagePath, dstImage); err != nil {
			return bosherr.WrapErrorf(err, "Uploading stemcell image")
		}
	}

	// chmod only applies to file-based images, not directory/ext4-based ones.
	// For qcow2 with an SSH runner the image lives on the remote host, so use
	// runner.Execute; for all other cases use the local filesystem.
	if format != "dir" && format != "ext4" {
		if format == "qcow2" && isSSHRunner(f.runner) {
			if _, _, err := f.runner.Execute("chmod", "0644", dstImage); err != nil {
				return bosherr.WrapErrorf(err, "Setting stemcell image permissions (remote)")
			}
		} else {
			if err := f.fs.Chmod(dstImage, 0644); err != nil {
				return bosherr.WrapErrorf(err, "Setting stemcell image permissions")
			}
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

// isSSHRunner reports whether r routes commands to a remote host over SSH.
// The runner may be wrapped in an ExpandingPathRunner, so we unwrap it.
func isSSHRunner(r driver.Runner) bool {
	type unwrapper interface{ Unwrap() driver.RawRunner }
	if u, ok := r.(unwrapper); ok {
		r = u.Unwrap()
	}
	_, ok := r.(*driver.SSHRunner)
	return ok
}
