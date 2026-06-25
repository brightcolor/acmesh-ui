package certs

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"
)

// ServedCert describes the certificate a live TLS endpoint presents.
type ServedCert struct {
	Reachable   bool      `json:"reachable"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Subject     string    `json:"subject,omitempty"`
	SANs        []string  `json:"sans,omitempty"`
	Issuer      string    `json:"issuer,omitempty"`
	NotAfter    time.Time `json:"not_after,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// CheckEndpoint dials host:port over TLS (SNI = host) and reports the leaf
// certificate the server presents. Trust is intentionally NOT validated - we
// only want to read what is being served (e.g. to detect "installed but the
// service was not reloaded"). The dial is bounded by a short timeout.
func CheckEndpoint(ctx context.Context, host string, port int) ServedCert {
	res := ServedCert{Host: host, Port: port}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conf := &tls.Config{ServerName: host, InsecureSkipVerify: true} //nolint:gosec // read-only inspection
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, conf)
	if err != nil {
		res.Error = fmt.Sprintf("connection to %s failed: %v", addr, err)
		return res
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		res.Error = "no certificate presented by the server"
		return res
	}
	leaf := state.PeerCertificates[0]
	res.Reachable = true
	res.Subject = leaf.Subject.CommonName
	res.SANs = leaf.DNSNames
	res.Issuer = leaf.Issuer.CommonName
	if res.Issuer == "" && len(leaf.Issuer.Organization) > 0 {
		res.Issuer = leaf.Issuer.Organization[0]
	}
	res.NotAfter = leaf.NotAfter
	res.Fingerprint = Fingerprint(leaf)
	return res
}
