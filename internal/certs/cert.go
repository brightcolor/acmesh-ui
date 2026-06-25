// Package certs models certificates discovered on disk and their expiry status.
package certs

import "time"

// Status is the high-level health of a certificate.
type Status string

const (
	StatusValid    Status = "valid"
	StatusExpiring Status = "expiring"
	StatusExpired  Status = "expired"
	StatusError    Status = "error"
	StatusUnknown  Status = "unknown"
)

// Cert is a certificate discovered in the acme.sh home directory.
type Cert struct {
	ID            string    `json:"id"` // stable id (main domain)
	MainDomain    string    `json:"main_domain"`
	SANs          []string  `json:"sans"`
	Wildcard      bool      `json:"wildcard"`
	Status        Status    `json:"status"`
	DaysRemaining int       `json:"days_remaining"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	Issuer        string    `json:"issuer"`
	Serial        string    `json:"serial"`
	Fingerprint   string    `json:"fingerprint"`
	KeyType       string    `json:"key_type"`
	CA            string    `json:"ca,omitempty"`
	Ecc           bool      `json:"ecc"`
	NextRenew     time.Time `json:"next_renew,omitempty"`

	// Filesystem layout inside the acme.sh home.
	DomainDir     string `json:"domain_dir"`
	CertPath      string `json:"cert_path"`
	KeyPath       string `json:"key_path"`
	FullchainPath string `json:"fullchain_path"`
	CAPath        string `json:"ca_path"`
	ConfPath      string `json:"conf_path"`

	// Install/deploy hints parsed from the acme.sh domain .conf, if present.
	Install *InstallConfig `json:"install,omitempty"`

	// Reissue carries the parameters needed to pre-fill the issue wizard for an
	// in-place re-issue, derived from the acme.sh domain .conf.
	Reissue *ReissueHint `json:"reissue,omitempty"`

	// LastRenewStatus / LastAction are filled from job history by the API layer.
	LastRenewStatus string    `json:"last_renew_status,omitempty"`
	LastAction      string    `json:"last_action,omitempty"`
	ParseError      string    `json:"parse_error,omitempty"`
	ParsedAt        time.Time `json:"parsed_at"`
}

// ReissueHint holds the original issuance parameters so the UI can pre-fill a
// re-issue. Challenge is one of "webroot", "standalone" or "dns".
type ReissueHint struct {
	Challenge string `json:"challenge"`
	Webroot   string `json:"webroot,omitempty"`
	DNSCode   string `json:"dns_code,omitempty"`
	KeyType   string `json:"key_type,omitempty"`
}

// InstallConfig captures the install-cert target paths from the acme.sh conf.
type InstallConfig struct {
	KeyFile       string `json:"key_file,omitempty"`
	CertFile      string `json:"cert_file,omitempty"`
	FullchainFile string `json:"fullchain_file,omitempty"`
	CAFile        string `json:"ca_file,omitempty"`
	ReloadCmd     string `json:"reload_cmd,omitempty"` // masked before display
}

// EvaluateStatus computes Status and DaysRemaining from NotAfter relative to
// now, treating certs within expiringSoonDays of expiry as "expiring".
func EvaluateStatus(notAfter time.Time, now time.Time, expiringSoonDays int) (Status, int) {
	if notAfter.IsZero() {
		return StatusUnknown, 0
	}
	remaining := notAfter.Sub(now)
	days := int(remaining.Hours() / 24)
	switch {
	case remaining <= 0:
		return StatusExpired, days
	case days <= expiringSoonDays:
		return StatusExpiring, days
	default:
		return StatusValid, days
	}
}
