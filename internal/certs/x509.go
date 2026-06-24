package certs

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

// ParsedCert holds the fields extracted from a leaf certificate PEM.
type ParsedCert struct {
	MainDomain  string
	SANs        []string
	Wildcard    bool
	NotBefore   string
	NotAfter    string
	Issuer      string
	Serial      string
	Fingerprint string
	KeyType     string
}

// ParseCertFile reads and parses a PEM certificate file, returning the leaf
// certificate's metadata.
func ParseCertFile(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cert %s: %w", path, err)
	}
	return ParseCertPEM(data)
}

// ParseCertPEM parses the first CERTIFICATE block found in pemData.
func ParseCertPEM(pemData []byte) (*x509.Certificate, error) {
	rest := pemData
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("no PEM CERTIFICATE block found")
		}
		if block.Type == "CERTIFICATE" {
			return x509.ParseCertificate(block.Bytes)
		}
	}
}

// Fingerprint returns the SHA-256 fingerprint as colon-separated hex.
func Fingerprint(c *x509.Certificate) string {
	sum := sha256.Sum256(c.Raw)
	hexStr := hex.EncodeToString(sum[:])
	var b strings.Builder
	for i := 0; i < len(hexStr); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(hexStr[i : i+2])
	}
	return strings.ToUpper(b.String())
}

// KeyTypeOf derives a human readable key type from the certificate's public
// key (e.g. "ec-256", "RSA-2048").
func KeyTypeOf(c *x509.Certificate) string {
	switch c.PublicKeyAlgorithm {
	case x509.RSA:
		if k, ok := c.PublicKey.(interface{ Size() int }); ok {
			return fmt.Sprintf("RSA-%d", k.Size()*8)
		}
		return "RSA"
	case x509.ECDSA:
		if k, ok := c.PublicKey.(*ecdsa.PublicKey); ok && k.Curve != nil {
			return fmt.Sprintf("ec-%d", k.Curve.Params().BitSize)
		}
		return "ECDSA"
	case x509.Ed25519:
		return "Ed25519"
	default:
		return c.PublicKeyAlgorithm.String()
	}
}

// HasWildcard reports whether any DNS name is a wildcard.
func HasWildcard(names []string) bool {
	for _, n := range names {
		if strings.HasPrefix(n, "*.") {
			return true
		}
	}
	return false
}
