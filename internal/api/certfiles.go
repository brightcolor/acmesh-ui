package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bright-color/acmesh-ui/internal/certs"
)

// certFilePath maps a logical file name to the cert's actual on-disk path. Only
// these four names are ever served - never an arbitrary path from the client.
func certFilePath(c certs.Cert, file string) (path string, isKey bool, ok bool) {
	switch file {
	case "cert":
		return c.CertPath, false, c.CertPath != ""
	case "fullchain":
		return c.FullchainPath, false, c.FullchainPath != ""
	case "chain", "ca":
		return c.CAPath, false, c.CAPath != ""
	case "key":
		return c.KeyPath, true, c.KeyPath != ""
	default:
		return "", false, false
	}
}

// CertPem handles GET /api/certs/{id}/pem?file=cert|fullchain|chain. The private
// key is never returned inline (download-only, with confirmation).
func (h *Handlers) CertPem(w http.ResponseWriter, r *http.Request) {
	c, ok := h.findCert(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "CERT_NOT_FOUND", "Certificate not found.", "")
		return
	}
	file := r.URL.Query().Get("file")
	path, isKey, ok := certFilePath(c, file)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_FILE", "Unknown or unavailable certificate file.", "file: "+file)
		return
	}
	if isKey {
		writeError(w, http.StatusForbidden, "KEY_INLINE_FORBIDDEN", "The private key cannot be viewed inline; use download with confirmation.", "")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "READ_FAILED", "Could not read the certificate file.", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": file, "path": path, "pem": string(data)})
}

// CertDownload handles GET /api/certs/{id}/download?file=...&confirm=1.
// Downloading the private key requires confirm=1 and is audit-logged.
func (h *Handlers) CertDownload(w http.ResponseWriter, r *http.Request) {
	c, ok := h.findCert(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "CERT_NOT_FOUND", "Certificate not found.", "")
		return
	}
	file := r.URL.Query().Get("file")
	path, isKey, ok := certFilePath(c, file)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_FILE", "Unknown or unavailable certificate file.", "file: "+file)
		return
	}
	if isKey && r.URL.Query().Get("confirm") != "1" {
		writeError(w, http.StatusForbidden, "KEY_DOWNLOAD_UNCONFIRMED",
			"Downloading the private key requires explicit confirmation.", "Add confirm=1 to proceed.")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "READ_FAILED", "Could not read the certificate file.", err.Error())
		return
	}
	if isKey {
		log.Printf("AUDIT: private key download for %s (%s)", c.MainDomain, path)
	}
	name := sanitizeFilename(c.MainDomain) + "-" + file + ".pem"
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// TLSCheck handles GET /api/certs/{id}/tls-check?host=&port=443. The host must be
// one of the certificate's own domains (prevents using the server as an SSRF
// probe against arbitrary hosts).
func (h *Handlers) TLSCheck(w http.ResponseWriter, r *http.Request) {
	c, ok := h.findCert(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "CERT_NOT_FOUND", "Certificate not found.", "")
		return
	}
	host := strings.TrimSpace(r.URL.Query().Get("host"))
	if host == "" {
		host = c.MainDomain
	}
	if !hostBelongsToCert(host, c) {
		writeError(w, http.StatusBadRequest, "HOST_NOT_IN_CERT",
			"The host must be one of this certificate's domains.", "host: "+host)
		return
	}
	port := 443
	if p := r.URL.Query().Get("port"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			writeError(w, http.StatusBadRequest, "INVALID_PORT", "Port must be between 1 and 65535.", "port: "+p)
			return
		}
		port = n
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	served := certs.CheckEndpoint(ctx, host, port)

	match := served.Reachable && served.Fingerprint != "" && served.Fingerprint == c.Fingerprint
	writeJSON(w, http.StatusOK, map[string]any{
		"served":       served,
		"issued_fp":    c.Fingerprint,
		"match":        match,
		"issued_until": c.NotAfter,
	})
}

// hostBelongsToCert reports whether host equals the main domain or any SAN
// (wildcards match one label).
func hostBelongsToCert(host string, c certs.Cert) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	candidates := append([]string{c.MainDomain}, c.SANs...)
	for _, d := range candidates {
		d = strings.ToLower(d)
		if d == host {
			return true
		}
		if strings.HasPrefix(d, "*.") {
			suffix := d[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) && strings.Count(host, ".") == strings.Count(d, ".") {
				return true
			}
		}
	}
	return false
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "*", "wildcard")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
