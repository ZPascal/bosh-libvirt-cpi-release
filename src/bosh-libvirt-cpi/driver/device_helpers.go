package driver

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	libvirt "libvirt.org/go/libvirt"
)

// isDeviceMissing reports whether a detach failed because the device is no
// longer in the domain (e.g. a retried detach_disk after an earlier detach
// already removed it). Newer libvirt returns VIR_ERR_DEVICE_MISSING; older
// versions report "operation failed: disk vdX not found".
func isDeviceMissing(err error) bool {
	var lverr libvirt.Error
	if !errors.As(err, &lverr) {
		return false
	}
	if lverr.Code == libvirt.ERR_DEVICE_MISSING {
		return true
	}
	return lverr.Code == libvirt.ERR_OPERATION_FAILED && strings.Contains(lverr.Message, "not found")
}

type diskTargetXML struct {
	Target struct {
		Dev string `xml:"dev,attr"`
	} `xml:"target"`
}

// diskTargetFromXML returns the <target dev> of a <disk> device, or "".
func diskTargetFromXML(deviceXML string) string {
	var d struct {
		XMLName xml.Name `xml:"disk"`
		diskTargetXML
	}
	if err := xml.Unmarshal([]byte(deviceXML), &d); err != nil {
		return ""
	}
	return d.Target.Dev
}

// domainHasTarget reports whether the domain XML still has a disk at target.
func domainHasTarget(domainXML, target string) bool {
	var dom struct {
		Disks []diskTargetXML `xml:"devices>disk"`
	}
	if err := xml.Unmarshal([]byte(domainXML), &dom); err != nil {
		return false
	}
	for _, d := range dom.Disks {
		if d.Target.Dev == target {
			return true
		}
	}
	return false
}

// hostArchFromCapabilities returns <capabilities><host><cpu><arch>, with
// arm64 spelled aarch64 and amd64 spelled x86_64 (the guest arch names).
func hostArchFromCapabilities(capsXML string) (string, error) {
	var caps struct {
		Arch string `xml:"host>cpu>arch"`
	}
	if err := xml.Unmarshal([]byte(capsXML), &caps); err != nil {
		return "", fmt.Errorf("parsing libvirt capabilities: %w", err)
	}
	switch arch := strings.TrimSpace(caps.Arch); arch {
	case "":
		return "", errors.New("libvirt capabilities report no host CPU architecture")
	case "arm64":
		return "aarch64", nil
	case "amd64":
		return "x86_64", nil
	default:
		return arch, nil
	}
}
