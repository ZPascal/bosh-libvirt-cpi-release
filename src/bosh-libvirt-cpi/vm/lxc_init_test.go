package vm_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/vm"
)

var _ = Describe("LXC init scripts", func() {
	// The script is the container's init: if redirecting its output into the
	// agent log fails, sh exits and libvirt shuts the container down before
	// the agent ever starts. Stemcells don't ship /var/vcap/bosh/log.
	for _, name := range []string{"lxc-dhcp", "lxc-deploy", "lxc-director"} {
		name := name
		It(name+" survives redirecting into the agent log on a stemcell without /var/vcap/bosh/log", func() {
			script := vm.RenderLXCInitForTest(name)
			lines := strings.Split(script, "\n")
			var preamble []string
			for _, l := range lines {
				preamble = append(preamble, l)
				if strings.HasPrefix(l, "exec >>") {
					break
				}
			}
			Expect(preamble[len(preamble)-1]).To(HavePrefix("exec >>/var/vcap/bosh/log/"))

			root, err := os.MkdirTemp("", "lxc-init-test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(root)

			body := strings.ReplaceAll(strings.Join(preamble, "\n"), "/var/vcap", root+"/var/vcap")
			body += "\necho INIT_CONTINUES\n"
			out, err := exec.Command("sh", "-c", body).CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(out))

			log, err := os.ReadFile(filepath.Join(root, "var/vcap/bosh/log/bosh-agent-init.log"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(log)).To(ContainSubstring("INIT_CONTINUES"))
		})
	}
})
