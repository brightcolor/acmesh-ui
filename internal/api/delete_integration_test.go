package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/config"
	"github.com/bright-color/acmesh-ui/internal/jobs"
	"github.com/bright-color/acmesh-ui/internal/secrets"
	"github.com/bright-color/acmesh-ui/internal/storage"
)

// makeCertPEMForAPI builds a short-lived self-signed EC cert for the domain.
func makeCertPEMForAPI(t *testing.T, domain string) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(20 * 24 * time.Hour),
		DNSNames:     []string{domain, "www." + domain},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), key
}

// buildFakeAcme compiles a tiny program that prints its args and exits 0, so the
// remove job succeeds and the purge step runs.
func buildFakeAcme(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	prog := "package main\nimport (\"fmt\";\"os\";\"strings\")\nfunc main(){ fmt.Println(\"fake-acme \"+strings.Join(os.Args[1:],\" \")); os.Exit(0) }\n"
	if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "acme")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, src)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake acme: %v\n%s", err, b)
	}
	return out
}

// makeCertDir writes a parseable EC cert + conf into home/<dir>.
func makeCertDir(t *testing.T, home, dirName, domain string) {
	t.Helper()
	d := filepath.Join(home, dirName)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	pem, _ := makeCertPEMForAPI(t, domain)
	for _, f := range []string{domain + ".cer", "fullchain.cer", "ca.cer"} {
		if err := os.WriteFile(filepath.Join(d, f), pem, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	conf := "Le_Domain='" + domain + "'\nLe_Webroot='dns_cf'\nLe_Keylength='ec-256'\n"
	if err := os.WriteFile(filepath.Join(d, domain+".conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestHandlers(t *testing.T, home, acmeBin string) *Handlers {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	masker := secrets.NewMasker()
	builder := acme.Builder{DefaultKeyType: "ec-256"}
	client := acme.NewClient(acmeBin, home, builder)
	jm := jobs.NewManager(client, masker, db, 2, time.Minute)
	cfg := config.Default()
	cfg.Acme.Home = home
	cfg.Acme.Binary = acmeBin
	return &Handlers{
		Cfg:     cfg,
		Client:  client,
		Scanner: acme.NewScanner(home, 30),
		Builder: builder,
		Jobs:    jm,
		Masker:  masker,
		Started: time.Now(),
	}
}

func TestDeleteCertWithPurge(t *testing.T) {
	acmeBin := buildFakeAcme(t)
	home := t.TempDir()
	makeCertDir(t, home, "example.com_ecc", "example.com")

	h := newTestHandlers(t, home, acmeBin)

	// Sanity: the cert is listed (this also primes the cert cache, so the test
	// below verifies the cache is invalidated when the job completes).
	if _, ok := h.findCert("example.com"); !ok {
		t.Fatalf("cert not found before delete")
	}

	req := httptest.NewRequest("DELETE", "/api/certs/example.com?purge=1", nil)
	req.SetPathValue("id", "example.com")
	w := httptest.NewRecorder()
	h.DeleteCert(w, req)
	if w.Code != 202 {
		t.Fatalf("delete status: got %d body %s", w.Code, w.Body.String())
	}

	// Wait for the job to finish and the directory to be purged.
	dir := filepath.Join(home, "example.com_ecc")
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("domain directory was not purged: %v", err)
	}

	// A CACHED read (not forced) must already reflect the deletion, proving the
	// job-completion hook invalidated the primed cache.
	list, err := h.certs(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list {
		if c.MainDomain == "example.com" {
			t.Fatalf("cert still present in cached list after delete+purge (cache not invalidated on completion)")
		}
	}
}

func TestDeleteCertNoPurgeKeepsFiles(t *testing.T) {
	acmeBin := buildFakeAcme(t)
	home := t.TempDir()
	makeCertDir(t, home, "example.com_ecc", "example.com")
	h := newTestHandlers(t, home, acmeBin)

	req := httptest.NewRequest("DELETE", "/api/certs/example.com", nil)
	req.SetPathValue("id", "example.com")
	w := httptest.NewRecorder()
	h.DeleteCert(w, req)
	if w.Code != 202 {
		t.Fatalf("delete status: got %d body %s", w.Code, w.Body.String())
	}
	// Give the job time to run.
	time.Sleep(1 * time.Second)
	// Without purge the directory remains (acme.sh --remove keeps files).
	if _, err := os.Stat(filepath.Join(home, "example.com_ecc")); err != nil {
		t.Fatalf("expected files to remain without purge: %v", err)
	}
}
