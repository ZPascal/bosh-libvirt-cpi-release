package vm

// InjectMbusCertForTest exposes injectMbusCert for unit tests only.
func (f Factory) InjectMbusCertForTest(envBytes []byte) []byte {
	return f.injectMbusCert(envBytes)
}

// RenderQEMUExt4InitForTest renders the kernel-boot /bosh-init script with
// empty network/wrapper snippets, for tests of the embedded monit stub.
func RenderQEMUExt4InitForTest() string {
	return string(renderAsset("assets/init/qemu-ext4.sh.tmpl", struct {
		QemuNetScript    string
		QemuPgDNAT       string
		CPIWrapperScript string
	}{}))
}

// RenderLXCInitForTest renders one of the LXC init templates
// (lxc-dhcp, lxc-deploy, lxc-director) with placeholder network values.
func RenderLXCInitForTest(name string) string {
	return string(renderAsset("assets/init/"+name+".sh.tmpl", struct {
		StaticCIDR string
		StaticGW   string
		StaticIP   string
		MonitStub  string
	}{"10.0.0.2/24", "10.0.0.1", "10.0.0.2", ""}))
}
