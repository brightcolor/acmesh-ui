package api

import (
	"testing"

	"github.com/bright-color/acmesh-ui/internal/certs"
)

func TestCertFilePathAllowList(t *testing.T) {
	c := certs.Cert{
		CertPath:      "/home/acme/.acme.sh/example.com/example.com.cer",
		FullchainPath: "/home/acme/.acme.sh/example.com/fullchain.cer",
		CAPath:        "/home/acme/.acme.sh/example.com/ca.cer",
		KeyPath:       "/home/acme/.acme.sh/example.com/example.com.key",
	}
	for _, f := range []string{"cert", "fullchain", "chain", "ca", "key"} {
		if p, _, ok := certFilePath(c, f); !ok || p == "" {
			t.Errorf("expected %q to resolve", f)
		}
	}
	if _, isKey, _ := certFilePath(c, "key"); !isKey {
		t.Errorf("key must be flagged as key")
	}
	for _, f := range []string{"", "passwd", "../../etc/passwd", "account.conf", "secret"} {
		if _, _, ok := certFilePath(c, f); ok {
			t.Errorf("expected %q to be rejected", f)
		}
	}
	// Missing file => not ok.
	empty := certs.Cert{}
	if _, _, ok := certFilePath(empty, "cert"); ok {
		t.Errorf("missing path must not resolve")
	}
}

func TestHostBelongsToCert(t *testing.T) {
	c := certs.Cert{MainDomain: "example.com", SANs: []string{"example.com", "www.example.com", "*.api.example.com"}}
	ok := []string{"example.com", "www.example.com", "foo.api.example.com"}
	for _, h := range ok {
		if !hostBelongsToCert(h, c) {
			t.Errorf("expected %q to belong", h)
		}
	}
	bad := []string{"evil.com", "example.org", "a.b.api.example.com", "attacker.example.com.evil.com"}
	for _, h := range bad {
		if hostBelongsToCert(h, c) {
			t.Errorf("expected %q to be rejected", h)
		}
	}
}

func TestDirWithin(t *testing.T) {
	home := "/home/acme/.acme.sh"
	if !dirWithin(home+"/example.com", home) {
		t.Errorf("subdir should be within")
	}
	for _, d := range []string{home, "/etc", "/home/acme/.acme.sh/../evil", ""} {
		if dirWithin(d, home) {
			t.Errorf("expected %q to be rejected", d)
		}
	}
}
