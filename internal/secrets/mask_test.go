package secrets

import "testing"

func TestMaskerLiterals(t *testing.T) {
	m := NewMasker()
	m.Add("super-secret-token-123")
	m.Add("ab") // too short, ignored

	in := "Using CF_Token super-secret-token-123 for issue"
	got := m.Mask(in)
	if want := "Using CF_Token " + Redaction + " for issue"; got != want {
		t.Fatalf("literal mask: got %q want %q", got, want)
	}
	if m.Mask("ab cd") != "ab cd" {
		t.Fatalf("short literal should not be masked")
	}
}

func TestMaskKeyValueLines(t *testing.T) {
	// Secret-looking NAMEs are redacted by heuristic; non-secret identifiers
	// (account id, log level) are left intact. Actual secret VALUES are
	// additionally masked via literal registration (see TestMaskerLiterals).
	in := "CF_Token=abcdef123456\nHETZNER_Api_Key=zzz999\nLOG_LEVEL=debug\n"
	got := NewMasker().Mask(in)
	if want := "CF_Token=" + Redaction + "\nHETZNER_Api_Key=" + Redaction + "\nLOG_LEVEL=debug\n"; got != want {
		t.Fatalf("kv mask:\n got %q\nwant %q", got, want)
	}
}

func TestMaskAuthorizationHeader(t *testing.T) {
	cases := []string{
		"Authorization: Bearer eyJhbGciOi.something",
		"authorization=Token deadbeef",
		"X-Auth-Key: abc123def456",
	}
	for _, c := range cases {
		got := NewMasker().Mask(c)
		if !containsRedaction(got) {
			t.Fatalf("header not masked: %q -> %q", c, got)
		}
		if got == c {
			t.Fatalf("header unchanged: %q", c)
		}
	}
}

func TestMaskValueHint(t *testing.T) {
	if MaskValue("anything") != Redaction {
		t.Fatalf("MaskValue should fully redact")
	}
	if MaskValue("") != "" {
		t.Fatalf("empty stays empty")
	}
	if got := MaskValueHint("abcdefghij"); got != "ab"+Redaction+"ij" {
		t.Fatalf("hint: got %q", got)
	}
	if MaskValueHint("short") != Redaction {
		t.Fatalf("short value should fully redact")
	}
}

func TestIsSecretEnvName(t *testing.T) {
	secret := []string{"CF_Token", "HETZNER_Token", "AWS_SECRET_ACCESS_KEY", "api_password"}
	plain := []string{"CF_Account_ID", "LOG_LEVEL", "ZONE", "TTL"}
	for _, s := range secret {
		if !IsSecretEnvName(s) {
			t.Errorf("expected %q to be secret", s)
		}
	}
	for _, p := range plain {
		if IsSecretEnvName(p) {
			t.Errorf("expected %q to be non-secret", p)
		}
	}
}

func containsRedaction(s string) bool {
	for i := 0; i+len(Redaction) <= len(s); i++ {
		if s[i:i+len(Redaction)] == Redaction {
			return true
		}
	}
	return false
}
