package vm_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// stubVMUUIDGen is a UUID generator for vm factory tests.
type stubVMUUIDGen struct {
	result string
	err    error
}

func (g *stubVMUUIDGen) Generate() (string, error) { return g.result, g.err }

// stubDiskUUIDGen is a UUID generator for the disk sub-factory.
type stubDiskUUIDGen struct {
	result string
	err    error
}

func (g *stubDiskUUIDGen) Generate() (string, error) { return g.result, g.err }

var _ = Describe("vm.Factory", func() {
	var (
		vmUUIDGen   *stubVMUUIDGen
		diskUUIDGen *stubDiskUUIDGen
		runner      *driverfakes.FakeRunner
		drv         *driverfakes.FakeDriver
		builder     *driverfakes.FakeDomainBuilder
		diskFactory bdisk.Factory
		factory     vm.Factory
		logger      boshlog.Logger
		stemcell    *stemcellfakes.FakeStemcell
		cloudProps  apiv1.VMCloudProps
		tmpDir      string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "vm-factory-test")
		Expect(err).ToNot(HaveOccurred())

		logger = boshlog.NewLogger(boshlog.LevelNone)
		vmUUIDGen = &stubVMUUIDGen{result: "uuid-vm-1"}
		diskUUIDGen = &stubDiskUUIDGen{result: "disk-uuid-1"}
		runner = &driverfakes.FakeRunner{}
		drv = &driverfakes.FakeDriver{
			// Make LookupDomain appear as "domain not found" so that
			// HaltIfRunning (called by cleanUpPartialCreate → Delete) returns
			// immediately instead of trying to call GetState on a nil domain.
			LookupDomainErr:          errors.New("domain not found"),
			IsMissingDomainErrResult: true,
		}
		builder = &driverfakes.FakeDomainBuilder{
			BuildDomainXML:        "<domain/>",
			DiskImageFormatResult: "qcow2",
		}

		diskFactory = bdisk.NewFactory(filepath.Join(tmpDir, "disks"), diskUUIDGen, drv, runner, logger)

		stemcell = stemcellfakes.NewFakeStemcell("sc-1")
		stemcell.ImagePathResult = "/stemcells/sc-1/image.qcow2"

		// CloudPropsImpl with an empty JSON object uses VMProps defaults.
		cloudProps = apiv1.CloudPropsImpl{RawMessage: json.RawMessage("{}")}

		factory = vm.NewFactory(
			vm.FactoryOpts{DirPath: filepath.Join(tmpDir, "vms")},
			vmUUIDGen,
			drv,
			runner,
			builder,
			diskFactory,
			apiv1.AgentOptions{Mbus: "nats://nats:nats-password@127.0.0.1:4222"},
			apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2}),
			logger,
		)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("Create", func() {
		It("returns VM with 'vm-' prefixed ID on success", func() {
			v, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(v.ID().AsString()).To(Equal("vm-uuid-vm-1"))
		})

		It("propagates mbus from AgentOptions into the agent env", func() {
			v, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(v).ToNot(BeNil())
			// Locate the env.json write among all Put calls.
			var envJSON []byte
			for path, contents := range runner.PutContents {
				if strings.HasSuffix(path, "env.json") {
					envJSON = contents
					break
				}
			}
			Expect(envJSON).ToNot(BeNil(), "env.json was never written")
			Expect(string(envJSON)).To(ContainSubstring("nats://nats:nats-password@127.0.0.1:4222"))
		})

		It("returns error when UUID generation fails", func() {
			vmUUIDGen.err = errors.New("uuid failure")
			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Generating VM id"))
		})

		It("returns error when BuildDomain fails", func() {
			builder.BuildDomainErr = errors.New("build failed")
			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Building domain XML"))
		})

		It("returns error when DefineDomain fails", func() {
			builder.BuildDomainXML = "<domain/>"
			drv.DefineDomainErr = errors.New("define failed")
			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Defining domain"))
		})
	})

	Describe("Find", func() {
		It("returns VM with the given CID", func() {
			v, err := factory.Find(apiv1.NewVMCID("vm-xyz"))
			Expect(err).ToNot(HaveOccurred())
			Expect(v.ID().AsString()).To(Equal("vm-xyz"))
		})
	})

	Describe("injectMbusCert", func() {
		It("preserves existing mbus.url when injecting cert", func() {
			f := vm.NewFactory(
				vm.FactoryOpts{DirPath: tmpDir},
				vmUUIDGen, drv, runner,
				&driverfakes.FakeDomainBuilder{DiskImageFormatResult: "qcow2"},
				diskFactory,
				apiv1.AgentOptions{Mbus: "nats://127.0.0.1:4222"},
				apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2}),
				logger,
			)

			// env JSON with mbus.url already set
			envWithURL := []byte(`{
				"env": {
					"bosh": {
						"mbus": {
							"url": "nats://nats:secret@192.168.0.1:4222"
						}
					}
				}
			}`)

			result := f.InjectMbusCertForTest(envWithURL)

			var m map[string]interface{}
			Expect(json.Unmarshal(result, &m)).To(Succeed())
			env := m["env"].(map[string]interface{})
			bosh := env["bosh"].(map[string]interface{})
			mbus := bosh["mbus"].(map[string]interface{})

			Expect(mbus["cert"]).ToNot(BeNil())
			Expect(mbus["url"]).To(Equal("nats://nats:secret@192.168.0.1:4222"))
		})
	})

	Describe("Create (ext4 branch)", func() {
		var ext4Builder *driverfakes.FakeDomainBuilder

		BeforeEach(func() {
			ext4Builder = &driverfakes.FakeDomainBuilder{
				BuildDomainXML:        "<domain/>",
				DiskImageFormatResult: "ext4",
			}
			factory = vm.NewFactory(
				vm.FactoryOpts{DirPath: filepath.Join(tmpDir, "vms")},
				vmUUIDGen,
				drv,
				runner,
				ext4Builder,
				diskFactory,
				apiv1.AgentOptions{Mbus: "nats://nats:nats-password@127.0.0.1:4222"},
				apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2}),
				logger,
			)
		})

		It("returns error when mount fails", func() {
			// Make the runner fail specifically for "mount" so the ext4 injection
			// path returns an error at the mount step.
			runner.ExecuteFunc = func(name string, args ...string) (string, int, error) {
				if name == "mount" {
					return "no loop devices", 1, errors.New("mount failed")
				}
				return "", 0, nil
			}
			defer func() { runner.ExecuteFunc = nil }()

			stemcell.ImagePathResult = filepath.Join(tmpDir, "stemcell.img")
			_ = os.WriteFile(stemcell.ImagePathResult, []byte("fake-ext4"), 0644)

			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Mounting ext4 for VM injection"))
		})

		It("runs through ext4 injection without error when all commands succeed", func() {
			stemcellImg := filepath.Join(tmpDir, "stemcell.img")
			_ = os.WriteFile(stemcellImg, []byte("fake"), 0644)
			stemcell.ImagePathResult = stemcellImg

			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			// AsBytes() on a real AgentEnv never fails with valid inputs; this test
			// exercises the AsBytes error-check branch via the happy path — a failure
			// from AsBytes would surface as an unexpected error here.
			Expect(err).ToNot(HaveOccurred())
		})

		It("uses truncate+losetup+e2fsck+resize2fs to grow ext4", func() {
			var executedCmds []string
			losetupCallCount := 0
			runner.ExecuteFunc = func(name string, args ...string) (string, int, error) {
				cmd := name + " " + strings.Join(args, " ")
				executedCmds = append(executedCmds, cmd)
				if name == "losetup" && len(args) >= 2 && args[0] == "-f" && args[1] == "--show" {
					losetupCallCount++
					if losetupCallCount == 1 {
						return "/dev/loop7\n", 0, nil // mount loop
					}
					return "/dev/loop8\n", 0, nil // clean loop
				}
				return "", 0, nil
			}
			defer func() { runner.ExecuteFunc = nil }()

			stemcellImg := filepath.Join(tmpDir, "stemcell.img")
			_ = os.WriteFile(stemcellImg, []byte("fake"), 0644)
			stemcell.ImagePathResult = stemcellImg

			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).ToNot(HaveOccurred())

			vmImg := filepath.Join(tmpDir, "vms/vm-uuid-vm-1/rootfs.img")
			cleanScript := vmImg + ".clean.sh"
			truncateIdx := -1
			losetupAttachIdx := -1   // first losetup: mount loop
			e2fsckPreResizeIdx := -1 // e2fsck on /dev/loop7 before resize2fs
			resizeLoopIdx := -1
			mountLoopIdx := -1
			syncBeforeUmountIdx := -1
			umountIdx := -1
			rmdirIdx := -1
			losetupDetachIdx := -1     // losetup -d /dev/loop7
			cleanLoopAttachIdx := -1   // second losetup: clean loop (/dev/loop8)
			e2fsckJournalOnlyIdx := -1 // e2fsck -E journal_only on /dev/loop8
			cleanScriptIdx := -1       // sh clean.sh (tune2fs on /dev/loop8)
			cleanLoopDetachIdx := -1   // losetup -d /dev/loop8
			qemuImgResizeCalled := false
			rmrfCalled := false
			for i, cmd := range executedCmds {
				if cmd == "truncate -s 65G "+vmImg {
					truncateIdx = i
				}
				if cmd == "losetup -f --show "+vmImg && losetupAttachIdx < 0 {
					losetupAttachIdx = i
				}
				if cmd == "losetup -f --show "+vmImg && losetupAttachIdx >= 0 && cleanLoopAttachIdx < 0 && i > losetupAttachIdx {
					cleanLoopAttachIdx = i
				}
				if cmd == "e2fsck -fy /dev/loop7" {
					e2fsckPreResizeIdx = i
				}
				if cmd == "resize2fs -f /dev/loop7" {
					resizeLoopIdx = i
				}
				if cmd == "mount /dev/loop7 "+vmImg+".mnt" {
					mountLoopIdx = i
				}
				if strings.TrimSpace(cmd) == "sync" && mountLoopIdx >= 0 && umountIdx < 0 {
					syncBeforeUmountIdx = i
				}
				if cmd == "umount "+vmImg+".mnt" {
					umountIdx = i
				}
				if cmd == "rmdir "+vmImg+".mnt" {
					rmdirIdx = i
				}
				if cmd == "losetup -d /dev/loop7" {
					losetupDetachIdx = i
				}
				if cmd == "e2fsck -E journal_only -fy /dev/loop8" {
					e2fsckJournalOnlyIdx = i
				}
				if cmd == "sh "+cleanScript {
					cleanScriptIdx = i
				}
				if cmd == "losetup -d /dev/loop8" {
					cleanLoopDetachIdx = i
				}
				if strings.HasPrefix(cmd, "qemu-img resize") {
					qemuImgResizeCalled = true
				}
				if strings.HasPrefix(cmd, "rm -rf") && strings.Contains(cmd, ".mnt") {
					rmrfCalled = true
				}
			}
			// Verify the clean script was Put with tune2fs content targeting the clean loop.
			Expect(runner.PutContents).ToNot(BeNil(), "runner.Put must have been called")
			scriptContent, ok := runner.PutContents[cleanScript]
			Expect(ok).To(BeTrue(), "clean script must have been Put at "+cleanScript)
			Expect(string(scriptContent)).To(ContainSubstring("tune2fs"), "clean script must contain tune2fs")
			Expect(string(scriptContent)).To(ContainSubstring("^needs_recovery"), "clean script must clear needs_recovery")
			Expect(string(scriptContent)).To(ContainSubstring("/dev/loop8"), "clean script must target clean loop device")

			Expect(truncateIdx).To(BeNumerically(">=", 0), "truncate must be called")
			Expect(losetupAttachIdx).To(BeNumerically(">=", 0), "first losetup -f --show (mount loop) must be called")
			Expect(e2fsckPreResizeIdx).To(BeNumerically(">=", 0), "e2fsck -fy on /dev/loop7 before resize2fs must be called")
			Expect(resizeLoopIdx).To(BeNumerically(">=", 0), "resize2fs -f on loop device must be called")
			Expect(mountLoopIdx).To(BeNumerically(">=", 0), "mount of loop device must be called")
			Expect(syncBeforeUmountIdx).To(BeNumerically(">=", 0), "sync must be called after mount and before umount")
			Expect(umountIdx).To(BeNumerically(">=", 0), "umount must be called")
			Expect(rmdirIdx).To(BeNumerically(">=", 0), "rmdir of mount point must be called (not rm -rf)")
			Expect(losetupDetachIdx).To(BeNumerically(">=", 0), "losetup -d /dev/loop7 must be called")
			Expect(cleanLoopAttachIdx).To(BeNumerically(">=", 0), "second losetup -f --show (clean loop) must be called")
			Expect(e2fsckJournalOnlyIdx).To(BeNumerically(">=", 0), "e2fsck -E journal_only on clean loop must be called")
			Expect(cleanScriptIdx).To(BeNumerically(">=", 0), "sh clean script (tune2fs) must be called")
			Expect(cleanLoopDetachIdx).To(BeNumerically(">=", 0), "losetup -d /dev/loop8 (clean loop detach) must be called")
			Expect(truncateIdx).To(BeNumerically("<", losetupAttachIdx), "truncate before losetup attach")
			Expect(losetupAttachIdx).To(BeNumerically("<", e2fsckPreResizeIdx), "losetup attach before pre-resize e2fsck")
			Expect(e2fsckPreResizeIdx).To(BeNumerically("<", resizeLoopIdx), "e2fsck before resize2fs")
			Expect(resizeLoopIdx).To(BeNumerically("<", mountLoopIdx), "resize2fs before mount")
			Expect(mountLoopIdx).To(BeNumerically("<", syncBeforeUmountIdx), "mount before pre-umount sync")
			Expect(syncBeforeUmountIdx).To(BeNumerically("<", umountIdx), "sync before umount")
			Expect(umountIdx).To(BeNumerically("<", rmdirIdx), "umount before rmdir")
			Expect(umountIdx).To(BeNumerically("<", losetupDetachIdx), "umount before losetup detach")
			Expect(losetupDetachIdx).To(BeNumerically("<", cleanLoopAttachIdx), "mount loop detached before clean loop attached")
			Expect(cleanLoopAttachIdx).To(BeNumerically("<", e2fsckJournalOnlyIdx), "clean loop attached before journal_only e2fsck")
			Expect(e2fsckJournalOnlyIdx).To(BeNumerically("<", cleanScriptIdx), "journal_only e2fsck before tune2fs clean script")
			Expect(cleanScriptIdx).To(BeNumerically("<", cleanLoopDetachIdx), "tune2fs clean script before clean loop detach")
			Expect(qemuImgResizeCalled).To(BeFalse(), "qemu-img resize must not be called")
			Expect(rmrfCalled).To(BeFalse(), "rm -rf on mount point must not be called (use rmdir)")
		})
	})
})
