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
