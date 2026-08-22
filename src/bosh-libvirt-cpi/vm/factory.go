package vm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

	// For QEMU kernel-boot (ext4), the ephemeral disk is attached as /dev/vdb inside
	// the VM. Override the hint so bosh-agent finds the actual device instead of the
	// host file path (which doesn't exist inside the VM, causing fallback to root disk).
	if f.domBuilder.DiskImageFormat() == "ext4" {
		initialAgentEnv.AttachEphemeralDisk(apiv1.NewDiskHintFromString("/dev/vdb"))
	}

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
	// mount the stemcell and inject per-VM agent env + init wrapper.
	if f.domBuilder.DiskImageFormat() == "dir" {
		vmRootfs := filepath.Join(f.opts.DirPath, vmID, "rootfs")
		out, copyErr := execCommand("cp", "-a", stemcell.ImagePath()+"/.", vmRootfs)
		if copyErr != nil {
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapErrorf(copyErr, "Copying stemcell rootfs to VM dir: %s", string(out))
		}

		// Remove stale supervise dirs from stemcell copy so runsv starts cleanly
		if svcs, _ := os.ReadDir(vmRootfs + "/etc/sv"); svcs != nil {
			for _, svc := range svcs {
				_ = os.RemoveAll(vmRootfs + "/etc/sv/" + svc.Name() + "/supervise")
			}
		}

		envBytes, err := initialAgentEnv.AsBytes()
		if err != nil {
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapError(err, "Marshalling agent env for rootfs injection")
		}
		boshDir := vmRootfs + "/var/vcap/bosh"
		agentEnvBytes := f.injectMbusCert(addBlobstoreToEnv(envBytes))
		if mkErr := os.MkdirAll(boshDir, 0755); mkErr == nil {
			_ = os.WriteFile(boshDir+"/warden-cpi-agent-env.json", agentEnvBytes, 0644)
		}
		// Write a stub sv wrapper so the agent's "sv start monit" succeeds
		// even when runsv can't acquire locks in restricted containers.
		svStub := "#!/bin/sh\n" +
			"# Stub: intercept all sv verbs so the monit stub is never killed.\n" +
			"case \"$1\" in\n" +
			"  start)       echo \"ok: run: $2: (pid 0) 1s\"; exit 0 ;;\n" +
			"  stop)        echo \"ok: down: $2: 0s\";        exit 0 ;;\n" +
			"  kill|force-stop) echo \"ok: down: $2: 0s\";   exit 0 ;;\n" +
			"  status)      echo \"run: $2: (pid 0) 1s\";    exit 0 ;;\n" +
			"esac\n" +
			"exec /usr/bin/sv \"$@\"\n"
		_ = os.WriteFile(vmRootfs+"/usr/local/bin/sv", []byte(svStub), 0755)
		// Symlink bosh tools and system tools into /usr/local/bin so the agent
		// finds them via exec.Command regardless of the inherited PATH.
		_ = os.MkdirAll(vmRootfs+"/usr/local/bin", 0755)
		for _, srcDir := range []string{
			vmRootfs + "/var/vcap/bosh/bin",
			vmRootfs + "/usr/sbin",
			vmRootfs + "/sbin",
		} {
			entries, _ := os.ReadDir(srcDir)
			for _, e := range entries {
				dst := vmRootfs + "/usr/local/bin/" + e.Name()
				rel := strings.TrimPrefix(srcDir, vmRootfs)
				src := rel + "/" + e.Name()
				_ = os.Remove(dst)
				_ = os.Symlink(src, dst)
			}
		}

		// Write LXC init wrapper — configure networking then exec bosh-agent.
		// sv stub handles "sv start monit" without needing runsv.
		// Extract static IP/gateway from the agent env and bake them in so the
		// container has network connectivity before bosh-agent starts.
		staticIP, staticGW := extractNetworkFromEnv(agentEnvBytes)
		// monit stub: persistent HTTP server on port 2822 returning valid monit XML.
		// bosh-agent parses incarnation as an XML *attribute* on the root <monit> element
		// (xml:"incarnation,attr" in status.go) and hits /_status2?format=xml.
		// Real monit uses time(...) as incarnation ID. We do the same: int(time.time())
		// advances every second, so incarnationChanged() returns true after one
		// DelayBetweenCheckTries sleep without needing any signal from sv kill/start.
		monitStub := "python3 -c \"\n" +
			"import http.server, socketserver, sys, time\n" +
			"socketserver.TCPServer.allow_reuse_address = True\n" +
			"def xml():\n" +
			"  inc = str(int(time.time()))\n" +
			"  return ('<monit id=\\\"stub\\\" incarnation=\\\"' + inc + '\\\" version=\\\"5\\\"><services/><servicegroups/></monit>').encode()\n" +
			"class H(http.server.BaseHTTPRequestHandler):\n" +
			"  def do_GET(self):\n" +
			"    body = xml()\n" +
			"    self.send_response(200)\n" +
			"    self.send_header('Content-Type','text/xml')\n" +
			"    self.send_header('Content-Length', str(len(body)))\n" +
			"    self.end_headers()\n" +
			"    self.wfile.write(body)\n" +
			"  def do_POST(self):\n" +
			"    self.rfile.read(int(self.headers.get('Content-Length','0')))\n" +
			"    self.send_response(200)\n" +
			"    self.send_header('Content-Length','0')\n" +
			"    self.end_headers()\n" +
			"  def log_message(self, *a): pass\n" +
			"try:\n" +
			"  srv = socketserver.TCPServer(('127.0.0.1',2822),H)\n" +
			"  sys.stderr.write('monit stub: ready\\\\n')\n" +
			"  sys.stderr.flush()\n" +
			"  srv.serve_forever()\n" +
			"except Exception as e:\n" +
			"  sys.stderr.write('monit stub error: %s\\\\n' % str(e))\n" +
			"  sys.stderr.flush()\n" +
			"\" >/tmp/monit-stub.log 2>&1 &\n" +
			"for i in $(seq 1 30); do\n" +
			"  (echo > /dev/tcp/127.0.0.1/2822) 2>/dev/null && break\n" +
			"  sleep 0.2\n" +
			"done\n"
		var lxcInitScript string
		if staticIP != "" {
			lxcInitScript = "#!/bin/sh\n" +
				"export PATH=/var/vcap/bosh/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n" +
				"exec >>/var/vcap/bosh/log/bosh-agent-init.log 2>&1\n" +
				"ip link set lo up 2>/dev/null || true\n" +
				"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}' | sed 's/@.*//')\n" +
				"if [ -n \"$IFACE\" ]; then\n" +
				"  ip link set \"$IFACE\" up\n" +
				"  ip addr add " + staticIP + "/24 dev \"$IFACE\" 2>/dev/null || true\n" +
				"  ip route add default via " + staticGW + " dev \"$IFACE\" 2>/dev/null || true\n" +
				"  # Gratuitous ARP so the host bridge learns our MAC/IP immediately.\n" +
				"  arping -c 3 -U -I \"$IFACE\" " + staticIP + " 2>/dev/null || true\n" +
				"fi\n" +
				monitStub +
				"exec /var/vcap/bosh/bin/bosh-agent -C /var/vcap/bosh/agent.json -P ubuntu\n"
		} else {
			lxcInitScript = "#!/bin/sh\n" +
				"export PATH=/var/vcap/bosh/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n" +
				"exec >>/var/vcap/bosh/log/bosh-agent-init.log 2>&1\n" +
				"ip link set lo up 2>/dev/null || true\n" +
				"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}')\n" +
				"if [ -n \"$IFACE\" ]; then\n" +
				"  ip link set \"$IFACE\" up\n" +
				"  dhclient -v \"$IFACE\" 2>/tmp/dhclient.log || true\n" +
				"fi\n" +
				monitStub +
				"exec /var/vcap/bosh/bin/bosh-agent -C /var/vcap/bosh/agent.json -P ubuntu\n"
		}
		_ = os.WriteFile(vmRootfs+"/bosh-lxc-init", []byte(lxcInitScript), 0755)

		if vmProps.Kernel != "" {
			initScript := "#!/bin/sh\n" +
				"export PATH=/var/vcap/bosh/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n" +
				"# Mount essential pseudo-filesystems\n" +
				"mount -t proc proc /proc 2>/dev/null || true\n" +
				"mount -t sysfs sysfs /sys 2>/dev/null || true\n" +
				"mount -t devtmpfs devtmpfs /dev 2>/dev/null || true\n" +
				"# Remove stale supervise locks before starting runsv\n" +
				"rm -rf /etc/sv/*/supervise\n" +
				"# Start runsv directly for each service\n" +
				"for svc in /etc/sv/*/; do\n" +
				"  [ -d \"$svc\" ] && runsv \"$svc\" &\n" +
				"done\n" +
				"# Wait for monit supervise dir\n" +
				"for i in $(seq 1 30); do\n" +
				"  [ -d /etc/sv/monit/supervise ] && break\n" +
				"  sleep 1\n" +
				"done\n" +
				"# Bring up network via DHCP\n" +
				"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}')\n" +
				"if [ -n \"$IFACE\" ]; then\n" +
				"  ip link set \"$IFACE\" up\n" +
				"  /usr/sbin/dhclient -v \"$IFACE\" 2>/tmp/dhclient.log || true\n" +
				"fi\n" +
				"exec /var/vcap/bosh/bin/bosh-agent -C /var/vcap/bosh/agent.json -P ubuntu\n"
			_ = os.WriteFile(vmRootfs+"/bosh-init", []byte(initScript), 0755)
		}

		// Override disk paths to use the per-VM rootfs copy.
		disks = driver.DomainDiskPaths{
			RootDisk:      vmRootfs,
			EphemeralDisk: ephemeralDisk.ImagePath(),
		}
	} else if f.domBuilder.DiskImageFormat() == "ext4" {
		// QEMU kernel-boot: copy the stemcell ext4 image per-VM, mount it,
		// inject warden-cpi-agent-env.json and /bosh-init, then unmount.
		vmExt4 := filepath.Join(f.opts.DirPath, vmID, "rootfs.img")
		if err := os.MkdirAll(filepath.Join(f.opts.DirPath, vmID), 0755); err != nil {
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapError(err, "Creating VM dir")
		}
		if out, err := execCommand("cp", stemcell.ImagePath(), vmExt4); err != nil {
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapErrorf(err, "Copying stemcell ext4 for VM: %s", string(out))
		}
		// Mount, inject, unmount
		mntDir := vmExt4 + ".mnt"
		if err := os.MkdirAll(mntDir, 0755); err == nil {
			if _, err := execCommand("mount", "-o", "loop", vmExt4, mntDir); err == nil {
				// Remove stale supervise dirs from stemcell while we have write access
				if svcs, _ := os.ReadDir(mntDir + "/etc/sv"); svcs != nil {
					for _, svc := range svcs {
						_ = os.RemoveAll(mntDir + "/etc/sv/" + svc.Name() + "/supervise")
					}
				}
				envBytes, _ := initialAgentEnv.AsBytes()
				boshDir := mntDir + "/var/vcap/bosh"
				if mkErr := os.MkdirAll(boshDir, 0755); mkErr == nil {
					agentEnvBytes2 := f.injectMbusCert(addBlobstoreToEnv(envBytes))
					_ = os.WriteFile(boshDir+"/warden-cpi-agent-env.json", agentEnvBytes2, 0644)
					// Override agent.json to enable ephemeral disk setup.
					// The warden-boshlite stemcell ships with SkipDiskSetup:true which
					// prevents bosh-agent from formatting and mounting /dev/vdb.
					// With SkipDiskSetup:false, bosh-agent uses the hint from
					// warden-cpi-agent-env.json ("disks.ephemeral": "/dev/vdb") to
					// format and mount the 65GB virtio disk as /var/vcap/data.
					agentJSON := []byte(`{
  "Platform": {
    "Linux": {
      "UseDefaultTmpDir": true,
      "UsePreformattedPersistentDisk": true,
      "BindMountPersistentDisk": true,
      "SkipDiskSetup": false
    }
  },
  "Infrastructure": {
    "Settings": {
      "Sources": [{"Type": "File", "SettingsPath": "/var/vcap/bosh/warden-cpi-agent-env.json"}],
      "UseServerName": false,
      "UseRegistry": false
    }
  }
}`)
					_ = os.WriteFile(boshDir+"/agent.json", agentJSON, 0644)
				}
				// Symlink bosh tools into /usr/local/bin
				_ = os.MkdirAll(mntDir+"/usr/local/bin", 0755)
				boshBins, _ := os.ReadDir(mntDir + "/var/vcap/bosh/bin")
				for _, b := range boshBins {
					dst := mntDir + "/usr/local/bin/" + b.Name()
					src := "/var/vcap/bosh/bin/" + b.Name()
					_ = os.Remove(dst)
					_ = os.Symlink(src, dst)
				}
				initScript := "#!/bin/sh\n" +
					"export PATH=/var/vcap/bosh/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n" +
					"mount -t proc proc /proc 2>/dev/null || true\n" +
					"mount -t sysfs sysfs /sys 2>/dev/null || true\n" +
					"mount -t devtmpfs devtmpfs /dev 2>/dev/null || true\n" +
					"ip link set lo up 2>/dev/null || true\n" +
					"# Bring up network via DHCP\n" +
					"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}')\n" +
					"if [ -n \"$IFACE\" ]; then\n" +
					"  ip link set \"$IFACE\" up\n" +
					"  /usr/sbin/dhclient -v \"$IFACE\" 2>/tmp/dhclient.log || true\n" +
					"fi\n" +
					"python3 -c \"\n" +
					"import http.server, socketserver, sys, time\n" +
					"socketserver.TCPServer.allow_reuse_address = True\n" +
					"def xml():\n" +
					"  inc = str(int(time.time()))\n" +
					"  return ('<monit id=\\\"stub\\\" incarnation=\\\"' + inc + '\\\" version=\\\"5\\\"><services/><servicegroups/></monit>').encode()\n" +
					"class H(http.server.BaseHTTPRequestHandler):\n" +
					"  def do_GET(self):\n" +
					"    body = xml()\n" +
					"    self.send_response(200)\n" +
					"    self.send_header('Content-Type','text/xml')\n" +
					"    self.send_header('Content-Length', str(len(body)))\n" +
					"    self.end_headers()\n" +
					"    self.wfile.write(body)\n" +
					"  def do_POST(self):\n" +
					"    self.rfile.read(int(self.headers.get('Content-Length','0')))\n" +
					"    self.send_response(200)\n" +
					"    self.send_header('Content-Length','0')\n" +
					"    self.end_headers()\n" +
					"  def log_message(self, *a): pass\n" +
					"try:\n" +
					"  srv = socketserver.TCPServer(('127.0.0.1',2822),H)\n" +
					"  sys.stderr.write('monit stub: ready\\\\n')\n" +
					"  sys.stderr.flush()\n" +
					"  srv.serve_forever()\n" +
					"except Exception as e:\n" +
					"  sys.stderr.write('monit stub error: %s\\\\n' % str(e))\n" +
					"  sys.stderr.flush()\n" +
					"\" >/tmp/monit-stub.log 2>&1 &\n" +
					"for i in $(seq 1 30); do\n" +
					"  (echo > /dev/tcp/127.0.0.1/2822) 2>/dev/null && break\n" +
					"  sleep 0.2\n" +
					"done\n" +
					"# Log disk usage periodically so we can see what fills up\n" +
					"( while true; do echo \"=== df /var/vcap/data ===\"; df -h /var/vcap/data 2>/dev/null; sleep 60; done ) &\n" +
					"exec /var/vcap/bosh/bin/bosh-agent -C /var/vcap/bosh/agent.json -P ubuntu\n"
				_ = os.WriteFile(mntDir+"/bosh-init", []byte(initScript), 0755)
				// Write sv stub at host-side mount so it always takes priority over /usr/bin/sv
				_ = os.MkdirAll(mntDir+"/usr/local/bin", 0755)
				ext4SvStub := "#!/bin/sh\ncase \"$1\" in\n  start)       echo \"ok: run: $2: (pid 0) 1s\"; exit 0 ;;\n  stop)        echo \"ok: down: $2: 0s\";        exit 0 ;;\n  kill|force-stop) echo \"ok: down: $2: 0s\";   exit 0 ;;\n  status)      echo \"run: $2: (pid 0) 1s\";    exit 0 ;;\nesac\nexec /usr/bin/sv \"$@\"\n"
				_ = os.WriteFile(mntDir+"/usr/local/bin/sv", []byte(ext4SvStub), 0755)
				_, _ = execCommand("umount", mntDir)
			}
			_ = os.RemoveAll(mntDir)
		}
		disks = driver.DomainDiskPaths{
			RootDisk:      vmExt4,
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

// extractNetworkFromEnv returns the first non-empty static IP and gateway
// from the agent env's networks map. Returns empty strings for dynamic networks.
func extractNetworkFromEnv(envBytes []byte) (ip, gateway string) {
	var m map[string]interface{}
	if err := json.Unmarshal(envBytes, &m); err != nil {
		return "", ""
	}
	networks, _ := m["networks"].(map[string]interface{})
	for _, v := range networks {
		net, _ := v.(map[string]interface{})
		if ipVal, _ := net["ip"].(string); ipVal != "" {
			gwVal, _ := net["gateway"].(string)
			return ipVal, gwVal
		}
	}
	return "", ""
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
	ip, _ := extractNetworkFromEnv(envBytes)
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
	bosh["mbus"] = map[string]interface{}{
		"cert": map[string]interface{}{
			"ca":          ca,
			"certificate": cert,
			"private_key": key,
		},
	}
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
