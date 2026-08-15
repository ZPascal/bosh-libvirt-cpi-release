package vm

import (
	"os"
	"os/exec"
	"path/filepath"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	bdisk "bosh-libvirt-cpi/disk"
	"bosh-libvirt-cpi/driver"
	bstem "bosh-libvirt-cpi/stemcell"
)

type FactoryOpts struct {
	DirPath string
	Network string // libvirt network name; defaults to "default" if empty
}

type Factory struct {
	opts    FactoryOpts
	uuidGen boshuuid.Generator

	driver      driver.Driver
	runner      driver.Runner
	domBuilder  driver.DomainBuilder
	diskFactory bdisk.Factory

	agentOptions       apiv1.AgentOptions
	stemcellAPIVersion apiv1.StemcellAPIVersion

	logTag string
	logger boshlog.Logger
}

func NewFactory(
	opts FactoryOpts,
	uuidGen boshuuid.Generator,
	driver driver.Driver,
	runner driver.Runner,
	domBuilder driver.DomainBuilder,
	diskFactory bdisk.Factory,
	agentOptions apiv1.AgentOptions,
	stemcellAPIVersion apiv1.StemcellAPIVersion,
	logger boshlog.Logger,
) Factory {
	return Factory{
		opts:    opts,
		uuidGen: uuidGen,

		driver:      driver,
		runner:      runner,
		domBuilder:  domBuilder,
		diskFactory: diskFactory,

		agentOptions:       agentOptions,
		stemcellAPIVersion: stemcellAPIVersion,

		logTag: "vm.Factory",
		logger: logger,
	}
}

func (f Factory) Create(
	agentID apiv1.AgentID,
	stemcell bstem.Stemcell,
	props apiv1.VMCloudProps,
	networks apiv1.Networks,
	env apiv1.VMEnv,
) (VM, error) {

	vmProps, err := NewVMProps(props)
	if err != nil {
		return nil, err
	}

	idInternal, err := f.uuidGen.Generate()
	if err != nil {
		return nil, bosherr.WrapError(err, "Generating VM id")
	}

	vmID := "vm-" + idInternal
	cid := apiv1.NewVMCID(vmID)

	vm := f.newVM(cid)

	// Create ephemeral disk before defining the domain so we can reference it.
	ephemeralDisk, err := f.diskFactory.Create(vmProps.EphemeralDisk)
	if err != nil {
		return nil, bosherr.WrapError(err, "Creating ephemeral disk")
	}

	// Build initial agent env, persist for later use by the agent.
	initialAgentEnv := apiv1.NewAgentEnvFactory().ForVM(
		agentID, vm.ID(), networks, env, f.agentOptions)

	initialAgentEnv.AttachSystemDisk(apiv1.NewDiskHintFromString("0"))
	initialAgentEnv.AttachEphemeralDisk(apiv1.NewDiskHintFromString(ephemeralDisk.ImagePath()))

	err = vm.ConfigureAgent(initialAgentEnv)
	if err != nil {
		f.cleanUpPartialCreate(vm)
		return nil, bosherr.WrapError(err, "Initial agent configuration")
	}

	// Default disk paths; overridden below for dir-format (container) backends.
	disks := driver.DomainDiskPaths{
		RootDisk:      stemcell.ImagePath(),
		EphemeralDisk: ephemeralDisk.ImagePath(),
	}

	// For container-based backends (LXC, kernel-boot QEMU) copy the stemcell
	// rootfs into the VM's own directory so each VM has a writable private copy,
	// then inject the agent env and (for QEMU) the init wrapper.
	if f.domBuilder.DiskImageFormat() == "dir" {
		vmRootfs := filepath.Join(f.opts.DirPath, vmID, "rootfs")
		out, copyErr := execCommand("cp", "-a", stemcell.ImagePath()+"/.", vmRootfs)
		if copyErr != nil {
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapErrorf(copyErr, "Copying stemcell rootfs to VM dir: %s", string(out))
		}

		envBytes, err := initialAgentEnv.AsBytes()
		if err != nil {
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapError(err, "Marshalling agent env for rootfs injection")
		}
		boshDir := vmRootfs + "/var/vcap/bosh"
		if mkErr := os.MkdirAll(boshDir, 0755); mkErr == nil {
			_ = os.WriteFile(boshDir+"/warden-cpi-agent-env.json", envBytes, 0644)
		}

		// For QEMU direct-kernel-boot: write an init wrapper that configures
		// networking via DHCP before exec'ing bosh-agent.
		if vmProps.Kernel != "" {
			initScript := "#!/bin/sh\n" +
				"# Mount essential pseudo-filesystems\n" +
				"mount -t proc proc /proc 2>/dev/null || true\n" +
				"mount -t sysfs sysfs /sys 2>/dev/null || true\n" +
				"mount -t devtmpfs devtmpfs /dev 2>/dev/null || true\n" +
				"# Bring up network via DHCP\n" +
				"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}')\n" +
				"if [ -n \"$IFACE\" ]; then\n" +
				"  ip link set \"$IFACE\" up\n" +
				"  /usr/sbin/dhclient -v \"$IFACE\" 2>/tmp/dhclient.log || true\n" +
				"fi\n" +
				"exec /var/vcap/bosh/bin/bosh-agent -C /var/vcap/bosh/agent.json -P warden\n"
			_ = os.WriteFile(vmRootfs+"/bosh-init", []byte(initScript), 0755)
		}

		// Override disk paths to use the per-VM rootfs copy.
		disks = driver.DomainDiskPaths{
			RootDisk:      vmRootfs,
			EphemeralDisk: ephemeralDisk.ImagePath(),
		}
	}

	domainProps := driver.VMDomainProps{
		CPUs:     vmProps.CPUs,
		MemoryMB: vmProps.Memory,
		Network:  f.opts.Network,
		MAC:      vmProps.MAC,
		Kernel:   vmProps.Kernel,
	}

	xml, err := f.domBuilder.BuildDomain(vmID, domainProps, disks)
	if err != nil {
		f.cleanUpPartialCreate(vm)
		return nil, bosherr.WrapError(err, "Building domain XML")
	}

	err = f.driver.DefineDomain(xml)
	if err != nil {
		f.cleanUpPartialCreate(vm)
		return nil, bosherr.WrapError(err, "Defining domain")
	}

	// Track ephemeral disk attachment for later DiskIDs accounting.
	err = vm.AttachEphemeralDisk(ephemeralDisk)
	if err != nil {
		f.cleanUpPartialCreate(vm)
		return nil, bosherr.WrapError(err, "Recording ephemeral disk attachment")
	}

	err = vm.Start()
	if err != nil {
		f.cleanUpPartialCreate(vm)
		return nil, bosherr.WrapError(err, "Starting VM")
	}

	return vm, nil
}

func (f Factory) cleanUpPartialCreate(vm VM) {
	err := vm.Delete()
	if err != nil {
		f.logger.Error(f.logTag, "Failed to clean up partially created VM: %s", err)
	}
}

func (f Factory) newVM(cid apiv1.VMCID) VMImpl {
	store := NewStore(filepath.Join(f.opts.DirPath, cid.AsString()), f.runner)
	return NewVMImpl(cid, store, f.stemcellAPIVersion, f.driver, f.logger)
}

func execCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (f Factory) Find(cid apiv1.VMCID) (VM, error) {
	return f.newVM(cid), nil
}
