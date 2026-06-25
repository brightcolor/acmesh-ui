package acme

import (
	"testing"
	"time"
)

func TestReissueFromConf(t *testing.T) {
	cases := []struct {
		name    string
		conf    map[string]string
		wantCh  string
		wantWR  string
		wantDNS string
		wantKey string
	}{
		{"dns", map[string]string{"Le_Webroot": "dns_cf", "Le_Keylength": "ec-384"}, "dns", "", "dns_cf", "ec-384"},
		{"webroot", map[string]string{"Le_Webroot": "/var/www/html"}, "webroot", "/var/www/html", "", "fallback"},
		{"standalone-no", map[string]string{"Le_Webroot": "no"}, "standalone", "", "", "fallback"},
		{"standalone-empty", map[string]string{}, "standalone", "", "", "fallback"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := reissueFromConf(c.conf, "fallback")
			if h.Challenge != c.wantCh {
				t.Errorf("challenge: got %s want %s", h.Challenge, c.wantCh)
			}
			if h.Webroot != c.wantWR {
				t.Errorf("webroot: got %s want %s", h.Webroot, c.wantWR)
			}
			if h.DNSCode != c.wantDNS {
				t.Errorf("dnscode: got %s want %s", h.DNSCode, c.wantDNS)
			}
			if h.KeyType != c.wantKey {
				t.Errorf("keytype: got %s want %s", h.KeyType, c.wantKey)
			}
		})
	}
}

func TestParseEpoch(t *testing.T) {
	if got := parseEpoch("1782000000"); got.IsZero() {
		t.Fatalf("expected valid epoch")
	} else if got.Unix() != 1782000000 {
		t.Fatalf("epoch mismatch: %v", got)
	}
	for _, bad := range []string{"", "abc", "0", "-5"} {
		if !parseEpoch(bad).IsZero() {
			t.Errorf("expected zero time for %q", bad)
		}
	}
	_ = time.Now
}
