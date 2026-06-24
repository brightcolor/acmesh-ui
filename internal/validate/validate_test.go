package validate

import "testing"

func TestDomain(t *testing.T) {
	valid := []string{"example.com", "www.example.com", "a.b.c.example.com", "xn--mnchen-3ya.de"}
	for _, d := range valid {
		if err := Domain(d); err != nil {
			t.Errorf("expected %q valid: %v", d, err)
		}
	}
	invalid := []string{
		"", "example", "exa mple.com", "example.com/path", "http://example.com",
		"example.com;rm -rf", "*.example.com", "-bad.com", "bad-.com",
		"a;b.com", "exam$ple.com", ".example.com", "example.com.",
	}
	for _, d := range invalid {
		if err := Domain(d); err == nil {
			t.Errorf("expected %q invalid", d)
		}
	}
}

func TestWildcard(t *testing.T) {
	if err := Wildcard("*.example.com"); err != nil {
		t.Errorf("valid wildcard rejected: %v", err)
	}
	for _, d := range []string{"example.com", "*.*.example.com", "*example.com", "*.", "*.bad-.com"} {
		if err := Wildcard(d); err == nil {
			t.Errorf("expected wildcard %q invalid", d)
		}
	}
}

func TestDomains(t *testing.T) {
	if err := Domains([]string{"example.com", "*.example.com"}); err != nil {
		t.Errorf("valid set rejected: %v", err)
	}
	if err := Domains(nil); err == nil {
		t.Errorf("empty set should fail")
	}
	if err := Domains([]string{"example.com", "example.com"}); err == nil {
		t.Errorf("duplicate should fail")
	}
}

func TestAbsolutePath(t *testing.T) {
	if err := AbsolutePath("/var/www/html", nil); err != nil {
		t.Errorf("valid path rejected: %v", err)
	}
	bad := []string{"", "relative/path", "/a/$(whoami)", "/a/`id`", "/a;b", "/a/../etc", "/a/&&b"}
	for _, p := range bad {
		if err := AbsolutePath(p, nil); err == nil {
			t.Errorf("expected path %q invalid", p)
		}
	}
	// allowed bases
	if err := AbsolutePath("/etc/ssl/example/key.pem", []string{"/etc/ssl"}); err != nil {
		t.Errorf("path under base rejected: %v", err)
	}
	if err := AbsolutePath("/root/secret", []string{"/etc/ssl"}); err == nil {
		t.Errorf("path outside base should fail")
	}
}

func TestProviderCode(t *testing.T) {
	for _, c := range []string{"dns_cf", "dns_hetzner", "dns_inwx"} {
		if err := ProviderCode(c); err != nil {
			t.Errorf("valid provider %q rejected: %v", c, err)
		}
	}
	for _, c := range []string{"", "cf", "dns_cf;rm", "dns_cf rm", "../dns_cf"} {
		if err := ProviderCode(c); err == nil {
			t.Errorf("expected provider %q invalid", c)
		}
	}
}

func TestHookName(t *testing.T) {
	if err := HookName("haproxy"); err != nil {
		t.Errorf("valid hook rejected: %v", err)
	}
	for _, h := range []string{"", "hap roxy", "hap;roxy", "hap$roxy"} {
		if err := HookName(h); err == nil {
			t.Errorf("expected hook %q invalid", h)
		}
	}
}

func TestKeyType(t *testing.T) {
	for _, k := range []string{"", "ec-256", "ec-384", "2048", "4096"} {
		if err := KeyType(k); err != nil {
			t.Errorf("valid key type %q rejected: %v", k, err)
		}
	}
	if err := KeyType("rsa-9999"); err == nil {
		t.Errorf("invalid key type accepted")
	}
}
