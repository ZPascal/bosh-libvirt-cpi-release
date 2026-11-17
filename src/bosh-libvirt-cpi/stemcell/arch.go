package stemcell

import "strings"

// Guest architectures the CPI can create VMs for.
const (
	ArchX86_64  = "x86_64"
	ArchAarch64 = "aarch64"
)

// NormalizeArch maps a stemcell's `architecture` cloud property to a guest
// architecture: arm64/aarch64 → aarch64, anything else (incl. empty) → x86_64.
func NormalizeArch(arch string) string {
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "arm64", "aarch64":
		return ArchAarch64
	default:
		return ArchX86_64
	}
}
