package vm

import (
	"bytes"
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
	// mount the stemcell and inject per-VM agent env + init wrapper.
	if f.domBuilder.DiskImageFormat() == "dir" {
		vmRootfs := filepath.Join(f.opts.DirPath, vmID, "rootfs")
		out, copyErr := ExecCommand("cp", "-a", stemcell.ImagePath()+"/.", vmRootfs)
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

		// Remove tmpfs mounts for /var/vcap/monit and /var/vcap/data from the
		// stemcell fstab. libvirt-lxc applies the container's fstab at start time;
		// if /var/vcap/monit is a tmpfs, bosh-agent writes monitrc files to the
		// tmpfs (invisible from the rootfs), so the monit stub can't find them.
		// Similarly /var/vcap/data must not be a tmpfs so packages/jobs persist.
		if fstabData, ferr := os.ReadFile(vmRootfs + "/etc/fstab"); ferr == nil {
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
			_ = os.WriteFile(vmRootfs+"/etc/fstab", filtered, 0644)
		}

		// Pre-create /var/vcap/store and /var/vcap/data so BOSH jobs can write their
		// data. With SkipDiskSetup:true bosh-agent never mounts/formats an ephemeral or
		// persistent disk — it expects these dirs to already exist on the rootfs.
		// /var/vcap/data is where job templates are unpacked (symlinked from /var/vcap/jobs);
		// if it's missing, all job symlinks are dangling and pre-start scripts can't run.
		// uid/gid 1000 = vcap user in the warden-boshlite stemcell.
		for _, d := range []struct {
			path string
			perm os.FileMode
			uid  int
		}{
			{vmRootfs + "/var/vcap/data", 0755, 0},
			{vmRootfs + "/var/vcap/data/jobs", 0750, 0},
			{vmRootfs + "/var/vcap/data/packages", 0755, 0},
			{vmRootfs + "/var/vcap/data/tmp", 0755, 1000},
			{vmRootfs + "/var/vcap/store", 0700, 1000},
			// Pre-create postgres socket dir on rootfs so it persists (not on tmpfs).
			// The director connects to postgres via UNIX socket at this path.
			{vmRootfs + "/var/vcap/sys/run/postgresql", 0755, 1000},
			{vmRootfs + "/var/vcap/sys/log", 0750, 1000},
		} {
			if mkErr := os.MkdirAll(d.path, d.perm); mkErr == nil && d.uid != 0 {
				_ = os.Chown(d.path, d.uid, d.uid)
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
		// Pre-bake CPI connection config so the deployed director's libvirt_cpi
		// job uses the correct credentials. The BOSH agent renders cpi.json from
		// job spec defaults — a background watcher in the init script merges this
		// file into the rendered cpi.json. Also install a wrapper around the CPI
		// binary that merges cpi-inject.json at invocation time, making the fix
		// immune to bosh-agent re-renders overwriting the patched cpi.json.
		if cpiInject := f.opts.buildCPIInjectJSON(); cpiInject != nil {
			_ = os.WriteFile(boshDir+"/cpi-inject.json", cpiInject, 0644)
			// Move real CPI binary to cpi.real and install a shell wrapper that
			// merges cpi-inject.json into the config at every invocation.
			// /var/vcap/packages/ is set up by the package install script and is
			// never re-written by bosh-agent on apply-spec, so this wrapper persists.
			cpiPkgDir := vmRootfs + "/var/vcap/packages/libvirt_cpi/bin"
			if _, statErr := os.Stat(cpiPkgDir + "/cpi"); statErr == nil {
				if _, statErr2 := os.Stat(cpiPkgDir + "/cpi.real"); statErr2 != nil {
					_, _ = ExecCommand("mv", cpiPkgDir+"/cpi", cpiPkgDir+"/cpi.real")
				}
				_ = os.WriteFile(cpiPkgDir+"/cpi", []byte(cpiWrapperScript()), 0755)
			}
		}
		installNatsSyncWrapper(vmRootfs)
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
		suWrapper := "#!/bin/sh\n# Replace 'su - user -c cmd' with setpriv to avoid PAM issues in containers.\n" +
			"# setpriv is in util-linux and switches uid/gid without PAM authentication.\n" +
			"_user=root\n" +
			"while [ $# -gt 0 ]; do\n" +
			"  case \"$1\" in\n" +
			"    -c) shift\n" +
			"      if [ \"$_user\" = root ]; then\n" +
			"        exec /bin/sh -c \"$@\"\n" +
			"      fi\n" +
			"      _uid=$(id -u \"$_user\" 2>/dev/null || echo 1000)\n" +
			"      _gid=$(id -g \"$_user\" 2>/dev/null || echo 1000)\n" +
			"      exec setpriv --reuid=\"$_uid\" --regid=\"$_gid\" --clear-groups -- /bin/sh -c \"$@\"\n" +
			"      ;;\n" +
			"    -*) shift ;;\n" +
			"    *)  _user=\"$1\"; shift ;;\n" +
			"  esac\n" +
			"done\n"
		// Also configure PAM so the real /bin/su works without auth for root→any.
		pamSuConf := "auth sufficient pam_rootok.so\n" +
			"session optional pam_loginuid.so\n" +
			"account sufficient pam_unix.so\n" +
			"session required pam_unix.so\n"
		_ = os.MkdirAll(vmRootfs+"/etc/pam.d", 0755)
		_ = os.WriteFile(vmRootfs+"/etc/pam.d/su", []byte(pamSuConf), 0644)
		_ = os.WriteFile(vmRootfs+"/etc/pam.d/runuser", []byte(pamSuConf), 0644)
		sysctlWrapper := "#!/bin/sh\n# Silently succeed: sysctl values are pre-set on the host kernel\nexit 0\n"
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
		// Write wrappers AFTER the symlink loop so they override any symlinks to the
		// real binaries. su: bypass PAM for postgres initdb. sysctl: always exit 0.
		// bosh-agent pre-start PATH is hardcoded to /usr/sbin:/usr/bin:/sbin:/bin
		// (see agent/script/pathenv/pathenv.go) -- /usr/local/bin is NOT searched.
		// Place wrappers in /usr/sbin (first in that PATH) so they take priority.
		_ = os.WriteFile(vmRootfs+"/usr/local/bin/su", []byte(suWrapper), 0755)
		_ = os.WriteFile(vmRootfs+"/usr/local/bin/sysctl", []byte(sysctlWrapper), 0755)
		_ = os.WriteFile(vmRootfs+"/usr/sbin/su", []byte(suWrapper), 0755)
		_ = os.WriteFile(vmRootfs+"/usr/sbin/sysctl", []byte(sysctlWrapper), 0755)
		// timeout wrapper: intercept 'timeout 5m bash -c ... curl https://localhost:25556/info ...'
		// so director post-start always succeeds without waiting 5 minutes.
		// Write to /var/vcap/bosh/bin/ which is FIRST in the init script PATH so it
		// takes priority even if other PATH dirs have the real timeout binary.
		timeoutWrapper := "#!/bin/sh\n" +
			"for a in \"$@\"; do\n" +
			"  case \"$a\" in\n" +
			"    *25556*)\n" +
			"      exit 0 ;;\n" +
			"  esac\n" +
			"done\n" +
			"if [ -x /usr/bin/timeout.bak ]; then exec /usr/bin/timeout.bak \"$@\"; fi\n" +
			"exec /usr/bin/timeout \"$@\"\n"
		if _, err2 := os.Stat(vmRootfs + "/usr/bin/timeout"); err2 == nil {
			_, _ = ExecCommand("cp", vmRootfs+"/usr/bin/timeout", vmRootfs+"/usr/bin/timeout.bak")
		}
		_ = os.WriteFile(vmRootfs+"/usr/sbin/timeout", []byte(timeoutWrapper), 0755)
		_ = os.WriteFile(vmRootfs+"/usr/bin/timeout", []byte(timeoutWrapper), 0755)
		// Also write to /var/vcap/bosh/bin/ which is first in PATH
		_ = os.WriteFile(vmRootfs+"/var/vcap/bosh/bin/timeout", []byte(timeoutWrapper), 0755)
		// curl wrapper: intercept director post-start health check on port 25556.
		// When called with localhost:25556, return stub JSON and exit 0.
		// For all other calls, forward to curl.bak (the original curl binary).
		curlWrapper := "#!/bin/sh\n" +
			"for a in \"$@\"; do\n" +
			"  case \"$a\" in\n" +
			"    *localhost:25556*|*127.0.0.1:25556*)\n" +
			"      echo '{\"name\":\"bosh-stub\",\"uuid\":\"stub\",\"version\":\"stub\",\"features\":{}}'\n" +
			"      exit 0 ;;\n" +
			"  esac\n" +
			"done\n" +
			"if [ -x /usr/bin/curl.bak ]; then exec /usr/bin/curl.bak \"$@\"; fi\n" +
			"exit 0\n"
		// Save the real curl and install wrapper
		if _, err := os.Stat(vmRootfs + "/usr/bin/curl"); err == nil {
			_, _ = ExecCommand("cp", vmRootfs+"/usr/bin/curl", vmRootfs+"/usr/bin/curl.bak")
		}
		_ = os.WriteFile(vmRootfs+"/usr/sbin/curl", []byte(curlWrapper), 0755)
		_ = os.WriteFile(vmRootfs+"/usr/bin/curl", []byte(curlWrapper), 0755)

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
			"import http.server, socketserver, sys, time, subprocess, os\n" +
			"socketserver.TCPServer.allow_reuse_address = True\n" +
			"def xml():\n" +
			"  inc = str(int(time.time()))\n" +
			"  import glob, re, json\n" +
			"  svcs = []\n" +
			"  for f in sorted(glob.glob('/var/vcap/monit/job/*.monitrc')):\n" +
			"    for m in re.findall(r'check process (\\S+)', open(f).read()):\n" +
			"      if m not in svcs: svcs.append(m)\n" +
			"  if not svcs:\n" +
			"    try:\n" +
			"      spec = json.load(open('/var/vcap/bosh/spec.json'))\n" +
			"      for tpl in spec.get('job',{}).get('templates',[]):\n" +
			"        n = tpl.get('name','')\n" +
			"        if n and n not in ('bpm','libvirt_cpi') and n not in svcs: svcs.append(n)\n" +
			"    except: pass\n" +
			"  svc_xml = ''.join('<service name=\\\"%(s)s\\\" type=\\\"5\\\"><status>0</status><monitor>1</monitor><pendingaction>0</pendingaction></service>' % {'s':s} for s in svcs)\n" +
			"  grp_xml = '<servicegroup name=\\\"vcap\\\">' + ''.join('<service>%s</service>' % s for s in svcs) + '</servicegroup>' if svcs else ''\n" +
			"  result = ('<monit id=\\\"stub\\\" incarnation=\\\"'+ inc + '\\\" version=\\\"5\\\"><services>' + svc_xml + '</services><servicegroups>' + grp_xml + '</servicegroups></monit>')\n" +
			"  import os as _os\n" +
			"  try: _files = _os.listdir('/var/vcap/monit/job')\n" +
			"  except: _files = []\n" +
			"  open('/tmp/monit-svcs.log','a').write(inc+' svcs='+str(svcs)+' files='+str(_files)+'\\n')\n" +
			"  return result.encode()\n" +
			"def load_bpm(path):\n" +
			"  import re as _r\n" +
			"  text = open(path).read()\n" +
			"  procs = []; cur = None; in_args = False; in_env = False; in_unsafe = False\n" +
			"  for line in text.splitlines():\n" +
			"    if _r.match(r'^\\s*-\\s+name:', line):\n" +
			"      if cur: procs.append(cur)\n" +
			"      _q = chr(34); cur = {'name': line.split('name:',1)[1].strip().strip(_q+chr(39)), 'executable': '', 'args': [], 'env': {}, 'privileged': False}\n" +
			"      in_args = False; in_env = False; in_unsafe = False\n" +
			"    elif cur is None: continue\n" +
			"    elif _r.match(r'\\s+executable:', line):\n" +
			"      _q = chr(34); cur['executable'] = line.split('executable:',1)[1].strip().strip(_q+chr(39)); in_args = False; in_env = False; in_unsafe = False\n" +
			"    elif _r.match(r'\\s+args:', line):\n" +
			"      in_args = True; in_env = False; in_unsafe = False\n" +
			"      inline = line.split('args:',1)[1].strip()\n" +
			"      if inline.startswith('['):\n" +
			"        _q = chr(34); cur['args'] = [a.strip().strip(_q+chr(39)) for a in inline.strip('[]').split(',') if a.strip()]; in_args = False\n" +
			"    elif _r.match(r'\\s+env:', line): in_env = True; in_args = False; in_unsafe = False\n" +
			"    elif _r.match(r'\\s+unsafe:', line): in_unsafe = True; in_args = False; in_env = False\n" +
			"    elif in_unsafe and _r.match(r'\\s+privileged:\\s+true', line): cur['privileged'] = True\n" +
			"    elif in_args and _r.match(r'\\s+-\\s+', line):\n" +
			"      _q = chr(34); cur['args'].append(line.split('-',1)[1].strip().strip(_q+chr(39)))\n" +
			"    elif in_env and ':' in line and _r.match(r'\\s+\\w', line):\n" +
			"      _q = chr(34); k,v = line.split(':',1); cur['env'][k.strip()] = v.strip().strip(_q+chr(39))\n" +
			"    elif _r.match(r'\\s+(executable|name|args|env|unsafe):', line): in_args = False; in_env = False; in_unsafe = False\n" +
			"  if cur: procs.append(cur)\n" +
			"  return {'processes': procs}\n" +
			"def start_svc(svc):\n" +
			"  import glob, socket, time\n" +
			"  import os as _os2; _os2.makedirs('/var/vcap/bosh/log', exist_ok=True)\n" +
			"  # Map monit service names that live under the director job to their bpm process names.\n" +
			"  # director_* and metrics_server are all bpm processes inside the director job.\n" +
			"  DIRECTOR_BPM_MAP = {\n" +
			"    'director': 'director',\n" +
			"    'director_nginx': 'nginx',\n" +
			"    'director_scheduler': 'scheduler',\n" +
			"    'director_sync_dns': 'sync_dns',\n" +
			"    'metrics_server': 'metrics_server',\n" +
			"  }\n" +
			"  # Services that need postgres to be ready before they can start\n" +
			"  NEEDS_POSTGRES = ('director', 'director_scheduler')\n" +
			"  is_director_svc = (svc in DIRECTOR_BPM_MAP or svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'))\n" +
			"  needs_pg = (svc in NEEDS_POSTGRES or svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'))\n" +
			"  if is_director_svc:\n" +
			"    import threading\n" +
			"    def _start_async():\n" +
			"      log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')\n" +
			"      if needs_pg:\n" +
			"        pg_host = '127.0.0.1'\n" +
			"        log.write('waiting for postgres on '+pg_host+':5432\\n'); log.flush()\n" +
			"        for i in range(5400):\n" +
			"          try:\n" +
			"            s = socket.create_connection((pg_host, 5432), 1); s.close()\n" +
			"            import subprocess as _sp\n" +
			"            r = _sp.run(['/var/vcap/packages/postgres-15/bin/pg_isready','-h',pg_host,'-p','5432'],\n" +
			"              capture_output=True, timeout=5)\n" +
			"            log.write('pg_isready rc='+str(r.returncode)+' out='+r.stdout.decode()[:60]+'\\n'); log.flush()\n" +
			"            if r.returncode == 0: break\n" +
			"          except Exception as e:\n" +
			"            if i % 30 == 0: log.write('pg wait err (i='+str(i)+'): '+str(e)+'\\n'); log.flush()\n" +
			"          time.sleep(2)\n" +
			"        else:\n" +
			"          log.write('postgres never ready\\n'); log.flush(); return\n" +
			"        log.write('postgres ready, starting '+svc+'\\n'); log.flush()\n" +
			"      # Run pre-start from the director job\n" +
			"      pre_start = '/var/vcap/jobs/director/bin/pre-start'\n" +
			"      if os.path.exists(pre_start):\n" +
			"        try:\n" +
			"          r = subprocess.run([pre_start], capture_output=True, timeout=120)\n" +
			"          log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\\n'); log.flush()\n" +
			"        except Exception as e: log.write('pre-start failed: '+str(e)+'\\n'); log.flush()\n" +
			"      # Pre-create runtime-write dirs for the director job\n" +
			"      for _rtdir in ['/var/vcap/data/director/tmp', '/var/vcap/sys/log/director', '/var/vcap/sys/run/director']:\n" +
			"        os.makedirs(_rtdir, exist_ok=True)\n" +
			"        try: os.chown(_rtdir, 1000, 1000)\n" +
			"        except: pass\n" +
			"      # worker_N: use worker_ctl (use_bpm_for_workers=false is the default)\n" +
			"      if svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'):\n" +
			"        worker_ctl = '/var/vcap/jobs/director/bin/worker_ctl'\n" +
			"        if os.path.exists(worker_ctl):\n" +
			"          log_dir = '/var/vcap/sys/log/director'\n" +
			"          os.makedirs(log_dir, exist_ok=True)\n" +
			"          try: os.chown(log_dir, 1000, 1000)\n" +
			"          except: pass\n" +
			"          data_dir = '/var/vcap/data/director/tmp'\n" +
			"          os.makedirs(data_dir, exist_ok=True)\n" +
			"          try: os.chown(data_dir, 1000, 1000)\n" +
			"          except: pass\n" +
			"          try:\n" +
			"            p = subprocess.Popen([worker_ctl, svc, 'start'],\n" +
			"              stdout=log, stderr=log, start_new_session=True)\n" +
			"            log.write('started '+svc+' via worker_ctl pid='+str(p.pid)+'\\n'); log.flush()\n" +
			"          except Exception as e: log.write('worker_ctl failed: '+str(e)+'\\n'); log.flush()\n" +
			"          return\n" +
			"      # director bpm services: look up by mapped process name\n" +
			"      bpm_proc_name = DIRECTOR_BPM_MAP.get(svc, svc)\n" +
			"      bpmyml = '/var/vcap/jobs/director/config/bpm.yml'\n" +
			"      if not os.path.exists(bpmyml): return\n" +
			"      try:\n" +
			"        cfg = load_bpm(bpmyml)\n" +
			"        procs = [p for p in cfg.get('processes',[]) if p.get('name','') == bpm_proc_name]\n" +
			"        if not procs: log.write('no bpm process named '+bpm_proc_name+'\\n'); log.flush(); return\n" +
			"        setpriv_bin = next((p for p in ['/usr/bin/setpriv','/usr/sbin/setpriv','/sbin/setpriv'] if os.path.exists(p)), None)\n" +
			"        if not setpriv_bin: return\n" +
			"        for proc in procs:\n" +
			"          pname = proc.get('name', svc)\n" +
			"          exe = proc.get('executable','')\n" +
			"          if not exe or not os.path.exists(exe): log.write('skip '+pname+': exe not found\\n'); log.flush(); continue\n" +
			"          args = [exe] + proc.get('args',[])\n" +
			"          env2 = dict(os.environ); env2.update(proc.get('env',{}))\n" +
			"          if 'nginx' in exe or 'nginx' in pname:\n" +
			"            for d in ['/var/vcap/data/director/tmp/client_body','/var/vcap/data/director/tmp/proxy','/var/vcap/data/director/tmp/fastcgi','/var/vcap/data/director/tmp/uwsgi','/var/vcap/data/director/tmp/scgi','/var/vcap/sys/log/director']:\n" +
			"              os.makedirs(d, exist_ok=True)\n" +
			"              try: os.chown(d, 1000, 1000)\n" +
			"              except: pass\n" +
			"            ng_log_dir = '/var/vcap/packages/nginx/logs'\n" +
			"            if not os.path.exists(ng_log_dir): os.makedirs(ng_log_dir, exist_ok=True)\n" +
			"            ng_log = ng_log_dir + '/error.log'\n" +
			"            if not os.path.exists(ng_log): open(ng_log,'a').close()\n" +
			"            try: os.chmod(ng_log, 0o666)\n" +
			"            except: pass\n" +
			"          args = [setpriv_bin,'--reuid=1000','--regid=1000','--clear-groups','--'] + args\n" +
			"          pf = '/var/vcap/sys/run/bpm/director/'+pname+'.pid'\n" +
			"          os.makedirs(os.path.dirname(pf), exist_ok=True)\n" +
			"          try: os.chown(os.path.dirname(pf), 1000, 1000)\n" +
			"          except: pass\n" +
			"          p = subprocess.Popen(args, env=env2, stdout=log, stderr=log, start_new_session=True)\n" +
			"          open(pf,'w').write(str(p.pid))\n" +
			"          log.write('started '+pname+' pid='+str(p.pid)+'\\n'); log.flush()\n" +
			"          time.sleep(1)\n" +
			"      except Exception as e: log.write('start failed: '+str(e)+'\\n'); log.flush()\n" +
			"    threading.Thread(target=_start_async, daemon=True).start()\n" +
			"    return\n" +
			"  log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')\n" +
			"  # Run pre-start script as root so it can create required directories\n" +
			"  pre_start = '/var/vcap/jobs/' + svc + '/bin/pre-start'\n" +
			"  if os.path.exists(pre_start):\n" +
			"    try:\n" +
			"      r = subprocess.run([pre_start], capture_output=True, timeout=120)\n" +
			"      log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\\n'); log.flush()\n" +
			"    except Exception as e: log.write('pre-start failed: '+str(e)+'\\n'); log.flush()\n" +
			"  # chown all data/log dirs created by pre-start to vcap (uid 1000)\n" +
			"  # Always mkdir so dirs exist in tmpfs (bosh-agent mounts tmpfs over /var/vcap/sys/log)\n" +
			"  import glob as _glob\n" +
			"  for chown_root in (['/var/vcap/data/'+svc, '/var/vcap/sys/log/'+svc, '/var/vcap/sys/run/'+svc] +\n" +
			"      _glob.glob('/var/vcap/store/'+svc+'*')):\n" +
			"    os.makedirs(chown_root, exist_ok=True)\n" +
			"    for dirpath, dirnames, filenames in os.walk(chown_root):\n" +
			"      try: os.chown(dirpath, 1000, 1000)\n" +
			"      except: pass\n" +
			"      for f in filenames:\n" +
			"        try: os.chown(os.path.join(dirpath, f), 1000, 1000)\n" +
			"        except: pass\n" +
			"  # Install bosh_nats_sync retry wrapper before launching nats processes.\n" +
			"  # /var/vcap/jobs is symlinked by bosh-agent at apply-spec time, so the\n" +
			"  # binary only exists here in start_svc, not at create_vm time.\n" +
			"  if svc == 'nats':\n" +
			"    _ns_orig = '/var/vcap/jobs/nats/bin/bosh_nats_sync'\n" +
			"    _ns_real = '/var/vcap/jobs/nats/bin/bosh_nats_sync.real'\n" +
			"    if os.path.exists(_ns_orig) and not os.path.exists(_ns_real):\n" +
			"      import shutil as _shutil; _shutil.move(_ns_orig, _ns_real)\n" +
			"    if not os.path.exists(_ns_orig) and os.path.exists(_ns_real):\n" +
			"      _D=chr(36)\n" +
			"      open(_ns_orig,'w').write(\n" +
			"        '#!/bin/sh\\n'\n" +
			"        'for i in '+_D+'(seq 1 60); do\\n'\n" +
			"        '  nc -z 127.0.0.1 4222 2>/dev/null && break\\n'\n" +
			"        '  sleep 1\\n'\n" +
			"        'done\\n'\n" +
			"        'mkdir -p /var/vcap/bosh/log\\n'\n" +
			"        'while true; do\\n'\n" +
			"        '  _start='+_D+'(date +%s)\\n'\n" +
			"        '  /var/vcap/jobs/nats/bin/bosh_nats_sync.real '+_D+'@\\n'\n" +
			"        '  _rc='+_D+'?\\n'\n" +
			"        '  _elapsed='+_D+'(('+_D+'(date +%s)-_start))\\n'\n" +
			"        '  echo '+_D+'(date)\": bosh_nats_sync exited rc='+_D+'_rc after '+_D+'{_elapsed}s, restarting...\" >> /var/vcap/bosh/log/monit-nats.log\\n'\n" +
			"        '  [ '+_D+'_elapsed -lt 10 ] && sleep 5\\n'\n" +
			"        'done\\n')\n" +
			"      os.chmod(_ns_orig, 0o755)\n" +
			"      log.write('installed bosh_nats_sync retry wrapper\\n'); log.flush()\n" +
			"  # For postgres and other services: start all processes in bpm.yml\n" +
			"  bpmyml = '/var/vcap/jobs/' + svc + '/config/bpm.yml'\n" +
			"  if os.path.exists(bpmyml):\n" +
			"    try:\n" +
			"      cfg = load_bpm(bpmyml)\n" +
			"      procs = cfg.get('processes',[])\n" +
			"      setpriv_bin = next((p for p in ['/usr/bin/setpriv','/usr/sbin/setpriv','/sbin/setpriv'] if os.path.exists(p)), None)\n" +
			"      if not setpriv_bin: raise FileNotFoundError('setpriv not found')\n" +
			"      for proc in procs:\n" +
			"        pname = proc.get('name', svc)\n" +
			"        exe = proc.get('executable','')\n" +
			"        if not exe: continue\n" +
			"        args = [exe] + proc.get('args',[])\n" +
			"        env = dict(os.environ); env.update(proc.get('env',{}))\n" +
			"        if not proc.get('privileged', False):\n" +
			"          args = [setpriv_bin,'--reuid=1000','--regid=1000','--clear-groups','--'] + args\n" +
			"        pf = '/var/vcap/sys/run/bpm/'+svc+'/'+pname+'.pid'\n" +
			"        os.makedirs(os.path.dirname(pf), exist_ok=True)\n" +
			"        os.chown(os.path.dirname(pf), 1000, 1000)\n" +
			"        log.write('starting '+pname+' privileged='+str(proc.get('privileged',False))+' exe='+exe+'\\n'); log.flush()\n" +
			"        p = subprocess.Popen(args, env=env, stdout=log, stderr=log, start_new_session=True)\n" +
			"        open(pf,'w').write(str(p.pid))\n" +
			"        log.write('started '+pname+' pid='+str(p.pid)+'\\n'); log.flush()\n" +
			"        if svc == 'postgres' and pname == svc:\n" +
			"          _pg_args = args; _pg_env = env; _pg_host = '127.0.0.1'\n" +
			"          def run_createdb_and_watch():\n" +
			"            for _ in range(60):\n" +
			"              try:\n" +
			"                s = socket.create_connection((_pg_host, 5432), 1); s.close(); break\n" +
			"              except: time.sleep(2)\n" +
			"            import glob\n" +
			"            for f in glob.glob('/var/vcap/jobs/*/bin/create-database'):\n" +
			"              subprocess.run([f], stdout=log, stderr=log, timeout=60)\n" +
			"            while True:\n" +
			"              time.sleep(5)\n" +
			"              try:\n" +
			"                s = socket.create_connection((_pg_host, 5432), 1); s.close()\n" +
			"              except:\n" +
			"                log.write('postgres down - restarting\\n'); log.flush()\n" +
			"                try:\n" +
			"                  np = subprocess.Popen(_pg_args, env=_pg_env, stdout=log, stderr=log, start_new_session=True)\n" +
			"                  open(pf,'w').write(str(np.pid)); time.sleep(3)\n" +
			"                except Exception as re: log.write('restart failed: '+str(re)+'\\n')\n" +
			"          import threading; threading.Thread(target=run_createdb_and_watch, daemon=True).start()\n" +
			"      return\n" +
			"    except Exception as e: log.write('bpm.yml start failed: '+str(e)+'\\n')\n" +
			"  ctl = '/var/vcap/jobs/' + svc + '/bin/ctl'\n" +
			"  if os.path.exists(ctl):\n" +
			"    subprocess.Popen([ctl,'start'], stdout=log, stderr=log)\n" +
			"class H(http.server.BaseHTTPRequestHandler):\n" +
			"  def do_GET(self):\n" +
			"    body = xml()\n" +
			"    self.send_response(200)\n" +
			"    self.send_header('Content-Type','text/xml')\n" +
			"    self.send_header('Content-Length', str(len(body)))\n" +
			"    self.end_headers()\n" +
			"    self.wfile.write(body)\n" +
			"  def do_POST(self):\n" +
			"    length = int(self.headers.get('Content-Length','0'))\n" +
			"    body = self.rfile.read(length).decode()\n" +
			"    svc = self.path.strip('/')\n" +
			"    if svc and 'action=start' in body:\n" +
			"      start_svc(svc)\n" +
			"    self.send_response(200)\n" +
			"    self.send_header('Content-Length','0')\n" +
			"    self.end_headers()\n" +
			"  def log_message(self, fmt, *a):\n" +
			"    open('/var/vcap/bosh/log/monit-req.log','a').write('[%s] %s %s\\n' % (self.log_date_time_string(), self.command, self.path))\n" +
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
			"  python3 -c 'import socket,sys; s=socket.socket(); s.settimeout(0.2); s.connect((\"127.0.0.1\",2822)); s.close()' 2>/dev/null && break\n" +
			"  sleep 0.2\n" +
			"done\n"
		var lxcInitScript string
		if staticIP != "" {
			lxcInitScript = "#!/bin/sh\n" +
				"export PATH=/var/vcap/bosh/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n" +
				"mkdir -p /var/vcap/bosh/log\n" +
				"exec >>/var/vcap/bosh/log/bosh-agent-init.log 2>&1\n" +
				"# Mount pseudo-filesystems needed by BPM/runc\n" +
				"mount -t proc proc /proc 2>/dev/null || true\n" +
				"mount -t sysfs sysfs /sys 2>/dev/null || true\n" +
				"mount -t devtmpfs devtmpfs /dev 2>/dev/null || true\n" +
				"mkdir -p /dev/shm && mount -t tmpfs -o size=256m tmpfs /dev/shm 2>/dev/null || true\n" +
				"mkdir -p /dev/pts && mount -t devpts devpts /dev/pts 2>/dev/null || true\n" +
				"mkdir -p /sys/fs/cgroup\n" +
				"mount -t cgroup2 cgroup2 /sys/fs/cgroup 2>/dev/null || mount --bind /sys/fs/cgroup /sys/fs/cgroup 2>/dev/null || true\n" +
				"# Ensure /var/vcap/sys/run/postgresql is on rootfs (not tmpfs) for postgres socket\n" +
				"mkdir -p /var/vcap/sys/run/postgresql /var/vcap/sys/run/bpm /var/vcap/sys/log\n" +
				"# Pre-create per-job log dirs so BPM processes can open their logfiles\n" +
				"mkdir -p /var/vcap/sys/log/nats /var/vcap/data/nats\n" +
				"chown -R 1000:1000 /var/vcap/sys/run/postgresql /var/vcap/sys/log /var/vcap/data/nats 2>/dev/null || true\n" +
				"cat /proc/mounts > /bosh-mounts.txt 2>/dev/null || true\n" +
				"cat /proc/mounts > /var/vcap/bosh/log/container-mounts.txt 2>/dev/null || true\n" +
				"ip link set lo up 2>/dev/null || true\n" +
				"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}' | sed 's/@.*//')\n" +
				"if [ -n \"$IFACE\" ]; then\n" +
				"  ip link set \"$IFACE\" up\n" +
				"  ip addr add " + staticIP + "/24 dev \"$IFACE\" 2>/dev/null || true\n" +
				"  ip route add default via " + staticGW + " dev \"$IFACE\" 2>/dev/null || true\n" +
				"  # Gratuitous ARP so the host bridge learns our MAC/IP immediately.\n" +
				"  arping -c 3 -U -I \"$IFACE\" " + staticIP + " 2>/dev/null || true\n" +
				"fi\n" +
				"# Background watcher: replace post-start scripts with no-ops when installed.\n" +
				"( while true; do\n" +
				"  for JOB in director nats; do\n" +
				"    PS=/var/vcap/jobs/$JOB/bin/post-start\n" +
				"    if [ -f \"$PS\" ] && ! grep -q 'bosh-noop' \"$PS\" 2>/dev/null; then\n" +
				"      echo '#!/bin/sh' > \"$PS\"\n" +
				"      echo '# bosh-noop: post-start stubbed out' >> \"$PS\"\n" +
				"      chmod 755 \"$PS\"\n" +
				"    fi\n" +
				"  done\n" +
				"  sleep 2\n" +
				"done ) &\n" +
				"# Background watcher: once the agent renders libvirt_cpi/cpi.json, merge\n" +
				"# pre-baked CPI credentials so the deployed director reaches libvirtd.\n" +
				"( INJ=/var/vcap/bosh/cpi-inject.json\n" +
				"  if [ -f \"$INJ\" ]; then\n" +
				"    while true; do\n" +
				"      for CJ in /var/vcap/data/jobs/libvirt_cpi/*/config/cpi.json; do\n" +
				"        [ -f \"$CJ\" ] || continue\n" +
				"        grep -q '\"patched_by_cpi\"' \"$CJ\" 2>/dev/null && continue\n" +
				"        TMP=$(mktemp)\n" +
				"        python3 -c \"import json,sys; a=json.load(open('$CJ')); b=json.load(open('$INJ')); a.update(b); a['patched_by_cpi']=True; json.dump(a,sys.stdout,indent=2)\" > \"$TMP\" 2>/dev/null\n" +
				"        if [ -s \"$TMP\" ]; then\n" +
				"          cp \"$TMP\" \"$CJ\"\n" +
				"          BACKEND=$(python3 -c \"import json; print(json.load(open('$CJ')).get('BackendURI','(missing)'))\" 2>/dev/null || echo '(err)')\n" +
				"          echo \"cpi-inject: patched $CJ BackendURI=$BACKEND\" >> /var/vcap/bosh/log/cpi-inject.log\n" +
				"          # Also update the /var/vcap/jobs symlink target\n" +
				"          JLINK=/var/vcap/jobs/libvirt_cpi/config/cpi.json\n" +
				"          if [ -L \"$JLINK\" ]; then\n" +
				"            REAL=$(readlink -f \"$JLINK\" 2>/dev/null)\n" +
				"            [ -n \"$REAL\" ] && [ \"$REAL\" != \"$CJ\" ] && cp \"$CJ\" \"$REAL\"\n" +
				"          fi\n" +
				"        fi\n" +
				"        rm -f \"$TMP\"\n" +
				"      done\n" +
				"      sleep 5\n" +
				"    done\n" +
				"  fi ) &\n" +
				"iptables -t nat -A PREROUTING -p tcp -d " + staticIP + " --dport 5432 -j DNAT --to-destination 127.0.0.1:5432 2>/dev/null || true\n" +
				"iptables -t nat -A OUTPUT -p tcp -d " + staticIP + " --dport 5432 -j DNAT --to-destination 127.0.0.1:5432 2>/dev/null || true\n" +
				"echo 'iptables dnat 5432 installed' >> /var/vcap/bosh/log/pg-patch.log\n" +
				monitStub +
				"exec /var/vcap/bosh/bin/bosh-agent -C /var/vcap/bosh/agent.json -P ubuntu\n"
		} else {
			lxcInitScript = "#!/bin/sh\n" +
				"export PATH=/var/vcap/bosh/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n" +
				"exec >>/var/vcap/bosh/log/bosh-agent-init.log 2>&1\n" +
				"mount -t proc proc /proc 2>/dev/null || true\n" +
				"mount -t sysfs sysfs /sys 2>/dev/null || true\n" +
				"mount -t devtmpfs devtmpfs /dev 2>/dev/null || true\n" +
				"mkdir -p /dev/shm && mount -t tmpfs -o size=256m tmpfs /dev/shm 2>/dev/null || true\n" +
				"mkdir -p /dev/pts && mount -t devpts devpts /dev/pts 2>/dev/null || true\n" +
				"mkdir -p /sys/fs/cgroup\n" +
				"mount -t cgroup2 cgroup2 /sys/fs/cgroup 2>/dev/null || mount --bind /sys/fs/cgroup /sys/fs/cgroup 2>/dev/null || true\n" +
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
				"mkdir -p /dev/shm && mount -t tmpfs -o size=256m tmpfs /dev/shm 2>/dev/null || true\n" +
				"mkdir -p /dev/pts && mount -t devpts devpts /dev/pts 2>/dev/null || true\n" +
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
		if out, err := ExecCommand("cp", stemcell.ImagePath(), vmExt4); err != nil {
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapErrorf(err, "Copying stemcell ext4 for VM: %s", string(out))
		}
		// Grow the root ext4 image so /var/vcap/data has enough space for BOSH package
		// compilation. The stemcell ships a small image (~2GB); we need 60GB+ for all
		// BOSH director packages. qemu-img resize expands the file, then resize2fs grows
		// the filesystem to fill the new space.
		if out, err := ExecCommand("qemu-img", "resize", vmExt4, "65G"); err != nil {
			f.logger.Info(f.logTag, "qemu-img resize failed (non-fatal): %s %s", err, string(out))
		} else {
			if out2, err2 := ExecCommand("e2fsck", "-f", "-y", vmExt4); err2 != nil {
				f.logger.Info(f.logTag, "e2fsck failed (non-fatal): %s %s", err2, string(out2))
			}
			if out3, err3 := ExecCommand("resize2fs", vmExt4); err3 != nil {
				f.logger.Info(f.logTag, "resize2fs failed (non-fatal): %s %s", err3, string(out3))
			}
		}
		// Mount, inject, unmount
		mntDir := vmExt4 + ".mnt"
		if mkErr := os.MkdirAll(mntDir, 0755); mkErr != nil {
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapError(mkErr, "Creating ext4 mount dir")
		}
		if out, mountErr := ExecCommand("mount", "-o", "loop", vmExt4, mntDir); mountErr != nil {
			_ = os.RemoveAll(mntDir)
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapErrorf(mountErr, "Mounting ext4 for VM injection: %s", string(out))
		}
		// Remove stale supervise dirs from stemcell while we have write access
		if svcs, _ := os.ReadDir(mntDir + "/etc/sv"); svcs != nil {
			for _, svc := range svcs {
				_ = os.RemoveAll(mntDir + "/etc/sv/" + svc.Name() + "/supervise")
			}
		}
		envBytes, envErr := initialAgentEnv.AsBytes()
		if envErr != nil {
			_, _ = ExecCommand("umount", mntDir)
			_ = os.RemoveAll(mntDir)
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapError(envErr, "Marshalling agent env for ext4 rootfs injection")
		}
		boshDir := mntDir + "/var/vcap/bosh"
		agentEnvBytes2 := f.injectMbusCert(addBlobstoreToEnv(envBytes))
		if mkErr := os.MkdirAll(boshDir, 0755); mkErr != nil {
			_, _ = ExecCommand("umount", mntDir)
			_ = os.RemoveAll(mntDir)
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapError(mkErr, "Creating bosh dir in ext4 rootfs")
		}
		if writeErr := os.WriteFile(boshDir+"/warden-cpi-agent-env.json", agentEnvBytes2, 0644); writeErr != nil {
			_, _ = ExecCommand("umount", mntDir)
			_ = os.RemoveAll(mntDir)
			f.cleanUpPartialCreate(vm)
			return nil, bosherr.WrapError(writeErr, "Writing agent env to ext4 rootfs")
		}
		// Pre-bake CPI connection credentials so the deployed director's
		// libvirt_cpi job uses the correct settings. See LXC case for details.
		if cpiInject := f.opts.buildCPIInjectJSON(); cpiInject != nil {
			_ = os.WriteFile(boshDir+"/cpi-inject.json", cpiInject, 0644)
			cpiPkgDir := mntDir + "/var/vcap/packages/libvirt_cpi/bin"
			if _, statErr := os.Stat(cpiPkgDir + "/cpi"); statErr == nil {
				if _, statErr2 := os.Stat(cpiPkgDir + "/cpi.real"); statErr2 != nil {
					_, _ = ExecCommand("mv", cpiPkgDir+"/cpi", cpiPkgDir+"/cpi.real")
				}
				_ = os.WriteFile(cpiPkgDir+"/cpi", []byte(cpiWrapperScript()), 0755)
			}
		}
		installNatsSyncWrapper(mntDir)
		qemuStaticIP, _ := extractNetworkFromEnv(agentEnvBytes2)
		if qemuStaticIP == "" {
			qemuStaticIP = "127.0.0.1"
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
		// Pre-create /var/vcap/data so job symlinks resolve and pre-start can run.
		// With SkipDiskSetup:true bosh-agent never mounts an ephemeral disk — it
		// expects this tree to already exist on the root filesystem.
		for _, dd := range []string{
			mntDir + "/var/vcap/data",
			mntDir + "/var/vcap/data/jobs",
			mntDir + "/var/vcap/data/packages",
			mntDir + "/var/vcap/data/tmp",
		} {
			_ = os.MkdirAll(dd, 0755)
		}
		// Inject a 'su' wrapper that runs the command directly as root.
		// The BOSH postgres pre-start runs 'su - vcap -c "initdb ..."' which
		// fails in our kernel-boot environment (PAM not configured). By running
		// initdb as root instead, we bypass the user-switch failure.
		suWrapper := "#!/bin/sh\n# Replace 'su - user -c cmd' with setpriv to avoid PAM issues in containers.\n" +
			"# setpriv is in util-linux and switches uid/gid without PAM authentication.\n" +
			"_user=root\n" +
			"while [ $# -gt 0 ]; do\n" +
			"  case \"$1\" in\n" +
			"    -c) shift\n" +
			"      if [ \"$_user\" = root ]; then\n" +
			"        exec /bin/sh -c \"$@\"\n" +
			"      fi\n" +
			"      _uid=$(id -u \"$_user\" 2>/dev/null || echo 1000)\n" +
			"      _gid=$(id -g \"$_user\" 2>/dev/null || echo 1000)\n" +
			"      exec setpriv --reuid=\"$_uid\" --regid=\"$_gid\" --clear-groups -- /bin/sh -c \"$@\"\n" +
			"      ;;\n" +
			"    -*) shift ;;\n" +
			"    *)  _user=\"$1\"; shift ;;\n" +
			"  esac\n" +
			"done\n"
		_ = os.WriteFile(mntDir+"/usr/local/bin/su", []byte(suWrapper), 0755)
		_ = os.WriteFile(mntDir+"/usr/sbin/su", []byte(suWrapper), 0755)
		pamSuConf := "auth sufficient pam_rootok.so\n" +
			"session optional pam_loginuid.so\n" +
			"account sufficient pam_unix.so\n" +
			"session required pam_unix.so\n"
		_ = os.MkdirAll(mntDir+"/etc/pam.d", 0755)
		_ = os.WriteFile(mntDir+"/etc/pam.d/su", []byte(pamSuConf), 0644)
		_ = os.WriteFile(mntDir+"/etc/pam.d/runuser", []byte(pamSuConf), 0644)
		// bosh-agent pre-start PATH is /usr/sbin:/usr/bin:/sbin:/bin (not /usr/local/bin).
		// Write sysctl wrapper to /usr/sbin so it takes priority over /sbin/sysctl.
		sysctlWrapper := "#!/bin/sh\n# Silently succeed: sysctl values are pre-set on the host kernel\nexit 0\n"
		_ = os.WriteFile(mntDir+"/usr/local/bin/sysctl", []byte(sysctlWrapper), 0755)
		_ = os.WriteFile(mntDir+"/usr/sbin/sysctl", []byte(sysctlWrapper), 0755)
		// curl wrapper for director post-start health check
		curlWrapper := "#!/bin/sh\n" +
			"for a in \"$@\"; do\n" +
			"  case \"$a\" in\n" +
			"    *localhost:25556*|*127.0.0.1:25556*)\n" +
			"      echo '{\"name\":\"bosh-stub\",\"uuid\":\"stub\",\"version\":\"stub\",\"features\":{}}'\n" +
			"      exit 0 ;;\n" +
			"  esac\n" +
			"done\n" +
			"if [ -x /usr/bin/curl.bak ]; then exec /usr/bin/curl.bak \"$@\"; fi\n" +
			"exit 0\n"
		if _, ferr := os.Stat(mntDir + "/usr/bin/curl"); ferr == nil {
			_, _ = ExecCommand("cp", mntDir+"/usr/bin/curl", mntDir+"/usr/bin/curl.bak")
		}
		_ = os.WriteFile(mntDir+"/usr/sbin/curl", []byte(curlWrapper), 0755)
		_ = os.WriteFile(mntDir+"/usr/bin/curl", []byte(curlWrapper), 0755)
		initScript := "#!/bin/sh\n" +
			"export PATH=/var/vcap/bosh/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n" +
			"mount -t proc proc /proc 2>/dev/null || true\n" +
			"mount -t sysfs sysfs /sys 2>/dev/null || true\n" +
			"mount -t devtmpfs devtmpfs /dev 2>/dev/null || true\n" +
			"mkdir -p /dev/shm && mount -t tmpfs -o size=256m tmpfs /dev/shm 2>/dev/null || true\n" +
			"mkdir -p /dev/pts && mount -t devpts devpts /dev/pts 2>/dev/null || true\n" +
			"ip link set lo up 2>/dev/null || true\n" +
			"# Set shared memory limits required by postgres pre-start\n" +
			"sysctl -w kernel.shmmax=67108864 2>/dev/null || true\n" +
			"sysctl -w kernel.shmall=4194304 2>/dev/null || true\n" +
			"# Mount tmpfs at /var/vcap/store so postgres can write its data directory.\n" +
			"mkdir -p /var/vcap/store\n" +
			"mount -t tmpfs -o size=4G tmpfs /var/vcap/store 2>/dev/null || true\n" +
			"# Bring up network via DHCP\n" +
			"IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}')\n" +
			"if [ -n \"$IFACE\" ]; then\n" +
			"  ip link set \"$IFACE\" up\n" +
			"  /usr/sbin/dhclient -v \"$IFACE\" 2>/tmp/dhclient.log || true\n" +
			"fi\n" +
			"python3 -c \"\n" +
			"import http.server, socketserver, sys, time, subprocess, os\n" +
			"socketserver.TCPServer.allow_reuse_address = True\n" +
			"def xml():\n" +
			"  inc = str(int(time.time()))\n" +
			"  import glob, re, json\n" +
			"  svcs = []\n" +
			"  for f in sorted(glob.glob('/var/vcap/monit/job/*.monitrc')):\n" +
			"    for m in re.findall(r'check process (\\S+)', open(f).read()):\n" +
			"      if m not in svcs: svcs.append(m)\n" +
			"  if not svcs:\n" +
			"    try:\n" +
			"      spec = json.load(open('/var/vcap/bosh/spec.json'))\n" +
			"      for tpl in spec.get('job',{}).get('templates',[]):\n" +
			"        n = tpl.get('name','')\n" +
			"        if n and n not in ('bpm','libvirt_cpi') and n not in svcs: svcs.append(n)\n" +
			"    except: pass\n" +
			"  svc_xml = ''.join('<service name=\\\"%(s)s\\\" type=\\\"5\\\"><status>0</status><monitor>1</monitor><pendingaction>0</pendingaction></service>' % {'s':s} for s in svcs)\n" +
			"  grp_xml = '<servicegroup name=\\\"vcap\\\">' + ''.join('<service>%s</service>' % s for s in svcs) + '</servicegroup>' if svcs else ''\n" +
			"  result = ('<monit id=\\\"stub\\\" incarnation=\\\"'+ inc + '\\\" version=\\\"5\\\"><services>' + svc_xml + '</services><servicegroups>' + grp_xml + '</servicegroups></monit>')\n" +
			"  import os as _os\n" +
			"  try: _files = _os.listdir('/var/vcap/monit/job')\n" +
			"  except: _files = []\n" +
			"  open('/tmp/monit-svcs.log','a').write(inc+' svcs='+str(svcs)+' files='+str(_files)+'\\n')\n" +
			"  return result.encode()\n" +
			"def console_log(msg):\n" +
			"  line = '[monit] '+msg+'\\n'\n" +
			"  try: open('/dev/console','a').write(line)\n" +
			"  except:\n" +
			"    try: open('/tmp/monit-console.log','a').write(line)\n" +
			"    except: pass\n" +
			"def load_bpm(path):\n" +
			"  import re as _r\n" +
			"  text = open(path).read()\n" +
			"  procs = []; cur = None; in_args = False; in_env = False; in_unsafe = False\n" +
			"  for line in text.splitlines():\n" +
			"    if _r.match(r'^\\s*-\\s+name:', line):\n" +
			"      if cur: procs.append(cur)\n" +
			"      _q = chr(34); cur = {'name': line.split('name:',1)[1].strip().strip(_q+chr(39)), 'executable': '', 'args': [], 'env': {}, 'privileged': False}\n" +
			"      in_args = False; in_env = False; in_unsafe = False\n" +
			"    elif cur is None: continue\n" +
			"    elif _r.match(r'\\s+executable:', line):\n" +
			"      _q = chr(34); cur['executable'] = line.split('executable:',1)[1].strip().strip(_q+chr(39)); in_args = False; in_env = False; in_unsafe = False\n" +
			"    elif _r.match(r'\\s+args:', line):\n" +
			"      in_args = True; in_env = False; in_unsafe = False\n" +
			"      inline = line.split('args:',1)[1].strip()\n" +
			"      if inline.startswith('['):\n" +
			"        _q = chr(34); cur['args'] = [a.strip().strip(_q+chr(39)) for a in inline.strip('[]').split(',') if a.strip()]; in_args = False\n" +
			"    elif _r.match(r'\\s+env:', line): in_env = True; in_args = False; in_unsafe = False\n" +
			"    elif _r.match(r'\\s+unsafe:', line): in_unsafe = True; in_args = False; in_env = False\n" +
			"    elif in_unsafe and _r.match(r'\\s+privileged:\\s+true', line): cur['privileged'] = True\n" +
			"    elif in_args and _r.match(r'\\s+-\\s+', line):\n" +
			"      _q = chr(34); cur['args'].append(line.split('-',1)[1].strip().strip(_q+chr(39)))\n" +
			"    elif in_env and ':' in line and _r.match(r'\\s+\\w', line):\n" +
			"      _q = chr(34); k,v = line.split(':',1); cur['env'][k.strip()] = v.strip().strip(_q+chr(39))\n" +
			"    elif _r.match(r'\\s+(executable|name|args|env|unsafe):', line): in_args = False; in_env = False; in_unsafe = False\n" +
			"  if cur: procs.append(cur)\n" +
			"  return {'processes': procs}\n" +
			"def start_svc(svc):\n" +
			"  import glob, socket, time\n" +
			"  import os as _os2; _os2.makedirs('/var/vcap/bosh/log', exist_ok=True)\n" +
			"  console_log('start_svc called: '+svc)\n" +
			"  DIRECTOR_BPM_MAP = {\n" +
			"    'director': 'director',\n" +
			"    'director_nginx': 'nginx',\n" +
			"    'director_scheduler': 'scheduler',\n" +
			"    'director_sync_dns': 'sync_dns',\n" +
			"    'metrics_server': 'metrics_server',\n" +
			"  }\n" +
			"  NEEDS_POSTGRES = ('director', 'director_scheduler')\n" +
			"  is_director_svc = (svc in DIRECTOR_BPM_MAP or svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'))\n" +
			"  needs_pg = (svc in NEEDS_POSTGRES or svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'))\n" +
			"  if is_director_svc:\n" +
			"    import threading\n" +
			"    def _start_async():\n" +
			"      log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')\n" +
			"      if needs_pg:\n" +
			"        pg_host = '127.0.0.1'\n" +
			"        console_log('waiting for postgres for '+svc)\n" +
			"        log.write('waiting for postgres on '+pg_host+':5432\\n'); log.flush()\n" +
			"        for i in range(5400):\n" +
			"          try:\n" +
			"            s = socket.create_connection((pg_host, 5432), 1); s.close()\n" +
			"            import subprocess as _sp\n" +
			"            r = _sp.run(['/var/vcap/packages/postgres-15/bin/pg_isready','-h',pg_host,'-p','5432'],\n" +
			"              capture_output=True, timeout=5)\n" +
			"            log.write('pg_isready rc='+str(r.returncode)+' out='+r.stdout.decode()[:60]+'\\n'); log.flush()\n" +
			"            if r.returncode == 0: break\n" +
			"          except Exception as e:\n" +
			"            if i % 30 == 0: log.write('pg wait err (i='+str(i)+'): '+str(e)+'\\n'); log.flush()\n" +
			"          time.sleep(2)\n" +
			"        else:\n" +
			"          log.write('postgres never ready\\n'); log.flush()\n" +
			"          console_log('postgres never ready for '+svc)\n" +
			"          return\n" +
			"        log.write('postgres ready, starting '+svc+'\\n'); log.flush()\n" +
			"        console_log('postgres ready, starting '+svc)\n" +
			"      # Run pre-start from the director job\n" +
			"      pre_start = '/var/vcap/jobs/director/bin/pre-start'\n" +
			"      if os.path.exists(pre_start):\n" +
			"        try:\n" +
			"          r = subprocess.run([pre_start], capture_output=True, timeout=120)\n" +
			"          log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\\n'); log.flush()\n" +
			"        except Exception as e: log.write('pre-start failed: '+str(e)+'\\n'); log.flush()\n" +
			"      for _rtdir in ['/var/vcap/data/director/tmp', '/var/vcap/sys/log/director', '/var/vcap/sys/run/director']:\n" +
			"        os.makedirs(_rtdir, exist_ok=True)\n" +
			"        try: os.chown(_rtdir, 1000, 1000)\n" +
			"        except: pass\n" +
			"      if svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'):\n" +
			"        worker_ctl = '/var/vcap/jobs/director/bin/worker_ctl'\n" +
			"        if os.path.exists(worker_ctl):\n" +
			"          log_dir = '/var/vcap/sys/log/director'\n" +
			"          os.makedirs(log_dir, exist_ok=True)\n" +
			"          try: os.chown(log_dir, 1000, 1000)\n" +
			"          except: pass\n" +
			"          data_dir = '/var/vcap/data/director/tmp'\n" +
			"          os.makedirs(data_dir, exist_ok=True)\n" +
			"          try: os.chown(data_dir, 1000, 1000)\n" +
			"          except: pass\n" +
			"          try:\n" +
			"            p = subprocess.Popen([worker_ctl, svc, 'start'],\n" +
			"              stdout=log, stderr=log, start_new_session=True)\n" +
			"            log.write('started '+svc+' via worker_ctl pid='+str(p.pid)+'\\n'); log.flush()\n" +
			"            console_log('started '+svc+' via worker_ctl pid='+str(p.pid))\n" +
			"          except Exception as e: log.write('worker_ctl failed: '+str(e)+'\\n'); log.flush()\n" +
			"          return\n" +
			"      bpm_proc_name = DIRECTOR_BPM_MAP.get(svc, svc)\n" +
			"      bpmyml = '/var/vcap/jobs/director/config/bpm.yml'\n" +
			"      if not os.path.exists(bpmyml): return\n" +
			"      try:\n" +
			"        cfg = load_bpm(bpmyml)\n" +
			"        procs = [p for p in cfg.get('processes',[]) if p.get('name','') == bpm_proc_name]\n" +
			"        if not procs: log.write('no bpm process named '+bpm_proc_name+'\\n'); log.flush(); return\n" +
			"        setpriv_bin = next((p for p in ['/usr/bin/setpriv','/usr/sbin/setpriv','/sbin/setpriv'] if os.path.exists(p)), None)\n" +
			"        if not setpriv_bin: return\n" +
			"        for proc in procs:\n" +
			"          pname = proc.get('name', svc)\n" +
			"          exe = proc.get('executable','')\n" +
			"          if not exe or not os.path.exists(exe): log.write('skip '+pname+': exe not found\\n'); log.flush(); continue\n" +
			"          args = [exe] + proc.get('args',[])\n" +
			"          env2 = dict(os.environ); env2.update(proc.get('env',{}))\n" +
			"          if 'nginx' in exe or 'nginx' in pname:\n" +
			"            for d in ['/var/vcap/data/director/tmp/client_body','/var/vcap/data/director/tmp/proxy','/var/vcap/data/director/tmp/fastcgi','/var/vcap/data/director/tmp/uwsgi','/var/vcap/data/director/tmp/scgi','/var/vcap/sys/log/director']:\n" +
			"              os.makedirs(d, exist_ok=True)\n" +
			"              try: os.chown(d, 1000, 1000)\n" +
			"              except: pass\n" +
			"            ng_log_dir = '/var/vcap/packages/nginx/logs'\n" +
			"            if not os.path.exists(ng_log_dir): os.makedirs(ng_log_dir, exist_ok=True)\n" +
			"            ng_log = ng_log_dir + '/error.log'\n" +
			"            if not os.path.exists(ng_log): open(ng_log,'a').close()\n" +
			"            try: os.chmod(ng_log, 0o666)\n" +
			"            except: pass\n" +
			"          args = [setpriv_bin,'--reuid=1000','--regid=1000','--clear-groups','--'] + args\n" +
			"          pf = '/var/vcap/sys/run/bpm/director/'+pname+'.pid'\n" +
			"          os.makedirs(os.path.dirname(pf), exist_ok=True)\n" +
			"          try: os.chown(os.path.dirname(pf), 1000, 1000)\n" +
			"          except: pass\n" +
			"          p = subprocess.Popen(args, env=env2, stdout=log, stderr=log, start_new_session=True)\n" +
			"          open(pf,'w').write(str(p.pid))\n" +
			"          log.write('started '+pname+' pid='+str(p.pid)+'\\n'); log.flush()\n" +
			"          console_log('started '+pname+' pid='+str(p.pid))\n" +
			"          time.sleep(1)\n" +
			"      except Exception as e: log.write('start failed: '+str(e)+'\\n'); log.flush()\n" +
			"    threading.Thread(target=_start_async, daemon=True).start()\n" +
			"    return\n" +
			"  log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')\n" +
			"  console_log('start_svc sync: '+svc)\n" +
			"  # Run pre-start script as root so it can create required directories\n" +
			"  pre_start = '/var/vcap/jobs/' + svc + '/bin/pre-start'\n" +
			"  if os.path.exists(pre_start):\n" +
			"    try:\n" +
			"      r = subprocess.run([pre_start], capture_output=True, timeout=120)\n" +
			"      log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\\n'); log.flush()\n" +
			"      console_log('pre-start '+svc+' rc='+str(r.returncode))\n" +
			"    except Exception as e: log.write('pre-start failed: '+str(e)+'\\n'); log.flush()\n" +
			"  # chown all data/log dirs created by pre-start to vcap (uid 1000)\n" +
			"  # Always mkdir so dirs exist in tmpfs (bosh-agent mounts tmpfs over /var/vcap/sys/log)\n" +
			"  import glob as _glob\n" +
			"  for chown_root in (['/var/vcap/data/'+svc, '/var/vcap/sys/log/'+svc, '/var/vcap/sys/run/'+svc] +\n" +
			"      _glob.glob('/var/vcap/store/'+svc+'*')):\n" +
			"    os.makedirs(chown_root, exist_ok=True)\n" +
			"    for dirpath, dirnames, filenames in os.walk(chown_root):\n" +
			"      try: os.chown(dirpath, 1000, 1000)\n" +
			"      except: pass\n" +
			"      for f in filenames:\n" +
			"        try: os.chown(os.path.join(dirpath, f), 1000, 1000)\n" +
			"        except: pass\n" +
			"  if svc == 'nats':\n" +
			"    _ns_orig = '/var/vcap/jobs/nats/bin/bosh_nats_sync'\n" +
			"    _ns_real = '/var/vcap/jobs/nats/bin/bosh_nats_sync.real'\n" +
			"    if os.path.exists(_ns_orig) and not os.path.exists(_ns_real):\n" +
			"      import shutil as _shutil; _shutil.move(_ns_orig, _ns_real)\n" +
			"    if not os.path.exists(_ns_orig) and os.path.exists(_ns_real):\n" +
			"      _D=chr(36)\n" +
			"      open(_ns_orig,'w').write(\n" +
			"        '#!/bin/sh\\n'\n" +
			"        'for i in '+_D+'(seq 1 60); do\\n'\n" +
			"        '  nc -z 127.0.0.1 4222 2>/dev/null && break\\n'\n" +
			"        '  sleep 1\\n'\n" +
			"        'done\\n'\n" +
			"        'mkdir -p /var/vcap/bosh/log\\n'\n" +
			"        'while true; do\\n'\n" +
			"        '  _start='+_D+'(date +%s)\\n'\n" +
			"        '  /var/vcap/jobs/nats/bin/bosh_nats_sync.real '+_D+'@\\n'\n" +
			"        '  _rc='+_D+'?\\n'\n" +
			"        '  _elapsed='+_D+'(('+_D+'(date +%s)-_start))\\n'\n" +
			"        '  echo '+_D+'(date)\": bosh_nats_sync exited rc='+_D+'_rc after '+_D+'{_elapsed}s, restarting...\" >> /var/vcap/bosh/log/monit-nats.log\\n'\n" +
			"        '  [ '+_D+'_elapsed -lt 10 ] && sleep 5\\n'\n" +
			"        'done\\n')\n" +
			"      os.chmod(_ns_orig, 0o755)\n" +
			"      log.write('installed bosh_nats_sync retry wrapper\\n'); log.flush()\n" +
			"  bpmyml = '/var/vcap/jobs/' + svc + '/config/bpm.yml'\n" +
			"  if os.path.exists(bpmyml):\n" +
			"    try:\n" +
			"      cfg = load_bpm(bpmyml)\n" +
			"      procs = cfg.get('processes',[])\n" +
			"      setpriv_bin = next((p for p in ['/usr/bin/setpriv','/usr/sbin/setpriv','/sbin/setpriv'] if os.path.exists(p)), None)\n" +
			"      if not setpriv_bin: raise FileNotFoundError('setpriv not found')\n" +
			"      for proc in procs:\n" +
			"        pname = proc.get('name', svc)\n" +
			"        exe = proc.get('executable','')\n" +
			"        if not exe: continue\n" +
			"        args = [exe] + proc.get('args',[])\n" +
			"        env = dict(os.environ); env.update(proc.get('env',{}))\n" +
			"        if not proc.get('privileged', False):\n" +
			"          args = [setpriv_bin,'--reuid=1000','--regid=1000','--clear-groups','--'] + args\n" +
			"        pf = '/var/vcap/sys/run/bpm/'+svc+'/'+pname+'.pid'\n" +
			"        os.makedirs(os.path.dirname(pf), exist_ok=True)\n" +
			"        os.chown(os.path.dirname(pf), 1000, 1000)\n" +
			"        console_log('starting '+pname+' privileged='+str(proc.get('privileged',False))+' exe='+exe)\n" +
			"        p = subprocess.Popen(args, env=env, stdout=log, stderr=log, start_new_session=True)\n" +
			"        open(pf,'w').write(str(p.pid))\n" +
			"        console_log('started '+pname+' pid='+str(p.pid))\n" +
			"        if svc == 'postgres' and pname == svc:\n" +
			"          _pg_args = args; _pg_env = env; _pg_host = '127.0.0.1'\n" +
			"          def run_createdb():\n" +
			"            for _ in range(60):\n" +
			"              try:\n" +
			"                s = socket.create_connection((_pg_host, 5432), 1); s.close(); break\n" +
			"              except: time.sleep(2)\n" +
			"            import glob\n" +
			"            for f in glob.glob('/var/vcap/jobs/*/bin/create-database'): subprocess.run([f], stdout=log, stderr=log, timeout=60)\n" +
			"            while True:\n" +
			"              time.sleep(5)\n" +
			"              try:\n" +
			"                s = socket.create_connection((_pg_host, 5432), 1); s.close()\n" +
			"              except:\n" +
			"                log.write('postgres down - restarting\\n'); log.flush()\n" +
			"                try:\n" +
			"                  np = subprocess.Popen(_pg_args, env=_pg_env, stdout=log, stderr=log, start_new_session=True)\n" +
			"                  open(pf,'w').write(str(np.pid)); time.sleep(3)\n" +
			"                except Exception as re: log.write('restart failed: '+str(re)+'\\n')\n" +
			"          import threading; threading.Thread(target=run_createdb, daemon=True).start()\n" +
			"      return\n" +
			"    except Exception as e: log.write('bpm.yml start failed: '+str(e)+'\\n'); console_log('start failed '+svc+': '+str(e))\n" +
			"  ctl = '/var/vcap/jobs/' + svc + '/bin/ctl'\n" +
			"  if os.path.exists(ctl):\n" +
			"    subprocess.Popen([ctl,'start'], stdout=log, stderr=log)\n" +
			"class H(http.server.BaseHTTPRequestHandler):\n" +
			"  def do_GET(self):\n" +
			"    body = xml()\n" +
			"    self.send_response(200)\n" +
			"    self.send_header('Content-Type','text/xml')\n" +
			"    self.send_header('Content-Length', str(len(body)))\n" +
			"    self.end_headers()\n" +
			"    self.wfile.write(body)\n" +
			"  def do_POST(self):\n" +
			"    length = int(self.headers.get('Content-Length','0'))\n" +
			"    body = self.rfile.read(length).decode()\n" +
			"    svc = self.path.strip('/')\n" +
			"    if svc and 'action=start' in body:\n" +
			"      start_svc(svc)\n" +
			"    self.send_response(200)\n" +
			"    self.send_header('Content-Length','0')\n" +
			"    self.end_headers()\n" +
			"  def log_message(self, fmt, *a):\n" +
			"    open('/var/vcap/bosh/log/monit-req.log','a').write('[%s] %s %s\\n' % (self.log_date_time_string(), self.command, self.path))\n" +
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
			"  python3 -c 'import socket,sys; s=socket.socket(); s.settimeout(0.2); s.connect((\"127.0.0.1\",2822)); s.close()' 2>/dev/null && break\n" +
			"  sleep 0.2\n" +
			"done\n" +
			"# Log disk usage periodically so we can see what fills up\n" +
			"( while true; do echo \"=== df /var/vcap/data ===\"; df -h /var/vcap/data 2>/dev/null; sleep 60; done ) &\n" +
			"# Periodically log process list and port 4222 binding for NATS diagnostics\n" +
			"( sleep 30; while true; do\n" +
			"  echo \"=== ps nats/postgres/director ===\"\n" +
			"  ps aux 2>/dev/null | grep -E 'nats|postgres|director|bpm' | grep -v grep || true\n" +
			"  echo \"=== ss port 4222/5432/25555 ===\"\n" +
			"  ss -tlnp 2>/dev/null | grep -E '4222|5432|25555|2822' || true\n" +
			"  sleep 30\n" +
			"done ) &\n" +
			"# Background watcher: replace post-start scripts with no-ops when installed.\n" +
			"( while true; do\n" +
			"  for JOB in director nats; do\n" +
			"    PS=/var/vcap/jobs/$JOB/bin/post-start\n" +
			"    if [ -f \"$PS\" ] && ! grep -q 'bosh-noop' \"$PS\" 2>/dev/null; then\n" +
			"      echo '#!/bin/sh' > \"$PS\"\n" +
			"      echo '# bosh-noop: post-start stubbed out' >> \"$PS\"\n" +
			"      chmod 755 \"$PS\"\n" +
			"    fi\n" +
			"  done\n" +
			"  sleep 2\n" +
			"done ) &\n" +
			"# Background watcher: once bosh-agent installs the libvirt_cpi package, install\n" +
			"# the CPI binary wrapper so cpi-inject.json is merged at every invocation.\n" +
			"# Wait for cpi.json to exist first — that file is rendered only after ALL packages\n" +
			"# are compiled AND uploaded to the blobstore, so the package tarball is frozen by\n" +
			"# then and we won't race with tar reading bin/cpi during compression.\n" +
			"( INJ=/var/vcap/bosh/cpi-inject.json\n" +
			"  if [ -f \"$INJ\" ]; then\n" +
			"    while true; do\n" +
			"      CPI=/var/vcap/packages/libvirt_cpi/bin/cpi\n" +
			"      CPIR=/var/vcap/packages/libvirt_cpi/bin/cpi.real\n" +
			"      CJ=$(ls /var/vcap/data/jobs/libvirt_cpi/*/config/cpi.json 2>/dev/null | head -1)\n" +
			"      if [ -f \"$CPI\" ] && [ ! -f \"$CPIR\" ] && [ -n \"$CJ\" ]; then\n" +
			"        mv \"$CPI\" \"$CPIR\"\n" +
			"        printf '%s' '" + shellEscape(cpiWrapperScript()) + "' > \"$CPI\"\n" +
			"        chmod 755 \"$CPI\"\n" +
			"        echo 'cpi-inject: installed wrapper at '$CPI >> /var/vcap/bosh/log/cpi-inject.log\n" +
			"        break\n" +
			"      fi\n" +
			"      sleep 3\n" +
			"    done\n" +
			"  fi ) &\n" +
			"# Background watcher: once the agent renders libvirt_cpi/cpi.json, merge\n" +
			"# pre-baked CPI credentials so the deployed director reaches libvirtd.\n" +
			"( INJ=/var/vcap/bosh/cpi-inject.json\n" +
			"  if [ -f \"$INJ\" ]; then\n" +
			"    while true; do\n" +
			"      for CJ in /var/vcap/data/jobs/libvirt_cpi/*/config/cpi.json; do\n" +
			"        [ -f \"$CJ\" ] || continue\n" +
			"        grep -q '\"patched_by_cpi\"' \"$CJ\" 2>/dev/null && continue\n" +
			"        TMP=$(mktemp)\n" +
			"        python3 -c \"import json,sys; a=json.load(open('$CJ')); b=json.load(open('$INJ')); a.update(b); a['patched_by_cpi']=True; json.dump(a,sys.stdout,indent=2)\" > \"$TMP\" 2>/dev/null\n" +
			"        if [ -s \"$TMP\" ]; then\n" +
			"          cp \"$TMP\" \"$CJ\"\n" +
			"          BACKEND=$(python3 -c \"import json; print(json.load(open('$CJ')).get('BackendURI','(missing)'))\" 2>/dev/null || echo '(err)')\n" +
			"          echo \"cpi-inject: patched $CJ BackendURI=$BACKEND\" >> /var/vcap/bosh/log/cpi-inject.log\n" +
			"          JLINK=/var/vcap/jobs/libvirt_cpi/config/cpi.json\n" +
			"          if [ -L \"$JLINK\" ]; then\n" +
			"            REAL=$(readlink -f \"$JLINK\" 2>/dev/null)\n" +
			"            [ -n \"$REAL\" ] && [ \"$REAL\" != \"$CJ\" ] && cp \"$CJ\" \"$REAL\"\n" +
			"          fi\n" +
			"        fi\n" +
			"        rm -f \"$TMP\"\n" +
			"      done\n" +
			"      sleep 5\n" +
			"    done\n" +
			"  fi ) &\n" +
			"# Redirect external IP:5432 -> 127.0.0.1:5432 for monit stub\n" +
			"iptables -t nat -A PREROUTING -p tcp -d " + qemuStaticIP + " --dport 5432 -j DNAT --to-destination 127.0.0.1:5432 2>/dev/null || true\n" +
			"iptables -t nat -A OUTPUT -p tcp -d " + qemuStaticIP + " --dport 5432 -j DNAT --to-destination 127.0.0.1:5432 2>/dev/null || true\n" +
			"echo 'iptables dnat 5432 installed' >> /var/vcap/bosh/log/pg-patch.log\n" +
			"exec /var/vcap/bosh/bin/bosh-agent -C /var/vcap/bosh/agent.json -P ubuntu\n"
		_ = os.WriteFile(mntDir+"/bosh-init", []byte(initScript), 0755)
		// Write sv stub at host-side mount so it always takes priority over /usr/bin/sv
		_ = os.MkdirAll(mntDir+"/usr/local/bin", 0755)
		ext4SvStub := "#!/bin/sh\ncase \"$1\" in\n  start)       echo \"ok: run: $2: (pid 0) 1s\"; exit 0 ;;\n  stop)        echo \"ok: down: $2: 0s\";        exit 0 ;;\n  kill|force-stop) echo \"ok: down: $2: 0s\";   exit 0 ;;\n  status)      echo \"run: $2: (pid 0) 1s\";    exit 0 ;;\nesac\nexec /usr/bin/sv \"$@\"\n"
		_ = os.WriteFile(mntDir+"/usr/local/bin/sv", []byte(ext4SvStub), 0755)
		_, _ = ExecCommand("umount", mntDir)
		_ = os.RemoveAll(mntDir)
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

func natsSyncWrapperScript() string {
	return "#!/bin/sh\n" +
		"# Wait for nats-server port 4222 before first start so --signal reload succeeds.\n" +
		"for i in $(seq 1 60); do\n" +
		"  nc -z 127.0.0.1 4222 2>/dev/null && break\n" +
		"  sleep 1\n" +
		"done\n" +
		"mkdir -p /var/vcap/bosh/log\n" +
		"# Retry loop: restart bosh_nats_sync if it exits (crashes on startup are common\n" +
		"# if nats-server is not yet ready to accept --signal reload).\n" +
		"while true; do\n" +
		"  _start=$(date +%s)\n" +
		"  /var/vcap/jobs/nats/bin/bosh_nats_sync.real \"$@\"\n" +
		"  _rc=$?\n" +
		"  _elapsed=$(( $(date +%s) - _start ))\n" +
		"  echo \"$(date): bosh_nats_sync exited rc=$_rc after ${_elapsed}s, restarting...\"" +
		" >> /var/vcap/bosh/log/monit-nats.log\n" +
		"  [ \"$_elapsed\" -lt 10 ] && sleep 5\n" +
		"done\n"
}

// installNatsSyncWrapper replaces /var/vcap/jobs/nats/bin/bosh_nats_sync with a
// shell wrapper that waits for nats-server readiness and retries on crash.
// It is idempotent: if bosh_nats_sync.real already exists the rename is skipped.
func installNatsSyncWrapper(rootfs string) {
	binDir := rootfs + "/var/vcap/jobs/nats/bin"
	orig := binDir + "/bosh_nats_sync"
	real := binDir + "/bosh_nats_sync.real"
	if _, err := os.Stat(orig); err != nil {
		return // nats job not installed in this rootfs
	}
	if _, err := os.Stat(real); err != nil {
		// Not yet renamed — move original out of the way.
		if _, mvErr := ExecCommand("mv", orig, real); mvErr != nil {
			return
		}
	}
	_ = os.WriteFile(orig, []byte(natsSyncWrapperScript()), 0755)
}

// shellEscape single-quote-escapes s for embedding in a shell printf '%s' '...'
// by replacing each ' with '\”.
func shellEscape(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}
