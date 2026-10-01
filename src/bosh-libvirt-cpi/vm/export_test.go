package vm

// InjectMbusCertForTest exposes injectMbusCert for unit tests only.
func (f Factory) InjectMbusCertForTest(envBytes []byte) []byte {
	return f.injectMbusCert(envBytes)
}
