package driver

// Test-only access to unexported helpers.
var (
	IsDeviceMissing   = isDeviceMissing
	DiskTargetFromXML = diskTargetFromXML
	DomainHasTarget   = domainHasTarget
)
