package dnsproviders

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bright-color/acmesh-ui/internal/secrets"
	"github.com/bright-color/acmesh-ui/internal/storage"
)

const sampleAccountConf = `
ACCOUNT_EMAIL='admin@example.com'
SAVED_CF_Token='cf-secret-token-value'
SAVED_CF_Account_ID='acct-1234'
SAVED_HETZNER_Token='hetzner-secret'
LOG_LEVEL='1'
`

func writeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "account.conf"), []byte(sampleAccountConf), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestDetectDNS(t *testing.T) {
	home := writeHome(t)
	det, err := DetectDNS(home)
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]Detected{}
	for _, d := range det {
		byCode[d.Code] = d
	}
	cf, ok := byCode["dns_cf"]
	if !ok {
		t.Fatalf("expected dns_cf detected, got %+v", det)
	}
	if _, ok := byCode["dns_hetzner"]; !ok {
		t.Fatalf("expected dns_hetzner detected")
	}
	// Secret value must be masked; non-secret account id must be visible.
	var tokenVar, acctVar *DetectedVar
	for i := range cf.Vars {
		switch cf.Vars[i].Name {
		case "CF_Token":
			tokenVar = &cf.Vars[i]
		case "CF_Account_ID":
			acctVar = &cf.Vars[i]
		}
	}
	if tokenVar == nil || tokenVar.MaskedValue != secrets.Redaction {
		t.Fatalf("CF_Token must be masked, got %+v", tokenVar)
	}
	if acctVar == nil || acctVar.MaskedValue != "acct-1234" {
		t.Fatalf("CF_Account_ID should be visible, got %+v", acctVar)
	}
}

func TestDetectDNSMissingFile(t *testing.T) {
	det, err := DetectDNS(t.TempDir())
	if err != nil {
		t.Fatalf("missing account.conf should not error: %v", err)
	}
	if len(det) != 0 {
		t.Fatalf("expected no detections")
	}
}

func TestImportAndAcmeSavedEnv(t *testing.T) {
	home := writeHome(t)
	db, err := storage.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cipher, err := secrets.NewCipher([]byte("test-key-material"))
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db, cipher)

	// Import copies SAVED_ values into the encrypted store.
	p, err := store.ImportFromAccountConf(home, "dns_cf")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if p.Source != SourceManaged {
		t.Fatalf("imported provider should be managed, got %s", p.Source)
	}
	code, env, err := store.DecryptedEnv(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if code != "dns_cf" || env["CF_Token"] != "cf-secret-token-value" {
		t.Fatalf("decrypted env wrong: %s %v", code, env)
	}

	// An acme_saved (reference) provider stores no env.
	ref, err := store.Create(Input{Name: "ref", Code: "dns_cf", Source: SourceAcmeSaved, Env: map[string]string{"CF_Token": "ignored"}})
	if err != nil {
		t.Fatalf("create ref: %v", err)
	}
	if ref.Source != SourceAcmeSaved || len(ref.Env) != 0 {
		t.Fatalf("acme_saved provider must hold no env: %+v", ref)
	}
	_, refEnv, err := store.DecryptedEnv(ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refEnv) != 0 {
		t.Fatalf("acme_saved DecryptedEnv must be empty, got %v", refEnv)
	}
}
