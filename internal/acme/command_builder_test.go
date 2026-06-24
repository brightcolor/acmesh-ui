package acme

import (
	"strings"
	"testing"
)

func argsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestIssueWebroot(t *testing.T) {
	b := Builder{DefaultKeyType: "ec-256"}
	cmd, err := b.Issue(IssueSpec{
		Domains:   []string{"example.com", "www.example.com"},
		Challenge: ChallengeWebroot,
		Webroot:   "/var/www/example",
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	want := []string{"--issue", "-d", "example.com", "-d", "www.example.com", "-w", "/var/www/example", "--keylength", "ec-256"}
	if !argsEqual(cmd.Args, want) {
		t.Fatalf("got %v want %v", cmd.Args, want)
	}
}

func TestIssueDNSWildcardWithEnv(t *testing.T) {
	b := Builder{}
	cmd, err := b.Issue(IssueSpec{
		Domains:   []string{"example.com", "*.example.com"},
		Challenge: ChallengeDNS,
		DNSCode:   "dns_cf",
		KeyType:   "ec-384",
		CA:        "letsencrypt",
		Staging:   true,
		DNSEnv:    map[string]string{"CF_Token": "secret123", "CF_Account_ID": "acct"},
	})
	if err != nil {
		t.Fatalf("issue dns: %v", err)
	}
	joined := strings.Join(cmd.Args, " ")
	for _, frag := range []string{"--dns dns_cf", "-d *.example.com", "--keylength ec-384", "--server letsencrypt", "--staging"} {
		if !strings.Contains(joined, frag) {
			t.Errorf("expected args to contain %q, got %q", frag, joined)
		}
	}
	// env must be deterministic & include the secret value (masking happens later)
	if !argsEqual(cmd.Env, []string{"CF_Account_ID=acct", "CF_Token=secret123"}) {
		t.Fatalf("env: got %v", cmd.Env)
	}
}

func TestIssueRejectsBadDomain(t *testing.T) {
	b := Builder{}
	_, err := b.Issue(IssueSpec{Domains: []string{"exa;mple.com"}, Challenge: ChallengeStandalone})
	if err == nil {
		t.Fatalf("expected bad domain to be rejected")
	}
}

func TestIssueRejectsBadProvider(t *testing.T) {
	b := Builder{}
	_, err := b.Issue(IssueSpec{Domains: []string{"example.com"}, Challenge: ChallengeDNS, DNSCode: "cf; rm -rf"})
	if err == nil {
		t.Fatalf("expected bad provider to be rejected")
	}
}

func TestRenew(t *testing.T) {
	b := Builder{}
	cmd, err := b.Renew("example.com", true)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !argsEqual(cmd.Args, []string{"--renew", "-d", "example.com", "--force"}) {
		t.Fatalf("got %v", cmd.Args)
	}
	if cmd.Action != "force-renew" {
		t.Fatalf("action: %s", cmd.Action)
	}
}

func TestInstallCertReloadAllowlist(t *testing.T) {
	b := Builder{}
	spec := InstallSpec{
		Domain:        "example.com",
		KeyFile:       "/etc/ssl/example/key.pem",
		FullchainFile: "/etc/ssl/example/fullchain.pem",
		ReloadCmd:     []string{"systemctl", "reload", "nginx"},
	}
	// not allowed -> rejected
	if _, err := b.InstallCert(spec, nil, false); err == nil {
		t.Fatalf("expected unverified reload to be rejected")
	}
	// allowed -> reloadcmd joined
	cmd, err := b.InstallCert(spec, nil, true)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "--reloadcmd systemctl reload nginx") {
		t.Fatalf("reloadcmd missing: %q", joined)
	}
}

func TestInstallCertBasePathEnforced(t *testing.T) {
	b := Builder{}
	spec := InstallSpec{Domain: "example.com", KeyFile: "/root/secret/key.pem"}
	if _, err := b.InstallCert(spec, []string{"/etc/ssl"}, false); err == nil {
		t.Fatalf("expected path outside base to be rejected")
	}
}

func TestDeploy(t *testing.T) {
	b := Builder{}
	cmd, err := b.Deploy("example.com", "haproxy", nil)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if !argsEqual(cmd.Args, []string{"--deploy", "-d", "example.com", "--deploy-hook", "haproxy"}) {
		t.Fatalf("got %v", cmd.Args)
	}
}

func TestSetDefaultCA(t *testing.T) {
	b := Builder{}
	if _, err := b.SetDefaultCA("letsencrypt"); err != nil {
		t.Fatalf("valid CA rejected: %v", err)
	}
	if _, err := b.SetDefaultCA("evil; rm"); err == nil {
		t.Fatalf("invalid CA accepted")
	}
}

func TestPreviewArgsQuoting(t *testing.T) {
	cmd := Command{Args: []string{"--issue", "-d", "*.example.com"}}
	got := cmd.PreviewArgs("/root/.acme.sh/acme.sh")
	if !strings.HasPrefix(got, "/root/.acme.sh/acme.sh --issue -d") {
		t.Fatalf("preview: %q", got)
	}
}
