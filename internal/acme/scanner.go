package acme

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bright-color/acmesh-ui/internal/certs"
)

// Scanner discovers certificates by walking the acme.sh home directory and
// parsing the certificate files directly with crypto/x509.
type Scanner struct {
	Home             string
	ExpiringSoonDays int
}

// NewScanner returns a Scanner for the given acme.sh home.
func NewScanner(home string, expiringSoonDays int) *Scanner {
	if expiringSoonDays <= 0 {
		expiringSoonDays = 30
	}
	return &Scanner{Home: home, ExpiringSoonDays: expiringSoonDays}
}

// Scan walks the home directory and returns one Cert per discovered domain
// directory. Directories without a leaf certificate are skipped. Parse errors
// are recorded on the Cert rather than aborting the scan.
func (s *Scanner) Scan() ([]certs.Cert, error) {
	entries, err := os.ReadDir(s.Home)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var result []certs.Cert
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "ca" || name == "deploy" || name == "dnsapi" || name == "notify" {
			continue
		}
		domainDir := filepath.Join(s.Home, name)
		c, ok := s.scanDomainDir(domainDir, name, now)
		if ok {
			result = append(result, c)
		}
	}
	return result, nil
}

// scanDomainDir builds a Cert from a single domain directory. The bool result
// is false when the directory does not look like an acme.sh cert dir.
func (s *Scanner) scanDomainDir(dir, dirName string, now time.Time) (certs.Cert, bool) {
	// EC certs live in "<domain>_ecc"; strip the suffix for the logical domain.
	domain := strings.TrimSuffix(dirName, "_ecc")

	certPath := firstExisting(
		filepath.Join(dir, domain+".cer"),
		filepath.Join(dir, "fullchain.cer"),
	)
	if certPath == "" {
		return certs.Cert{}, false
	}

	c := certs.Cert{
		ID:            domain,
		MainDomain:    domain,
		Ecc:           strings.HasSuffix(dirName, "_ecc"),
		DomainDir:     dir,
		CertPath:      filepath.Join(dir, domain+".cer"),
		KeyPath:       filepath.Join(dir, domain+".key"),
		FullchainPath: filepath.Join(dir, "fullchain.cer"),
		CAPath:        filepath.Join(dir, "ca.cer"),
		ConfPath:      filepath.Join(dir, domain+".conf"),
		ParsedAt:      now,
	}
	clearMissing(&c)

	leaf, err := certs.ParseCertFile(certPath)
	if err != nil {
		c.Status = certs.StatusError
		c.ParseError = err.Error()
	} else {
		c.SANs = leaf.DNSNames
		c.Wildcard = certs.HasWildcard(leaf.DNSNames)
		c.NotBefore = leaf.NotBefore
		c.NotAfter = leaf.NotAfter
		c.Issuer = leaf.Issuer.CommonName
		if c.Issuer == "" && len(leaf.Issuer.Organization) > 0 {
			c.Issuer = leaf.Issuer.Organization[0]
		}
		c.Serial = leaf.SerialNumber.Text(16)
		c.Fingerprint = certs.Fingerprint(leaf)
		c.KeyType = certs.KeyTypeOf(leaf)
		c.Status, c.DaysRemaining = certs.EvaluateStatus(leaf.NotAfter, now, s.ExpiringSoonDays)
		if len(leaf.Subject.CommonName) > 0 {
			c.MainDomain = leaf.Subject.CommonName
			c.ID = leaf.Subject.CommonName
		}
	}

	// Parse the per-domain .conf for CA, install hints and re-issue parameters.
	if data, err := os.ReadFile(c.ConfPath); err == nil {
		conf := ParseDomainConf(string(data))
		if ca := conf["Le_API"]; ca != "" {
			c.CA = ca
		}
		c.Install = installFromConf(conf)
		c.Reissue = reissueFromConf(conf, c.KeyType)
		if ts := parseEpoch(conf["Le_NextRenewTime"]); !ts.IsZero() {
			c.NextRenew = ts
		}
	}

	return c, true
}

// reissueFromConf derives the original issuance parameters used to pre-fill the
// re-issue wizard. acme.sh stores the challenge in Le_Webroot:
//   - "dns_xxx"      -> DNS-01 via that provider code
//   - an absolute /path -> HTTP-01 webroot
//   - "no" / empty   -> standalone
func reissueFromConf(conf map[string]string, keyType string) *certs.ReissueHint {
	h := &certs.ReissueHint{KeyType: keyType}
	if kl := conf["Le_Keylength"]; kl != "" {
		h.KeyType = kl
	}
	w := strings.TrimSpace(conf["Le_Webroot"])
	switch {
	case strings.HasPrefix(w, "dns_"):
		h.Challenge = "dns"
		h.DNSCode = w
	case strings.HasPrefix(w, "/"):
		h.Challenge = "webroot"
		h.Webroot = w
	default:
		h.Challenge = "standalone"
	}
	return h
}

// parseEpoch converts a unix-seconds string to a time.Time (zero on failure).
func parseEpoch(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(n, 0)
}

func installFromConf(conf map[string]string) *certs.InstallConfig {
	ic := &certs.InstallConfig{
		KeyFile:       conf["Le_RealKeyPath"],
		CertFile:      conf["Le_RealCertPath"],
		FullchainFile: conf["Le_RealFullChainPath"],
		CAFile:        conf["Le_RealCACertPath"],
		ReloadCmd:     conf["Le_ReloadCmd"],
	}
	if ic.KeyFile == "" && ic.CertFile == "" && ic.FullchainFile == "" && ic.ReloadCmd == "" {
		return nil
	}
	return ic
}

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// clearMissing blanks out file path fields whose targets do not exist so the
// UI does not show paths that are not actually present.
func clearMissing(c *certs.Cert) {
	check := func(p *string) {
		if *p == "" {
			return
		}
		if fi, err := os.Stat(*p); err != nil || fi.IsDir() {
			*p = ""
		}
	}
	check(&c.CertPath)
	check(&c.KeyPath)
	check(&c.FullchainPath)
	check(&c.CAPath)
	check(&c.ConfPath)
}
