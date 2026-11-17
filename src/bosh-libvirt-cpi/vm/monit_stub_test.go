package vm_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/vm"
)

// monitStubPython returns the Python source of the monit stub embedded in the
// kernel-boot /bosh-init (a bash `python3 -c "..."`), unescaped and cut off
// before the HTTP server so its helpers can be exercised directly.
func monitStubPython() string {
	script := vm.RenderQEMUExt4InitForTest()
	start := strings.Index(script, "python3 -c \"\n")
	Expect(start).To(BeNumerically(">=", 0), "monit stub not found in /bosh-init")
	start += len("python3 -c \"\n")
	end := strings.Index(script[start:], "\n\" >/tmp/monit-stub.log")
	Expect(end).To(BeNumerically(">=", 0), "end of monit stub not found")
	src := script[start : start+end]

	// Undo bash double-quote escaping: \" \\ \$ \` lose their backslash.
	var b strings.Builder
	for i := 0; i < len(src); i++ {
		if src[i] == '\\' && i+1 < len(src) && strings.ContainsRune("\"\\$`", rune(src[i+1])) {
			i++
		}
		b.WriteByte(src[i])
	}
	py := b.String()
	cut := strings.Index(py, "\nclass H(")
	Expect(cut).To(BeNumerically(">=", 0), "HTTP handler not found in monit stub")
	return py[:cut] + "\n"
}

var _ = Describe("kernel-boot monit stub", func() {
	var (
		tmp     string
		jobGlob string
		jobsDir string
		pgPid   string
	)

	runPy := func(driver string) string {
		if _, err := exec.LookPath("python3"); err != nil {
			Skip("python3 not available")
		}
		prelude := "MONIT_GLOB = '" + jobGlob + "'\nJOBS_DIR = '" + jobsDir + "'\n"
		cmd := exec.Command("python3", "-c", monitStubPython()+prelude+driver)
		out, err := cmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(out))
		return strings.TrimSpace(string(out))
	}

	BeforeEach(func() {
		var err error
		tmp, err = os.MkdirTemp("", "monit-stub-test")
		Expect(err).ToNot(HaveOccurred())

		monitDir := filepath.Join(tmp, "monit", "job")
		jobsDir = filepath.Join(tmp, "jobs")
		jobGlob = filepath.Join(monitDir, "*.monitrc")
		pgPid = filepath.Join(tmp, "postgres.pid")
		Expect(os.MkdirAll(monitDir, 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(jobsDir, "postgres", "bin"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(jobsDir, "nats", "config"), 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(jobsDir, "nats", "config", "bpm.yml"), []byte("processes: []\n"), 0644)).To(Succeed())

		// A classic (non-bpm) monit job, like postgres-release's postgres job.
		Expect(os.WriteFile(filepath.Join(monitDir, "0000_postgres.monitrc"), []byte(`check process postgres
  with pidfile `+pgPid+`
  start program "`+jobsDir+`/postgres/bin/postgres_ctl start"
    as uid vcap and gid vcap timeout 60 seconds
  stop program "`+jobsDir+`/postgres/bin/postgres_ctl stop"
  group vcap
check process pg_janitor
  with pidfile `+filepath.Join(tmp, "pg_janitor.pid")+`
  start program "`+jobsDir+`/postgres/bin/pg_janitor_ctl start"
    as uid vcap and gid vcap timeout 60 seconds
  stop program "`+jobsDir+`/postgres/bin/pg_janitor_ctl stop"
  depends on postgres
  group vcap
`), 0644)).To(Succeed())
		// A bpm job: status handling must stay as it was.
		Expect(os.WriteFile(filepath.Join(monitDir, "0001_nats.monitrc"), []byte(`check process nats
  with pidfile /var/vcap/sys/run/bpm/nats/nats.pid
  start program "/var/vcap/jobs/bpm/bin/bpm start nats"
  stop program "/var/vcap/jobs/bpm/bin/bpm stop nats"
  group vcap
`), 0644)).To(Succeed())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmp)
	})

	It("parses start/stop programs, uid/gid, pidfile and owning job from monitrc", func() {
		out := runPy(`
s = monit_services()
pg = s['postgres']; jan = s['pg_janitor']
print(pg['job'], pg['start'], pg['stop'], pg['uid'], pg['gid'], pg['pidfile'])
print(jan['job'], jan['start'])
print(sorted(s))
`)
		lines := strings.Split(out, "\n")
		Expect(lines[0]).To(Equal("postgres " + jobsDir + "/postgres/bin/postgres_ctl start " + jobsDir + "/postgres/bin/postgres_ctl stop vcap vcap " + pgPid))
		Expect(lines[1]).To(Equal("postgres " + jobsDir + "/postgres/bin/pg_janitor_ctl start"))
		Expect(lines[2]).To(Equal("['nats', 'pg_janitor', 'postgres']"))
	})

	It("treats only jobs without bpm.yml as monitrc-managed", func() {
		Expect(runPy(`print(is_monitrc_managed('postgres'), is_monitrc_managed('nats'), is_monitrc_managed('unknown'))`)).
			To(Equal("True False False"))
	})

	It("reports monitrc services as unmonitored until started, then by their pidfile", func() {
		out := runPy(`
import os
print(monit_status('postgres'))            # never started
STARTED.add('postgres')
print(monit_status('postgres'))            # started, no pidfile
open('` + pgPid + `','w').write(str(os.getpid()))
print(monit_status('postgres'))            # started, process alive
open('` + pgPid + `','w').write('999999')
print(monit_status('postgres'))            # started, process gone
print(monit_status('nats'))                # bpm service: unchanged
`)
		Expect(strings.Split(out, "\n")).To(Equal([]string{
			"(0, 0)",
			"(512, 1)",
			"(0, 1)",
			"(512, 1)",
			"(0, 1)",
		}))
	})

	It("runs a monitrc start program and stop program", func() {
		marker := filepath.Join(tmp, "calls.log")
		ctl := filepath.Join(jobsDir, "postgres", "bin", "postgres_ctl")
		Expect(os.WriteFile(ctl, []byte("#!/bin/sh\necho \"$1\" >> "+marker+"\n"), 0755)).To(Succeed())

		runPy(`
log = open('` + filepath.Join(tmp, "monit-postgres.log") + `', 'a')
s = monit_services()['postgres']
run_program(s['start'], None, None, log).wait()
run_program(s['stop'], None, None, log).wait()
`)
		Eventually(func() string {
			b, _ := os.ReadFile(marker)
			return string(b)
		}, 5*time.Second).Should(Equal("start\nstop\n"))
	})

	It("parses depends on", func() {
		Expect(runPy(`
s = monit_services()
print(s['pg_janitor']['depends'], s['postgres']['depends'])
`)).To(Equal("['postgres'] []"))
	})

	It("waits for dependencies to run before starting a service", func() {
		out := runPy(`
import os
print(wait_for_deps('pg_janitor', timeout=0.5, interval=0.1))   # postgres not started
STARTED.add('postgres')
open('` + pgPid + `','w').write(str(os.getpid()))
print(wait_for_deps('pg_janitor', timeout=0.5, interval=0.1))   # postgres running
print(wait_for_deps('postgres', timeout=0.5, interval=0.1))     # no dependencies
`)
		Expect(strings.Split(out, "\n")).To(Equal([]string{"False", "True", "True"}))
	})

	Describe("restarting monitrc services that died (monit cycle)", func() {
		var marker string

		BeforeEach(func() {
			marker = filepath.Join(tmp, "janitor-starts.log")
			ctl := filepath.Join(jobsDir, "postgres", "bin", "pg_janitor_ctl")
			Expect(os.WriteFile(ctl, []byte("#!/bin/sh\necho \"$1\" >> "+marker+"\n"), 0755)).To(Succeed())
		})

		starts := func() int {
			b, _ := os.ReadFile(marker)
			return strings.Count(string(b), "start")
		}

		// Driver prelude: postgres running, pg_janitor started long ago and dead.
		// Programs run as the test user: the uid switch (setpriv) needs root
		// and is not what these tests are about.
		const setup = `
import os, time
_parsed = monit_services
def monit_services():
  s = _parsed()
  for d in s.values(): d['uid'] = d['gid'] = None
  return s
LOG = open(os.devnull, 'a')
STARTED.update(['postgres', 'pg_janitor'])
open(monit_services()['postgres']['pidfile'],'w').write(str(os.getpid()))
open(monit_services()['pg_janitor']['pidfile'],'w').write('999999')
STARTED_AT['pg_janitor'] = time.time() - 600
`

		It("re-runs the start program of a dead service once its grace period is over", func() {
			runPy(setup + `
for p in check_restarts(LOG): p.wait()
print(RESTARTS['pg_janitor'])
`)
			Expect(starts()).To(Equal(1))
		})

		It("leaves a service alone during its start grace period", func() {
			runPy(setup + `
STARTED_AT['pg_janitor'] = time.time()
for p in check_restarts(LOG): p.wait()
`)
			Expect(starts()).To(Equal(0))
		})

		It("does not restart while a dependency is down", func() {
			runPy(setup + `
open(monit_services()['postgres']['pidfile'],'w').write('999999')
STARTED_AT['postgres'] = time.time()
for p in check_restarts(LOG): p.wait()
`)
			Expect(starts()).To(Equal(0))
		})

		It("stops restarting after the retry limit", func() {
			runPy(setup + `
RESTARTS['pg_janitor'] = MAX_RESTARTS
for p in check_restarts(LOG): p.wait()
`)
			Expect(starts()).To(Equal(0))
		})

		It("does not restart services that were stopped", func() {
			runPy(setup + `
STARTED.discard('pg_janitor')
for p in check_restarts(LOG): p.wait()
`)
			Expect(starts()).To(Equal(0))
		})
	})

	It("is wired into start_svc, stop requests and the status XML", func() {
		script := vm.RenderQEMUExt4InitForTest()
		Expect(script).To(ContainSubstring("is_monitrc_managed(svc)"))
		Expect(script).To(ContainSubstring("action=stop"))
		Expect(script).To(ContainSubstring("monit_status(s)"))
		Expect(script).To(ContainSubstring("target=watchdog_loop"))
	})
})
