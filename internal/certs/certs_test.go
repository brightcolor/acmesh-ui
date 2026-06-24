package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestEvaluateStatus(t *testing.T) {
	now := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		notAfter  time.Time
		wantState Status
	}{
		{"valid", now.Add(60 * 24 * time.Hour), StatusValid},
		{"expiring", now.Add(10 * 24 * time.Hour), StatusExpiring},
		{"boundary-30", now.Add(30 * 24 * time.Hour), StatusExpiring},
		{"expired", now.Add(-1 * time.Hour), StatusExpired},
		{"unknown", time.Time{}, StatusUnknown},
	}
	for _, c := range cases {
		got, _ := EvaluateStatus(c.notAfter, now, 30)
		if got != c.wantState {
			t.Errorf("%s: got %s want %s", c.name, got, c.wantState)
		}
	}
}

func TestParseCertPEMAndFingerprint(t *testing.T) {
	pemBytes, want := makeTestCert(t, []string{"example.com", "*.example.com"})
	c, err := ParseCertPEM(pemBytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.DNSNames[0] != want {
		t.Fatalf("dns name: got %v", c.DNSNames)
	}
	if !HasWildcard(c.DNSNames) {
		t.Errorf("expected wildcard detected")
	}
	if fp := Fingerprint(c); len(fp) == 0 {
		t.Errorf("empty fingerprint")
	}
	if kt := KeyTypeOf(c); kt != "ec-256" {
		t.Errorf("key type: got %s want ec-256", kt)
	}
}

func makeTestCert(t *testing.T, dnsNames []string) ([]byte, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsNames[0]},
		Issuer:       pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
		DNSNames:     dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return pemBytes, dnsNames[0]
}
