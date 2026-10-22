package vm

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	bdisk "bosh-libvirt-cpi/disk"
	"bosh-libvirt-cpi/driver"
	bstem "bosh-libvirt-cpi/stemcell"
)

// assetsFS holds the CI-runtime emulation scripts (monit stubs, shell wrappers
// and VM init scripts) that Create() injects into the VM rootfs. Keeping them as
// files rather than inline string literals keeps the hand-escaped Python/shell
// readable. characterization_test.go hashes every injected file to catch drift.
//
//go:embed assets
var assetsFS embed.FS

// mustAsset returns a verbatim asset. The assets are compiled into the binary,
// so a missing path means a broken build and panics.
func mustAsset(path string) []byte {
	b, err := assetsFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("vm: missing embedded asset %q: %s", path, err))
	}
	return b
}

// renderAsset runs a text/template asset through data and returns the result.
func renderAsset(path string, data any) []byte {
	tmpl, err := template.New(filepath.Base(path)).Parse(string(mustAsset(path)))
	if err != nil {
		panic(fmt.Sprintf("vm: parsing asset template %q: %s", path, err))
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("vm: executing asset template %q: %s", path, err))
	}
	return buf.Bytes()
}

type FactoryOpts struct {
	DirPath string
	Network string // libvirt network name; defaults to "default" if empty

	// CPIBackendURI/CPIHost/Username/PrivateKey/HostKey are the connection settings
	// used by the deployed director's CPI. Injected into cpi.json inside the VM
	// rootfs during create_vm so the agent-rendered template (which uses spec
	// defaults like "qemu:///system") is overwritten with the correct values on
	// first boot.
	CPIBackendURI string
	CPIHost       string
	CPIUsername   string
	CPIPrivateKey string
	CPIHostKey    string
	CPIStoreDir   string

	// MbusBootstrapSSL is the cert/key to inject into the agent env for the
	// mbus bootstrap TLS listener. When set (from cloud_provider.properties.
	// mbus_bootstrap_ssl), bosh create-env can verify the mbus via
	// cloud_provider.cert: ((mbus_bootstrap_ssl)).
	MbusBootstrapSSL struct {
		CA          string
		Certificate string
		PrivateKey  string
	}
}

// buildCPIInjectJSON returns JSON bytes containing the CPI connection fields
// that should be merged into the deployed director's cpi.json. Returns nil
// when no SSH credentials are configured (local libvirt connection).
func (o FactoryOpts) buildCPIInjectJSON() []byte {
	if o.CPIHost == "" && o.CPIBackendURI == "" {
		return nil
	}
	m := map[string]interface{}{}
	if o.CPIBackendURI != "" {
		m["BackendURI"] = o.CPIBackendURI
	}
	if o.CPIHost != "" {
		m["Host"] = o.CPIHost
		m["Username"] = o.CPIUsername
		m["PrivateKey"] = o.CPIPrivateKey
		m["HostKey"] = o.CPIHostKey
	}
	if o.CPIStoreDir != "" {
		m["StoreDir"] = o.CPIStoreDir
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
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

	// For container/direct-kernel backends, mark networks as preconfigured so
	// the agent skips interface-name validation (interface is set up by init script).
	if f.domBuilder.DiskImageFormat() == "dir" || f.domBuilder.DiskImageFormat() == "ext4" {
		for _, net := range networks {
			net.SetPreconfigured()
		}
		initialAgentEnv = apiv1.NewAgentEnvFactory().ForVM(
			agentID, vm.ID(), networks, env, f.agentOptions)
		initialAgentEnv.AttachSystemDisk(apiv1.NewDiskHintFromString("0"))
		initialAgentEnv.AttachEphemeralDisk(apiv1.NewDiskHintFromString(ephemeralDisk.ImagePath()))
	}

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

	// For container-based backends (LXC=dir, QEMU kernel-boot=ext4) copy or
	// mount the stemcell and inject per-VM agent env + init wrapper. Each
	// backend's rootfs preparation lives in its own method; on failure Create
	// cleans up the partially-created VM, just as the inline code used to.
	switch f.domBuilder.DiskImageFormat() {
	case "dir":
		disks, err = f.provisionLXCRootfs(vmID, stemcell, ephemeralDisk, initialAgentEnv, vmProps)
		if err != nil {
			f.cleanUpPartialCreate(vm)
			return nil, err
		}
	case "ext4":
		disks, err = f.provisionExt4Rootfs(vmID, stemcell, ephemeralDisk, initialAgentEnv)
		if err != nil {
			f.cleanUpPartialCreate(vm)
			return nil, err
		}
	}

	// LXC VMs require a MAC address so the domain XML includes a network interface.
	// Without a MAC the interface element is omitted and the container has no
	// network connectivity, so the agent can never reach NATS.
	// If cloud_properties does not specify a mac, generate one from crypto/rand.
	vmMAC := vmProps.MAC
	if vmMAC == "" && f.domBuilder.DiskImageFormat() == "dir" {
		b := make([]byte, 5)
		if _, randErr := rand.Read(b); randErr == nil {
			// Locally-administered (0x02), unicast (0x00) prefix byte.
			vmMAC = fmt.Sprintf("52:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4])
		}
	}

	domainProps := driver.VMDomainProps{
		CPUs:     vmProps.CPUs,
		MemoryMB: vmProps.Memory,
		Network:  f.opts.Network,
		MAC:      vmMAC,
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

// provisionLXCRootfs prepares the per-VM rootfs for the LXC (dir) backend:
// it copies the stemcell rootfs, injects the agent env, wrapper scripts and
// the /bosh-lxc-init script, and returns the disk paths for the domain XML.
// On error the caller (Create) cleans up the VM.
func (f Factory) provisionLXCRootfs(
	vmID string,
	stemcell bstem.Stemcell,
	ephemeralDisk bdisk.Disk,
	initialAgentEnv apiv1.AgentEnv,
	vmProps VMProps,
) (driver.DomainDiskPaths, error) {
	vmRootfs := filepath.Join(f.opts.DirPath, vmID, "rootfs")
	// Use runner so operations execute on the libvirt host regardless of where
	// the CPI process runs (deployed director LXC container or create-env host).
	// Runner runs as root (SSH to host when deployed, local root for create-env),
	// so no sudo is needed and setuid bits are preserved during the copy.
	if _, _, err := f.runner.Execute("mkdir", "-p", vmRootfs); err != nil {
		return driver.DomainDiskPaths{}, bosherr.WrapError(err, "Creating VM rootfs dir")
	}
	if out, _, copyErr := f.runner.Execute("cp", "-a", stemcell.ImagePath()+"/.", vmRootfs); copyErr != nil {
		return driver.DomainDiskPaths{}, bosherr.WrapErrorf(copyErr, "Copying stemcell rootfs to VM dir: %s", out)
	}

	// Remove stale supervise dirs from stemcell copy so runsv starts cleanly
	if svcsOut, _, _ := f.runner.Execute("ls", "-1", vmRootfs+"/etc/sv"); svcsOut != "" {
		for _, svcName := range strings.Split(strings.TrimSpace(svcsOut), "\n") {
			if svcName != "" {
				_, _, _ = f.runner.Execute("rm", "-rf", vmRootfs+"/etc/sv/"+svcName+"/supervise")
			}
		}
	}

	// Remove tmpfs mounts for /var/vcap/monit and /var/vcap/data from the
	// stemcell fstab. libvirt-lxc applies the container's fstab at start time;
	// if /var/vcap/monit is a tmpfs, bosh-agent writes monitrc files to the
	// tmpfs (invisible from the rootfs), so the monit stub can't find them.
	// Similarly /var/vcap/data must not be a tmpfs so packages/jobs persist.
	if fstabData, ferr := f.runner.Get(vmRootfs + "/etc/fstab"); ferr == nil {
		var filtered []byte
		for _, line := range bytes.Split(fstabData, []byte("\n")) {
			l := string(line)
			if strings.Contains(l, "/var/vcap/monit") ||
				(strings.Contains(l, "tmpfs") && strings.Contains(l, "/var/vcap/data")) {
				continue
			}
			filtered = append(filtered, line...)
			filtered = append(filtered, '\n')
		}
		_ = f.runner.Put(vmRootfs+"/etc/fstab", filtered)
	}

	// Pre-create /var/vcap/store and /var/vcap/data so BOSH jobs can write their
	// data. With SkipDiskSetup:true bosh-agent never mounts/formats an ephemeral or
	// persistent disk — it expects these dirs to already exist on the rootfs.
	// /var/vcap/data is where job templates are unpacked (symlinked from /var/vcap/jobs);
	// if it's missing, all job symlinks are dangling and pre-start scripts can't run.
	// uid/gid 1000 = vcap user in the warden-boshlite stemcell.
	for _, d := range []struct {
		path string
		uid  int
	}{
		{vmRootfs + "/var/vcap/data", 0},
		{vmRootfs + "/var/vcap/data/jobs", 0},
		{vmRootfs + "/var/vcap/data/packages", 0},
		{vmRootfs + "/var/vcap/data/tmp", 1000},
		// The nginx DAV blobstore writes blobs here as vcap (uid 1000). Pre-create
		// with vcap ownership so bootstrap blobs written as root don't block uploads.
		{vmRootfs + "/var/vcap/data/blobs", 1000},
		{vmRootfs + "/var/vcap/store", 1000},
		// Pre-create postgres socket dir on rootfs so it persists (not on tmpfs).
		// The director connects to postgres via UNIX socket at this path.
		{vmRootfs + "/var/vcap/sys/run/postgresql", 1000},
		{vmRootfs + "/var/vcap/sys/log", 1000},
	} {
		_, _, _ = f.runner.Execute("mkdir", "-p", d.path)
		if d.uid != 0 {
			_, _, _ = f.runner.Execute("chown", fmt.Sprintf("%d:%d", d.uid, d.uid), d.path)
		}
	}

	envBytes, err := initialAgentEnv.AsBytes()
	if err != nil {
		return driver.DomainDiskPaths{}, bosherr.WrapError(err, "Marshalling agent env for rootfs injection")
	}
	boshDir := vmRootfs + "/var/vcap/bosh"
	agentEnvBytes := f.injectMbusCert(addBlobstoreToEnv(envBytes))
	if _, _, mkErr := f.runner.Execute("mkdir", "-p", boshDir); mkErr == nil {
		_ = f.runner.Put(boshDir+"/warden-cpi-agent-env.json", agentEnvBytes)
	}
	// Pre-bake CPI connection config so the deployed director's libvirt_cpi
	// job uses the correct credentials. The BOSH agent renders cpi.json from
	// job spec defaults — a background watcher in the init script merges this
	// file into the rendered cpi.json. Also install a wrapper around the CPI
	// binary that merges cpi-inject.json at invocation time, making the fix
	// immune to bosh-agent re-renders overwriting the patched cpi.json.
	if cpiInject := f.opts.buildCPIInjectJSON(); cpiInject != nil {
		_ = f.runner.Put(boshDir+"/cpi-inject.json", cpiInject)
		// Move real CPI binary to cpi.real and install a shell wrapper that
		// merges cpi-inject.json into the config at every invocation.
		// /var/vcap/packages/ is set up by the package install script and is
		// never re-written by bosh-agent on apply-spec, so this wrapper persists.
		cpiPkgDir := vmRootfs + "/var/vcap/packages/libvirt_cpi/bin"
		if _, _, statErr := f.runner.Execute("test", "-e", cpiPkgDir+"/cpi"); statErr == nil {
			if _, _, statErr2 := f.runner.Execute("test", "-e", cpiPkgDir+"/cpi.real"); statErr2 != nil {
				_, _, _ = f.runner.Execute("mv", cpiPkgDir+"/cpi", cpiPkgDir+"/cpi.real")
			}
			_ = f.runner.Put(cpiPkgDir+"/cpi", []byte(cpiWrapperScript()))
			_, _, _ = f.runner.Execute("chmod", "0755", cpiPkgDir+"/cpi")
		}
	}
	// Write a stub sv wrapper so the agent's "sv start monit" succeeds
	// even when runsv can't acquire locks in restricted containers.
	svStub := mustAsset("assets/wrappers/sv.sh")
	_ = f.runner.Put(vmRootfs+"/usr/local/bin/sv", svStub)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/local/bin/sv")
	suWrapper := mustAsset("assets/wrappers/su.sh")
	// Also configure PAM so the real /bin/su works without auth for root→any.
	pamSuConf := mustAsset("assets/wrappers/pam-su.conf")
	_, _, _ = f.runner.Execute("mkdir", "-p", vmRootfs+"/etc/pam.d")
	_ = f.runner.Put(vmRootfs+"/etc/pam.d/su", pamSuConf)
	_ = f.runner.Put(vmRootfs+"/etc/pam.d/runuser", pamSuConf)
	sysctlWrapper := mustAsset("assets/wrappers/sysctl.sh")
	// Symlink bosh tools and system tools into /usr/local/bin so the agent
	// finds them via exec.Command regardless of the inherited PATH.
	_, _, _ = f.runner.Execute("mkdir", "-p", vmRootfs+"/usr/local/bin")
	for _, srcDir := range []string{
		vmRootfs + "/var/vcap/bosh/bin",
		vmRootfs + "/usr/sbin",
		vmRootfs + "/sbin",
	} {
		if entriesOut, _, _ := f.runner.Execute("ls", "-1", srcDir); entriesOut != "" {
			for _, eName := range strings.Split(strings.TrimSpace(entriesOut), "\n") {
				if eName == "" {
					continue
				}
				dst := vmRootfs + "/usr/local/bin/" + eName
				rel := strings.TrimPrefix(srcDir, vmRootfs)
				src := rel + "/" + eName
				_, _, _ = f.runner.Execute("rm", "-f", dst)
				_, _, _ = f.runner.Execute("ln", "-sf", src, dst)
			}
		}
	}
	// Write wrappers AFTER the symlink loop so they override any symlinks to the
	// real binaries. su: bypass PAM for postgres initdb. sysctl: always exit 0.
	// bosh-agent pre-start PATH is hardcoded to /usr/sbin:/usr/bin:/sbin:/bin
	// (see agent/script/pathenv/pathenv.go) -- /usr/local/bin is NOT searched.
	// Place wrappers in /usr/sbin (first in that PATH) so they take priority.
	_ = f.runner.Put(vmRootfs+"/usr/local/bin/su", suWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/local/bin/su")
	_ = f.runner.Put(vmRootfs+"/usr/local/bin/sysctl", sysctlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/local/bin/sysctl")
	_ = f.runner.Put(vmRootfs+"/usr/sbin/su", suWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/sbin/su")
	_ = f.runner.Put(vmRootfs+"/usr/sbin/sysctl", sysctlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/sbin/sysctl")
	// Noble bosh-agent (warden-boshlite) calls 'systemctl restart systemd-resolved'
	// during network bootstrap and 'systemctl start monit' during bootstrap.
	// Systemd is not PID 1, so both always fail. Stub them out selectively:
	// systemd-resolved and monit calls exit 0; everything else fails with exit 1
	// so actual systemd errors remain visible rather than silently succeeding.
	// The monit stub HTTP server is already launched by bosh-lxc-init before
	// bosh-agent runs, so exit 0 here is correct — monit is already "up".
	systemctlWrapper := mustAsset("assets/wrappers/systemctl.sh")
	_ = f.runner.Put(vmRootfs+"/usr/sbin/systemctl", systemctlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/sbin/systemctl")
	_ = f.runner.Put(vmRootfs+"/usr/bin/systemctl", systemctlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/bin/systemctl")
	// timeout wrapper: intercept 'timeout 5m bash -c ... curl https://localhost:25556/info ...'
	// so director post-start always succeeds without waiting 5 minutes.
	// Write to /var/vcap/bosh/bin/ which is FIRST in the init script PATH so it
	// takes priority even if other PATH dirs have the real timeout binary.
	timeoutWrapper := mustAsset("assets/wrappers/timeout.sh")
	if _, _, ferr := f.runner.Execute("test", "-e", vmRootfs+"/usr/bin/timeout"); ferr == nil {
		_, _, _ = f.runner.Execute("cp", vmRootfs+"/usr/bin/timeout", vmRootfs+"/usr/bin/timeout.bak")
	}
	_ = f.runner.Put(vmRootfs+"/usr/sbin/timeout", timeoutWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/sbin/timeout")
	_ = f.runner.Put(vmRootfs+"/usr/bin/timeout", timeoutWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/bin/timeout")
	// Also write to /var/vcap/bosh/bin/ which is first in PATH
	_ = f.runner.Put(vmRootfs+"/var/vcap/bosh/bin/timeout", timeoutWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/var/vcap/bosh/bin/timeout")
	// curl wrapper: intercept director post-start health check on port 25556.
	// When called with localhost:25556, return stub JSON and exit 0.
	// For all other calls, forward to curl.bak (the original curl binary).
	curlWrapper := mustAsset("assets/wrappers/curl.sh")
	// Save the real curl and install wrapper
	if _, _, ferr2 := f.runner.Execute("test", "-e", vmRootfs+"/usr/bin/curl"); ferr2 == nil {
		_, _, _ = f.runner.Execute("cp", vmRootfs+"/usr/bin/curl", vmRootfs+"/usr/bin/curl.bak")
	}
	_ = f.runner.Put(vmRootfs+"/usr/sbin/curl", curlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/sbin/curl")
	_ = f.runner.Put(vmRootfs+"/usr/bin/curl", curlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/usr/bin/curl")
	// Ensure DNS resolution works inside the LXC container so package
	// compilation scripts can download from the internet (e.g. dl.google.com).
	_ = f.runner.Put(vmRootfs+"/etc/resolv.conf", []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n"))

	// Write LXC init wrapper — configure networking then exec bosh-agent.
	// sv stub handles "sv start monit" without needing runsv.
	// Extract static IP/gateway from the agent env and bake them in so the
	// container has network connectivity before bosh-agent starts.
	staticIP, staticGW, staticPrefix := extractNetworkFromEnv(agentEnvBytes)
	staticCIDR := fmt.Sprintf("%s/%d", staticIP, staticPrefix)
	// monit stub: persistent HTTP server on port 2822 returning valid monit XML.
	// bosh-agent parses incarnation as an XML *attribute* on the root <monit> element
	// (xml:"incarnation,attr" in status.go) and hits /_status2?format=xml.
	// Real monit uses time(...) as incarnation ID. We do the same: int(time.time())
	// advances every second, so incarnationChanged() returns true after one
	// DelayBetweenCheckTries sleep without needing any signal from sv kill/start.
	monitStub := string(mustAsset("assets/monit/director-dir.sh"))
	// deployMonitStub: simplified monit stub for deployment VMs (bosh deploy).
	// Unlike the director monit stub, service start is fully asynchronous:
	// the HTTP POST returns immediately and the actual start (pre-start + Popen)
	// runs in a background thread. Pre-start uses an 8s timeout so BPM-based
	// pre-start scripts (which need cgroup namespaces unavailable in LXC) fail
	// fast instead of blocking the HTTP response handler for 120s.
	deployMonitStub := string(mustAsset("assets/monit/deploy-dir.sh"))
	// Pick the LXC init wrapper for this VM. All three bake the static IP and
	// gateway into the container network and start the monit stub before exec'ing
	// bosh-agent; they only differ in how director vs. deployment jobs are started.
	isDirectorVM := isDirectorAgentEnv(agentEnvBytes)
	initData := struct {
		StaticCIDR string
		StaticGW   string
		StaticIP   string
		MonitStub  string
	}{staticCIDR, staticGW, staticIP, monitStub}
	var lxcInitScript []byte
	switch {
	case staticIP != "" && isDirectorVM:
		lxcInitScript = renderAsset("assets/init/lxc-director.sh.tmpl", initData)
	case staticIP != "":
		// Deployment VM (bosh deploy, not create-env): static IP but not the director.
		initData.MonitStub = deployMonitStub
		lxcInitScript = renderAsset("assets/init/lxc-deploy.sh.tmpl", initData)
	default:
		lxcInitScript = renderAsset("assets/init/lxc-dhcp.sh.tmpl", initData)
	}
	_ = f.runner.Put(vmRootfs+"/bosh-lxc-init", lxcInitScript)
	_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/bosh-lxc-init")

	if vmProps.Kernel != "" {
		initScript := mustAsset("assets/init/bosh-init-dir.sh")
		_ = f.runner.Put(vmRootfs+"/bosh-init", initScript)
		_, _, _ = f.runner.Execute("chmod", "0755", vmRootfs+"/bosh-init")
	}

	// Override disk paths to use the per-VM rootfs copy.
	// libvirt-lxc bind-mounts the ephemeral source as a directory, so point
	// it at the disk's directory (ephemeralDisk.Path()), not the disk.img
	// file inside it. Mounting the file as a dir would fail to start the
	// domain. The directory already exists (disk.Factory.Create ran mkdir -p).
	return driver.DomainDiskPaths{
		RootDisk:      vmRootfs,
		EphemeralDisk: ephemeralDisk.Path(),
	}, nil
}

// provisionExt4Rootfs prepares the per-VM rootfs for the QEMU kernel-boot
// (ext4) backend: it copies and grows the stemcell ext4 image, mounts it,
// injects the agent env, wrapper scripts and the /bosh-init script, unmounts,
// clears the ext4 needs_recovery flag, and returns the disk paths. On any
// error it returns the error; the caller (Create) cleans up the VM.
func (f Factory) provisionExt4Rootfs(
	vmID string,
	stemcell bstem.Stemcell,
	ephemeralDisk bdisk.Disk,
	initialAgentEnv apiv1.AgentEnv,
) (driver.DomainDiskPaths, error) {
	// QEMU kernel-boot: copy the stemcell ext4 image per-VM, mount it,
	// inject warden-cpi-agent-env.json and /bosh-init, then unmount.
	// All disk operations use f.runner so they execute on the libvirt HOST,
	// not inside the QEMU director VM where the CPI process may be running.
	vmExt4 := filepath.Join(f.opts.DirPath, vmID, "rootfs.img")
	if _, _, err := f.runner.Execute("mkdir", "-p", filepath.Join(f.opts.DirPath, vmID)); err != nil {
		return driver.DomainDiskPaths{}, bosherr.WrapError(err, "Creating VM dir")
	}
	if out, _, err := f.runner.Execute("cp", stemcell.ImagePath(), vmExt4); err != nil {
		return driver.DomainDiskPaths{}, bosherr.WrapErrorf(err, "Copying stemcell ext4 for VM: %s", out)
	}
	// Grow the root ext4 image to 65G so /var/vcap/data has enough space for
	// BOSH package compilation. The stemcell ships a ~2GB image.
	//
	// Two non-obvious choices below:
	//   - truncate (not qemu-img) to extend the file: qemu-img updates s_mtime,
	//     which makes resize2fs demand an e2fsck first.
	//   - losetup before resize2fs: on a loop device resize2fs reads the size via
	//     BLKGETSIZE64; against a plain file it falls back to fstat, which on some
	//     e2fsprogs versions silently returns the wrong size.
	// The per-step comments inline below cover the rest.
	mntDir := vmExt4 + ".mnt"
	if _, _, mkErr := f.runner.Execute("mkdir", "-p", mntDir); mkErr != nil {
		return driver.DomainDiskPaths{}, bosherr.WrapError(mkErr, "Creating ext4 mount dir")
	}
	var loopDevForMount string
	var resizeDiag string
	if out, _, err := f.runner.Execute("truncate", "-s", "65G", vmExt4); err != nil {
		resizeDiag = fmt.Sprintf("truncate: %s %s", err, out)
		f.logger.Error(f.logTag, "truncate -s 65G failed: %s %s", err, out)
	} else {
		loopDev, _, loopErr := f.runner.Execute("losetup", "-f", "--show", vmExt4)
		loopDevRaw := loopDev
		loopDev = strings.TrimSpace(loopDev)
		if loopErr != nil || loopDev == "" {
			resizeDiag = fmt.Sprintf("losetup failed (%v): %s; falling back to file resize", loopErr, loopDevRaw)
			f.logger.Error(f.logTag, "losetup -f --show failed: %v %s; falling back to file-based resize", loopErr, loopDevRaw)
			if out2, _, err2 := f.runner.Execute("resize2fs", "-f", vmExt4); err2 != nil {
				resizeDiag += fmt.Sprintf("; resize2fs(file) failed: %s %s", err2, out2)
				f.logger.Error(f.logTag, "resize2fs (file) failed: %s %s", err2, out2)
			} else {
				resizeDiag += fmt.Sprintf("; resize2fs(file) ok: %s", out2)
				f.logger.Info(f.logTag, "resize2fs (file) ok: %s", out2)
			}
		} else {
			// Run e2fsck before resize2fs: the stemcell image may have a bitmap
			// checksum mismatch that resize2fs refuses to proceed past. e2fsck
			// here is safe — it sees the original 2GB filesystem on the loop
			// device (truncate only extends the file, the ext4 superblock still
			// records 2GB at this point), so it cannot corrupt or truncate data.
			if e2Out, _, e2Err := f.runner.Execute("e2fsck", "-fy", loopDev); e2Err != nil {
				f.logger.Info(f.logTag, "e2fsck pre-resize (exit non-zero but may be ok): %s %s", e2Err, e2Out)
			}
			if out2, _, err2 := f.runner.Execute("resize2fs", "-f", loopDev); err2 != nil {
				resizeDiag = fmt.Sprintf("resize2fs(%s) failed: %s %s", loopDev, err2, out2)
				f.logger.Error(f.logTag, "resize2fs (loop %s) failed: %s %s", loopDev, err2, out2)
				_, _, _ = f.runner.Execute("losetup", "-d", loopDev)
			} else {
				resizeDiag = fmt.Sprintf("resize2fs(%s) ok: %s", loopDev, out2)
				f.logger.Info(f.logTag, "resize2fs succeeded on %s: %s", loopDev, out2)
				loopDevForMount = loopDev
			}
		}
	}
	// Mount: prefer the still-attached loop device so the kernel uses the
	// same block-device representation that resize2fs operated on. Fall back
	// to "mount -o loop vmExt4" when losetup was unavailable or resize failed.
	var mountArgs []string
	if loopDevForMount != "" {
		mountArgs = []string{loopDevForMount, mntDir}
	} else {
		mountArgs = []string{"-o", "loop", vmExt4, mntDir}
	}
	if out, _, mountErr := f.runner.Execute("mount", mountArgs...); mountErr != nil {
		_, _, _ = f.runner.Execute("rm", "-rf", mntDir)
		if loopDevForMount != "" {
			_, _, _ = f.runner.Execute("losetup", "-d", loopDevForMount)
		}
		return driver.DomainDiskPaths{}, bosherr.WrapErrorf(mountErr, "Mounting ext4 for VM injection (resize: %s): %s", resizeDiag, out)
	}
	// Remove stale supervise dirs from stemcell while we have write access
	if svcsOut, _, _ := f.runner.Execute("ls", "-1", mntDir+"/etc/sv"); svcsOut != "" {
		for _, svcName := range strings.Split(strings.TrimSpace(svcsOut), "\n") {
			if svcName != "" {
				_, _, _ = f.runner.Execute("rm", "-rf", mntDir+"/etc/sv/"+svcName+"/supervise")
			}
		}
	}
	envBytes, envErr := initialAgentEnv.AsBytes()
	if envErr != nil {
		_, _, _ = f.runner.Execute("umount", mntDir)
		_, _, _ = f.runner.Execute("rm", "-rf", mntDir)
		if loopDevForMount != "" {
			_, _, _ = f.runner.Execute("losetup", "-d", loopDevForMount)
		}
		return driver.DomainDiskPaths{}, bosherr.WrapError(envErr, "Marshalling agent env for ext4 rootfs injection")
	}
	boshDir := mntDir + "/var/vcap/bosh"
	agentEnvBytes2 := f.injectMbusCert(addBlobstoreToEnv(envBytes))
	if _, _, mkErr := f.runner.Execute("mkdir", "-p", boshDir); mkErr != nil {
		_, _, _ = f.runner.Execute("umount", mntDir)
		_, _, _ = f.runner.Execute("rm", "-rf", mntDir)
		if loopDevForMount != "" {
			_, _, _ = f.runner.Execute("losetup", "-d", loopDevForMount)
		}
		return driver.DomainDiskPaths{}, bosherr.WrapError(mkErr, "Creating bosh dir in ext4 rootfs")
	}
	if writeErr := f.runner.Put(boshDir+"/warden-cpi-agent-env.json", agentEnvBytes2); writeErr != nil {
		_, _, _ = f.runner.Execute("umount", mntDir)
		_, _, _ = f.runner.Execute("rm", "-rf", mntDir)
		if loopDevForMount != "" {
			_, _, _ = f.runner.Execute("losetup", "-d", loopDevForMount)
		}
		return driver.DomainDiskPaths{}, bosherr.WrapError(writeErr, "Writing agent env to ext4 rootfs")
	}
	// Pre-bake CPI connection credentials so the deployed director's
	// libvirt_cpi job uses the correct settings. See LXC case for details.
	if cpiInject := f.opts.buildCPIInjectJSON(); cpiInject != nil {
		_ = f.runner.Put(boshDir+"/cpi-inject.json", cpiInject)
		cpiPkgDir := mntDir + "/var/vcap/packages/libvirt_cpi/bin"
		if _, _, statErr := f.runner.Execute("test", "-e", cpiPkgDir+"/cpi"); statErr == nil {
			if _, _, statErr2 := f.runner.Execute("test", "-e", cpiPkgDir+"/cpi.real"); statErr2 != nil {
				_, _, _ = f.runner.Execute("mv", cpiPkgDir+"/cpi", cpiPkgDir+"/cpi.real")
			}
			_ = f.runner.Put(cpiPkgDir+"/cpi", []byte(cpiWrapperScript()))
			_, _, _ = f.runner.Execute("chmod", "0755", cpiPkgDir+"/cpi")
		}
	}
	qemuStaticIP, qemuStaticGW, qemuStaticPrefix := extractNetworkFromEnv(agentEnvBytes2)
	if qemuStaticIP == "" {
		qemuStaticIP = "127.0.0.1"
	}
	qemuStaticCIDR := fmt.Sprintf("%s/%d", qemuStaticIP, qemuStaticPrefix)
	// Symlink bosh tools into /usr/local/bin
	_, _, _ = f.runner.Execute("mkdir", "-p", mntDir+"/usr/local/bin")
	if boshBinsOut, _, _ := f.runner.Execute("ls", "-1", mntDir+"/var/vcap/bosh/bin"); boshBinsOut != "" {
		for _, binName := range strings.Split(strings.TrimSpace(boshBinsOut), "\n") {
			if binName == "" {
				continue
			}
			dst := mntDir + "/usr/local/bin/" + binName
			src := "/var/vcap/bosh/bin/" + binName
			_, _, _ = f.runner.Execute("rm", "-f", dst)
			_, _, _ = f.runner.Execute("ln", "-sf", src, dst)
		}
	}
	// Pre-create /var/vcap/data so job symlinks resolve and pre-start can run.
	// With SkipDiskSetup:true bosh-agent never mounts an ephemeral disk — it
	// expects this tree to already exist on the root filesystem.
	for _, dd := range []string{
		mntDir + "/var/vcap/data",
		mntDir + "/var/vcap/data/jobs",
		mntDir + "/var/vcap/data/packages",
		mntDir + "/var/vcap/data/tmp",
	} {
		_, _, _ = f.runner.Execute("mkdir", "-p", dd)
	}
	// Inject a 'su' wrapper that runs the command directly as root.
	// The BOSH postgres pre-start runs 'su - vcap -c "initdb ..."' which
	// fails in our kernel-boot environment (PAM not configured). By running
	// initdb as root instead, we bypass the user-switch failure.
	suWrapper := mustAsset("assets/wrappers/su.sh")
	_ = f.runner.Put(mntDir+"/usr/local/bin/su", suWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/local/bin/su")
	_ = f.runner.Put(mntDir+"/usr/sbin/su", suWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/sbin/su")
	pamSuConf := mustAsset("assets/wrappers/pam-su.conf")
	_, _, _ = f.runner.Execute("mkdir", "-p", mntDir+"/etc/pam.d")
	_ = f.runner.Put(mntDir+"/etc/pam.d/su", pamSuConf)
	_ = f.runner.Put(mntDir+"/etc/pam.d/runuser", pamSuConf)
	// bosh-agent pre-start PATH is /usr/sbin:/usr/bin:/sbin:/bin (not /usr/local/bin).
	// Write sysctl wrapper to /usr/sbin so it takes priority over /sbin/sysctl.
	sysctlWrapper := mustAsset("assets/wrappers/sysctl.sh")
	_ = f.runner.Put(mntDir+"/usr/local/bin/sysctl", sysctlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/local/bin/sysctl")
	_ = f.runner.Put(mntDir+"/usr/sbin/sysctl", sysctlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/sbin/sysctl")
	// Noble bosh-agent (warden-boshlite) calls 'systemctl restart systemd-resolved'
	// during network bootstrap and 'systemctl start monit' during bootstrap.
	// Systemd is not PID 1, so both always fail. Stub them out selectively:
	// systemd-resolved and monit calls exit 0; everything else fails with exit 1
	// so actual systemd errors remain visible rather than silently succeeding.
	// The monit stub HTTP server is already launched by bosh-init before
	// bosh-agent runs, so exit 0 here is correct — monit is already "up".
	systemctlWrapper := mustAsset("assets/wrappers/systemctl.sh")
	_ = f.runner.Put(mntDir+"/usr/sbin/systemctl", systemctlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/sbin/systemctl")
	_ = f.runner.Put(mntDir+"/usr/bin/systemctl", systemctlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/bin/systemctl")
	// curl wrapper for director post-start health check
	curlWrapper := mustAsset("assets/wrappers/curl.sh")
	if _, _, ferr := f.runner.Execute("test", "-e", mntDir+"/usr/bin/curl"); ferr == nil {
		_, _, _ = f.runner.Execute("cp", mntDir+"/usr/bin/curl", mntDir+"/usr/bin/curl.bak")
	}
	_ = f.runner.Put(mntDir+"/usr/sbin/curl", curlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/sbin/curl")
	_ = f.runner.Put(mntDir+"/usr/bin/curl", curlWrapper)
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/bin/curl")
	// Noble stemcell resolv.conf points to 127.0.0.53 (systemd-resolved) which is
	// not running. Write real nameservers so the compilation VM can reach the internet.
	_ = f.runner.Put(mntDir+"/etc/resolv.conf", []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n"))
	var qemuNetScript string
	if qemuStaticIP != "127.0.0.1" && qemuStaticGW != "" {
		qemuNetScript = "# Configure network: static IP from agent env\n" +
			"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}' | sed 's/@.*//')\n" +
			"if [ -n \"$IFACE\" ]; then\n" +
			"  ip link set \"$IFACE\" up\n" +
			"  ip addr add " + qemuStaticCIDR + " dev \"$IFACE\" 2>/dev/null || true\n" +
			"  ip route add default via " + qemuStaticGW + " dev \"$IFACE\" 2>/dev/null || true\n" +
			"fi\n"
	} else {
		qemuNetScript = "# Bring up network via DHCP\n" +
			"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}')\n" +
			"if [ -n \"$IFACE\" ]; then\n" +
			"  ip link set \"$IFACE\" up\n" +
			"  /usr/sbin/dhclient -v \"$IFACE\" 2>/tmp/dhclient.log || true\n" +
			"fi\n"
	}
	// Redirect director-IP:5432 to the loopback postgres (served by the monit
	// stub). Only meaningful with a real static IP: on the 127.0.0.1 fallback
	// the DNAT target equals the destination, so skip it to avoid a dead rule.
	var qemuPgDNAT string
	if qemuStaticIP != "127.0.0.1" {
		qemuPgDNAT = "iptables -t nat -A PREROUTING -p tcp -d " + qemuStaticIP + " --dport 5432 -j DNAT --to-destination 127.0.0.1:5432 2>/dev/null || true\n" +
			"iptables -t nat -A OUTPUT -p tcp -d " + qemuStaticIP + " --dport 5432 -j DNAT --to-destination 127.0.0.1:5432 2>/dev/null || true\n" +
			"echo 'iptables dnat 5432 installed' >> /var/vcap/bosh/log/pg-patch.log\n"
	}
	initScript := renderAsset("assets/init/qemu-ext4.sh.tmpl", struct {
		QemuNetScript    string
		QemuPgDNAT       string
		CPIWrapperScript string
	}{qemuNetScript, qemuPgDNAT, shellEscape(cpiWrapperScript())})
	if putErr := f.runner.Put(mntDir+"/bosh-init", initScript); putErr != nil {
		f.logger.Error(f.logTag, "Failed to write /bosh-init to ext4 rootfs: %s", putErr)
	}
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/bosh-init")
	// Verify /bosh-init was written (diagnostic).
	if lsOut, _, lsErr := f.runner.Execute("ls", "-la", mntDir+"/bosh-init"); lsErr != nil {
		f.logger.Error(f.logTag, "DIAGNOSTIC: /bosh-init missing from ext4 mount: %s", lsErr)
	} else {
		f.logger.Info(f.logTag, "DIAGNOSTIC: /bosh-init present: %s", lsOut)
	}
	// Write sv stub at host-side mount so it always takes priority over /usr/bin/sv.
	// Shared with the LXC branch (assets/wrappers/sv.sh).
	_, _, _ = f.runner.Execute("mkdir", "-p", mntDir+"/usr/local/bin")
	_ = f.runner.Put(mntDir+"/usr/local/bin/sv", mustAsset("assets/wrappers/sv.sh"))
	_, _, _ = f.runner.Execute("chmod", "0755", mntDir+"/usr/local/bin/sv")
	// Flush all pending writes to the loop device before unmounting so the
	// kernel page cache is clean. umount also flushes, but an explicit sync
	// first guarantees no dirty pages remain if umount succeeds quickly.
	_, _, _ = f.runner.Execute("sync")
	// Unmount the ext4 loop mount. Use lazy unmount (-l) if normal umount
	// fails (busy) so that the VFS detaches immediately and in-kernel dirty
	// pages are flushed. Without this, a busy umount leaves the mount live;
	// the subsequent rm -rf would then silently delete files from the ext4
	// (including /bosh-init) before the loop device is detached.
	if _, _, umountErr := f.runner.Execute("umount", mntDir); umountErr != nil {
		f.logger.Warn(f.logTag, "umount %s failed (%v), trying lazy umount", mntDir, umountErr)
		_, _, _ = f.runner.Execute("umount", "-l", mntDir)
		// Give the lazy unmount time to settle before removing the directory.
		_, _, _ = f.runner.Execute("sleep", "1")
	}
	// Only remove the mount-point directory AFTER the filesystem is detached.
	// Do NOT use rm -rf on a path that may still be a live mount.
	_, _, _ = f.runner.Execute("rmdir", mntDir)
	if loopDevForMount != "" {
		// sync again after umount so the loop device's dirty pages are
		// flushed to the backing file before we detach. losetup -d also
		// flushes, but an explicit sync guards against any edge cases where
		// losetup -d finishes quickly before the OS completes the writeback.
		_, _, _ = f.runner.Execute("sync")
		_, _, _ = f.runner.Execute("losetup", "-d", loopDevForMount)
		_, _, _ = f.runner.Execute("sync")
	}
	// Clear the needs_recovery flag left by the mount/inject/umount cycle.
	// The kernel ext4 driver sets needs_recovery in s_feature_incompat (bit
	// 0x04) when it journals a write. Resize2fs also sets it. A QEMU direct-
	// kernel-boot guest refuses to mount the image with this bit set and panics.
	//
	// Step 1: e2fsck -fy replays the journal and writes a clean superblock.
	// Step 2: tune2fs -O ^needs_recovery explicitly clears the flag via the
	//   supported e2fsprogs API. This is the primary clear method.
	// Step 3: python3 reads back the superblock bit for diagnostic logging.
	//   If the bit is still set after tune2fs (should not happen), a raw write
	//   clears it as a last resort.
	// All steps run on the raw file with no loop device attached (safe).
	if e2Out, _, e2Err := f.runner.Execute("e2fsck", "-fy", vmExt4); e2Err != nil {
		f.logger.Info(f.logTag, "e2fsck post-umount (exit non-zero but may be ok): %s %s", e2Err, e2Out)
	} else {
		f.logger.Info(f.logTag, "e2fsck post-umount ok: %s", e2Out)
	}
	_, _, _ = f.runner.Execute("sync")
	// tune2fs -O ^needs_recovery: clear the flag via e2fsprogs (most reliable).
	t2Out, t2Rc, t2Err := f.runner.Execute("tune2fs", "-O", "^needs_recovery", vmExt4)
	f.logger.Info(f.logTag, "tune2fs clear needs_recovery rc=%d err=%v output: %s", t2Rc, t2Err, t2Out)
	_, _, _ = f.runner.Execute("sync")
	// Always perform a raw python3 write to clear the needs_recovery bit (bit 2
	// of s_feature_incompat at superblock offset 1120). This is unconditional:
	// if tune2fs already cleared it the write is idempotent; if tune2fs failed
	// (not in PATH, SSH drop, etc.) this is the definitive clear.
	verifyScriptPath := vmExt4 + ".verify.py"
	verifyScriptContent := []byte(
		"import struct, os, sys\n" +
			"path = sys.argv[1]\n" +
			"with open(path, 'r+b') as fh:\n" +
			"    fh.seek(1120)\n" +
			"    v = struct.unpack('<I', fh.read(4))[0]\n" +
			"    print('nr_check: before incompat=0x{:08x} nr={}'.format(v, (v>>2)&1))\n" +
			"    if v & 0x04:\n" +
			"        v2 = v & ~0x04\n" +
			"        fh.seek(1120)\n" +
			"        fh.write(struct.pack('<I', v2))\n" +
			"        fh.flush()\n" +
			"        os.fsync(fh.fileno())\n" +
			"    fh.seek(1120)\n" +
			"    verify = struct.unpack('<I', fh.read(4))[0]\n" +
			"    print('nr_check: after incompat=0x{:08x} nr={}'.format(verify, (verify>>2)&1))\n" +
			"    sys.exit(0 if not (verify & 0x04) else 2)\n")
	if putErr := f.runner.Put(verifyScriptPath, verifyScriptContent); putErr != nil {
		f.logger.Error(f.logTag, "Failed to write verify script: %s", putErr)
	} else {
		fixOut, fixRc, fixErr := f.runner.Execute("python3", verifyScriptPath, vmExt4)
		f.logger.Info(f.logTag, "nr_raw_fix rc=%d err=%v output: %s", fixRc, fixErr, fixOut)
		_, _, _ = f.runner.Execute("rm", "-f", verifyScriptPath)
	}
	_, _, _ = f.runner.Execute("sync")
	return driver.DomainDiskPaths{
		RootDisk:      vmExt4,
		EphemeralDisk: ephemeralDisk.ImagePath(),
	}, nil
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

// DefaultExecCommand is the real exec implementation. Tests may override ExecCommand.
var DefaultExecCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// ExecCommand is the package-level exec hook. Override in tests.
var ExecCommand = DefaultExecCommand

// addBlobstoreToEnv injects a local blobstore config into the agent env JSON.
// The CPI SDK ForVM factory doesn't include blobstore so the bootstrap agent
// can't find bosh-blobstore-<provider> without it.
func addBlobstoreToEnv(envBytes []byte) []byte {
	var m map[string]interface{}
	if err := json.Unmarshal(envBytes, &m); err != nil {
		return envBytes
	}
	m["blobstore"] = map[string]interface{}{
		"provider": "local",
		"options": map[string]interface{}{
			"blobstore_path": "/var/vcap/micro_bosh/data/cache",
		},
	}
	out, err := json.Marshal(m)
	if err != nil {
		return envBytes
	}
	return out
}

// isDirectorAgentEnv returns true when the agent env is for a bootstrap (create-env)
// director VM. Such VMs have the top-level "mbus" field pointing to the HTTPS mbus
// endpoint (https://mbus:...@IP:6868). Regular deploy VMs use nats:// instead.
func isDirectorAgentEnv(envBytes []byte) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(envBytes, &m); err != nil {
		return false
	}
	mbus, _ := m["mbus"].(string)
	return strings.HasPrefix(mbus, "https://")
}

// extractNetworkFromEnv returns the first non-empty static IP, gateway and
// CIDR prefix length from the agent env's networks map. The prefix is derived
// from the network's netmask (e.g. 255.255.255.0 -> 24); it falls back to 24
// when no usable netmask is present. Returns empty IP/gateway for dynamic networks.
func extractNetworkFromEnv(envBytes []byte) (ip, gateway string, prefix int) {
	prefix = 24
	var m map[string]interface{}
	if err := json.Unmarshal(envBytes, &m); err != nil {
		return "", "", prefix
	}
	networks, _ := m["networks"].(map[string]interface{})
	for _, v := range networks {
		net, _ := v.(map[string]interface{})
		if ipVal, _ := net["ip"].(string); ipVal != "" {
			gwVal, _ := net["gateway"].(string)
			if nmVal, _ := net["netmask"].(string); nmVal != "" {
				if p, ok := netmaskToPrefix(nmVal); ok {
					prefix = p
				}
			}
			return ipVal, gwVal, prefix
		}
	}
	return "", "", prefix
}

// netmaskToPrefix converts a dotted-quad netmask (e.g. "255.255.255.0") to a
// CIDR prefix length (e.g. 24). The ok return is false for malformed masks.
func netmaskToPrefix(netmask string) (int, bool) {
	ipMask := net.ParseIP(netmask)
	if ipMask == nil {
		return 0, false
	}
	v4 := ipMask.To4()
	if v4 == nil {
		return 0, false
	}
	ones, bits := net.IPv4Mask(v4[0], v4[1], v4[2], v4[3]).Size()
	if bits == 0 {
		return 0, false
	}
	return ones, true
}

// injectMbusCert injects a TLS cert into env.bosh.mbus.cert so the agent can
// start its HTTPS mbus listener. If MbusBootstrapSSL is configured in FactoryOpts
// (from cloud_provider.properties.mbus_bootstrap_ssl in the manifest), that cert
// is used directly so bosh create-env can verify it via cloud_provider.cert.
// Otherwise a self-signed cert with an IP SAN is generated as a fallback.
func (f Factory) injectMbusCert(envBytes []byte) []byte {
	// If a cert is already in the env (from bosh create-env), preserve it.
	var m map[string]interface{}
	if err := json.Unmarshal(envBytes, &m); err == nil {
		if env, _ := m["env"].(map[string]interface{}); env != nil {
			if bosh, _ := env["bosh"].(map[string]interface{}); bosh != nil {
				if mbus, _ := bosh["mbus"].(map[string]interface{}); mbus != nil {
					if cert, _ := mbus["cert"].(map[string]interface{}); cert != nil {
						if ca, _ := cert["ca"].(string); ca != "" {
							return envBytes
						}
					}
				}
			}
		}
	}

	// Use the manifest-provided mbus_bootstrap_ssl cert when available.
	ssl := f.opts.MbusBootstrapSSL
	if ssl.CA != "" && ssl.Certificate != "" && ssl.PrivateKey != "" {
		return injectCert(envBytes, ssl.CA, ssl.Certificate, ssl.PrivateKey)
	}

	// Fallback: generate a self-signed cert with the director's IP SAN.
	ip, _, _ := extractNetworkFromEnv(envBytes)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return envBytes
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "bosh-bootstrap"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		tmpl.IPAddresses = []net.IP{parsed}
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return envBytes
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return envBytes
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return injectCert(envBytes, string(certPEM), string(certPEM), string(keyPEM))
}

func injectCert(envBytes []byte, ca, cert, key string) []byte {
	var m map[string]interface{}
	if err := json.Unmarshal(envBytes, &m); err != nil {
		return envBytes
	}
	env, _ := m["env"].(map[string]interface{})
	if env == nil {
		env = map[string]interface{}{}
	}
	bosh, _ := env["bosh"].(map[string]interface{})
	if bosh == nil {
		bosh = map[string]interface{}{}
	}
	mbus, _ := bosh["mbus"].(map[string]interface{})
	if mbus == nil {
		mbus = map[string]interface{}{}
	}
	mbus["cert"] = map[string]interface{}{
		"ca":          ca,
		"certificate": cert,
		"private_key": key,
	}
	bosh["mbus"] = mbus
	env["bosh"] = bosh
	m["env"] = env
	out, err := json.Marshal(m)
	if err != nil {
		return envBytes
	}
	return out
}

func (f Factory) Find(cid apiv1.VMCID) (VM, error) {
	return f.newVM(cid), nil
}

// cpiWrapperScript returns the shell wrapper that is installed as
// /var/vcap/packages/libvirt_cpi/bin/cpi in the director VM rootfs.
// At each CPI invocation it merges /var/vcap/bosh/cpi-inject.json into
// the config file, then execs the real binary (renamed to cpi.real).
// This is immune to bosh-agent re-rendering cpi.json after initial boot.
func cpiWrapperScript() string {
	return "#!/bin/sh\n" +
		"INJ=/var/vcap/bosh/cpi-inject.json\n" +
		"if [ -f \"$INJ\" ]; then\n" +
		"  PREV=''; CFG=''\n" +
		"  for a in \"$@\"; do\n" +
		"    [ \"$PREV\" = '-configPath' ] && CFG=$a\n" +
		"    PREV=$a\n" +
		"  done\n" +
		"  if [ -n \"$CFG\" ] && [ -f \"$CFG\" ]; then\n" +
		"    TMP=$(mktemp /tmp/cpi-cfg.XXXXXX.json)\n" +
		"    python3 -c \"import json,sys; a=json.load(open(sys.argv[1])); b=json.load(open(sys.argv[2])); a.update(b); json.dump(a,sys.stdout)\" \"$CFG\" \"$INJ\" > \"$TMP\" 2>/dev/null\n" +
		"    if [ -s \"$TMP\" ]; then\n" +
		"      exec /var/vcap/packages/libvirt_cpi/bin/cpi.real -configPath \"$TMP\"\n" +
		"    fi\n" +
		"    rm -f \"$TMP\"\n" +
		"  fi\n" +
		"fi\n" +
		"exec /var/vcap/packages/libvirt_cpi/bin/cpi.real \"$@\"\n"
}

// shellEscape single-quote-escapes s for embedding in a shell printf '%s' '...'
// by replacing each ' with '\”.
func shellEscape(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}
