package vm_test

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	bdisk "bosh-libvirt-cpi/disk"
	driverfakes "bosh-libvirt-cpi/driver/fakes"
	stemcellfakes "bosh-libvirt-cpi/stemcell/fakes"
	"bosh-libvirt-cpi/vm"
)

// These are characterization tests: they lock the exact set of files the
// container-backend Create() writes into the VM rootfs, with a hash of each
// file's contents. The decomposition refactor must not change this manifest.
// Regenerate the golden files with REGEN_GOLDEN=1 after an intentional change.

// nonDeterministicSuffixes are files whose contents embed a freshly generated
// self-signed mbus cert and a timestamp (via injectMbusCert) or the agent env
// built from them, so their bytes differ every run. We record their presence
// but not a hash. The decomposition refactor does not touch these paths.
var nonDeterministicSuffixes = []string{
	"env.json",
	"env.iso",
	"disk-attachment.json",
	"warden-cpi-agent-env.json",
}

func isNonDeterministic(path string) bool {
	for _, s := range nonDeterministicSuffixes {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}

// putManifest renders a deterministic "relpath sha256" listing of every file
// written via runner.Put. Both prefixes are stripped so the manifest does not
// depend on the test's temp directory: tmpPrefix (the volatile os.MkdirTemp
// path) first, then rootPrefix (the per-VM rootfs mount point). Files whose
// contents are inherently non-deterministic are listed with a fixed marker.
func putManifest(put map[string][]byte, tmpPrefix, rootPrefix string) string {
	paths := make([]string, 0, len(put))
	for p := range put {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	for _, p := range paths {
		rel := strings.TrimPrefix(p, rootPrefix)
		rel = strings.TrimPrefix(rel, tmpPrefix)
		if isNonDeterministic(p) {
			fmt.Fprintf(&b, "%s <nondeterministic>\n", rel)
			continue
		}
		sum := sha256.Sum256(put[p])
		fmt.Fprintf(&b, "%s %x\n", rel, sum)
	}
	return b.String()
}

func checkGolden(goldenPath, actual string) {
	if os.Getenv("REGEN_GOLDEN") == "1" {
		Expect(os.MkdirAll(filepath.Dir(goldenPath), 0755)).To(Succeed())
		Expect(os.WriteFile(goldenPath, []byte(actual), 0644)).To(Succeed())
		return
	}
	want, err := os.ReadFile(goldenPath)
	Expect(err).ToNot(HaveOccurred(), "missing golden file %s; run with REGEN_GOLDEN=1 to create it", goldenPath)
	Expect(actual).To(Equal(string(want)),
		"injected-file manifest changed vs %s. If this is intentional, regenerate with REGEN_GOLDEN=1.", goldenPath)
}

var _ = Describe("vm.Factory container-backend characterization", func() {
	var (
		vmUUIDGen   *stubVMUUIDGen
		diskUUIDGen *stubDiskUUIDGen
		runner      *driverfakes.FakeRunner
		drv         *driverfakes.FakeDriver
		diskFactory bdisk.Factory
		logger      boshlog.Logger
		stemcell    *stemcellfakes.FakeStemcell
		tmpDir      string
		vmsDir      string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "vm-char-test")
		Expect(err).ToNot(HaveOccurred())
		vmsDir = filepath.Join(tmpDir, "vms")

		logger = boshlog.NewLogger(boshlog.LevelNone)
		vmUUIDGen = &stubVMUUIDGen{result: "uuid-vm-1"}
		diskUUIDGen = &stubDiskUUIDGen{result: "disk-uuid-1"}
		runner = &driverfakes.FakeRunner{}
		drv = &driverfakes.FakeDriver{
			LookupDomainErr:          errors.New("domain not found"),
			IsMissingDomainErrResult: true,
		}
		diskFactory = bdisk.NewFactory(filepath.Join(tmpDir, "disks"), diskUUIDGen, drv, runner, logger)
		stemcell = stemcellfakes.NewFakeStemcell("sc-1")
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	// newFactory builds a Factory with the given disk-image format so we can
	// drive the dir (LXC) and ext4 (QEMU kernel-boot) branches of Create().
	newFactory := func(format string) vm.Factory {
		builder := &driverfakes.FakeDomainBuilder{
			BuildDomainXML:        "<domain/>",
			DiskImageFormatResult: format,
		}
		return vm.NewFactory(
			vm.FactoryOpts{DirPath: vmsDir},
			vmUUIDGen, drv, runner, builder, diskFactory,
			apiv1.AgentOptions{Mbus: "nats://nats:nats-password@127.0.0.1:4222"},
			apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2}),
			logger,
		)
	}

	// directorNetworks returns a fixed static network so the generated init
	// scripts are deterministic (no DHCP, no random values).
	directorNetworks := func() apiv1.Networks {
		raw := json.RawMessage(`{
			"default": {
				"type": "manual",
				"ip": "192.168.122.11",
				"netmask": "255.255.255.0",
				"gateway": "192.168.122.1",
				"default": ["dns", "gateway"]
			}
		}`)
		var nets apiv1.Networks
		Expect(json.Unmarshal(raw, &nets)).To(Succeed())
		return nets
	}

	It("writes a stable set of files for the LXC (dir) director VM", func() {
		f := newFactory("dir")

		// isDirectorAgentEnv keys off an https mbus; the agent env mbus comes
		// from AgentOptions, which is nats://. Use the env's director marker by
		// deploying with a static IP director network (the dir director branch
		// also requires the https mbus). This test characterizes the non-director
		// static-IP dir branch, which is the common deploy path.
		stemcell.ImagePathResult = filepath.Join(tmpDir, "stemcells", "sc-1")
		Expect(os.MkdirAll(stemcell.ImagePathResult, 0755)).To(Succeed())

		_, err := f.Create(
			apiv1.NewAgentID("agent-1"),
			stemcell,
			apiv1.CloudPropsImpl{RawMessage: json.RawMessage(`{"mac":"52:54:00:ab:cd:11"}`)},
			directorNetworks(),
			apiv1.NewVMEnv(nil),
		)
		Expect(err).ToNot(HaveOccurred())

		rootPrefix := filepath.Join(vmsDir, "vm-uuid-vm-1", "rootfs") + "/"
		manifest := putManifest(runner.PutContents, tmpDir+"/", rootPrefix)
		checkGolden("testdata/lxc_dir_putmanifest.golden", manifest)
	})

	It("writes a stable set of files for the QEMU ext4 VM", func() {
		f := newFactory("ext4")

		stemcellImg := filepath.Join(tmpDir, "stemcell.img")
		Expect(os.WriteFile(stemcellImg, []byte("fake-ext4"), 0644)).To(Succeed())
		stemcell.ImagePathResult = stemcellImg

		_, err := f.Create(
			apiv1.NewAgentID("agent-1"),
			stemcell,
			apiv1.CloudPropsImpl{RawMessage: json.RawMessage(`{"mac":"52:54:00:ab:cd:10"}`)},
			directorNetworks(),
			apiv1.NewVMEnv(nil),
		)
		Expect(err).ToNot(HaveOccurred())

		rootPrefix := filepath.Join(vmsDir, "vm-uuid-vm-1", "rootfs.img.mnt") + "/"
		manifest := putManifest(runner.PutContents, tmpDir+"/", rootPrefix)
		checkGolden("testdata/qemu_ext4_putmanifest.golden", manifest)
	})
})
