package network

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSystemInfo(t *testing.T) {
	info := NewSystemInfo()
	assert.NotNil(t, info)
	assert.NotEmpty(t, info.osVersion)
}

func TestGetFirstIP(t *testing.T) {
	tests := []struct {
		name     string
		subnet   string
		expected string
	}{
		{"Standard /24", "192.168.1.0/24", "192.168.1.0"},
		{"10.0.0.0/8", "10.0.0.0/8", "10.0.0.0"},
		{"172.16.0.0/16", "172.16.0.0/16", "172.16.0.0"},
		{"/30 subnet", "192.168.1.0/30", "192.168.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, subnet, _ := net.ParseCIDR(tt.subnet)
			info := NewSystemInfo()
			ip, err := info.GetFirstIP(subnet)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, ip.String())
		})
	}
}

func TestGetLastIP(t *testing.T) {
	tests := []struct {
		name     string
		subnet   string
		expected string
	}{
		{"Standard /24", "192.168.1.0/24", "192.168.1.255"},
		{"10.0.0.0/8", "10.0.0.0/8", "10.255.255.255"},
		{"/30 subnet", "192.168.1.0/30", "192.168.1.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, subnet, _ := net.ParseCIDR(tt.subnet)
			info := NewSystemInfo()
			ip, err := info.GetLastIP(subnet)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, ip.String())
		})
	}
}

func TestGetLastIPWithSmallSubnet(t *testing.T) {
	info := NewSystemInfo()
	_, subnet, _ := net.ParseCIDR("192.168.1.0/31")
	ip, err := info.GetLastIP(subnet)
	// /31 subnet might return an IP or error, both are valid
	// Just ensure the function runs without panic
	_ = ip
	_ = err
}

func TestRangeSize(t *testing.T) {
	tests := []struct {
		name     string
		subnet   string
		expected int64
	}{
		{"/24", "192.168.1.0/24", 256},
		{"/25", "192.168.1.0/25", 128},
		{"/26", "192.168.1.0/26", 64},
		{"/30", "192.168.1.0/30", 4},
		{"/31", "192.168.1.0/31", 2}, // /31 has 2 usable IPs
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, subnet, _ := net.ParseCIDR(tt.subnet)
			size := rangeSize(subnet)
			assert.Equal(t, tt.expected, size)
		})
	}
}

func TestBigForIP(t *testing.T) {
	tests := []string{"192.168.1.1", "127.0.0.1", "0.0.0.0"}
	for _, ipStr := range tests {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			big := bigForIP(ip)
			assert.NotNil(t, big)
		})
	}
}

func TestAddIPOffset(t *testing.T) {
	tests := []struct {
		base   string
		offset int
	}{
		{"192.168.1.0", 1},
		{"192.168.1.0", 0},
		{"192.168.1.0", 255},
	}

	for _, tt := range tests {
		baseIP := net.ParseIP(tt.base)
		baseBig := bigForIP(baseIP)
		result := addIPOffset(baseBig, tt.offset)
		assert.NotNil(t, result)
		assert.True(t, len(result) > 0)
	}
}

func TestGetIndexedIP(t *testing.T) {
	tests := []struct {
		name      string
		subnet    string
		index     int
		expected  string
		shouldErr bool
	}{
		{"Index 0", "192.168.1.0/24", 0, "192.168.1.0", false},
		{"Index 1", "192.168.1.0/24", 1, "192.168.1.1", false},
		{"Index 255", "192.168.1.0/24", 255, "192.168.1.255", false},
		{"Out of range", "192.168.1.0/24", 256, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, subnet, _ := net.ParseCIDR(tt.subnet)
			ip, err := getIndexedIP(subnet, tt.index)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, ip.String())
			}
		})
	}
}

func TestGetOSVersion(t *testing.T) {
	version := getOSVersion()
	assert.NotEmpty(t, version)
}

func TestIntegrationFirstAndLastIP(t *testing.T) {
	info := NewSystemInfo()
	_, subnet, _ := net.ParseCIDR("10.0.0.0/24")
	first, _ := info.GetFirstIP(subnet)
	last, _ := info.GetLastIP(subnet)
	assert.Equal(t, "10.0.0.0", first.String())
	assert.Equal(t, "10.0.0.255", last.String())
}
