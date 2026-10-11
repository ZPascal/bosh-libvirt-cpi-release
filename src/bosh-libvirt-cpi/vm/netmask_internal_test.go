package vm

import "testing"

func TestNetmaskToPrefix(t *testing.T) {
	cases := []struct {
		netmask    string
		wantPrefix int
		wantOK     bool
	}{
		{"255.255.255.0", 24, true},
		{"255.255.0.0", 16, true},
		{"255.255.255.128", 25, true},
		{"255.255.255.255", 32, true},
		{"0.0.0.0", 0, true},
		{"", 0, false},
		{"not-an-ip", 0, false},
		// Non-contiguous masks are not valid prefixes.
		{"255.0.255.0", 0, false},
	}

	for _, c := range cases {
		prefix, ok := netmaskToPrefix(c.netmask)
		if ok != c.wantOK {
			t.Errorf("netmaskToPrefix(%q) ok = %v, want %v", c.netmask, ok, c.wantOK)
			continue
		}
		if ok && prefix != c.wantPrefix {
			t.Errorf("netmaskToPrefix(%q) = %d, want %d", c.netmask, prefix, c.wantPrefix)
		}
	}
}

func TestExtractNetworkFromEnvPrefix(t *testing.T) {
	// Static network with a /16 netmask should yield prefix 16, not the /24 default.
	env := []byte(`{"networks":{"default":{"ip":"10.0.5.6","gateway":"10.0.0.1","netmask":"255.255.0.0"}}}`)
	ip, gw, prefix := extractNetworkFromEnv(env)
	if ip != "10.0.5.6" || gw != "10.0.0.1" || prefix != 16 {
		t.Errorf("extractNetworkFromEnv = (%q,%q,%d), want (10.0.5.6,10.0.0.1,16)", ip, gw, prefix)
	}

	// Missing netmask falls back to /24.
	env = []byte(`{"networks":{"default":{"ip":"192.168.1.2","gateway":"192.168.1.1"}}}`)
	if _, _, prefix := extractNetworkFromEnv(env); prefix != 24 {
		t.Errorf("extractNetworkFromEnv prefix = %d, want 24 fallback", prefix)
	}

	// Dynamic network (no ip) returns empty with the default prefix.
	env = []byte(`{"networks":{"default":{"type":"dynamic"}}}`)
	if ip, _, prefix := extractNetworkFromEnv(env); ip != "" || prefix != 24 {
		t.Errorf("extractNetworkFromEnv = (%q,_,%d), want (\"\",_,24)", ip, prefix)
	}
}
