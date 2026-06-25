// Package api implements the internal JSON API consumed by the embedded UI.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/certs"
	"github.com/bright-color/acmesh-ui/internal/config"
	"github.com/bright-color/acmesh-ui/internal/dnsproviders"
	"github.com/bright-color/acmesh-ui/internal/jobs"
	"github.com/bright-color/acmesh-ui/internal/secrets"
)

// Handlers bundles the dependencies the API needs.
type Handlers struct {
	Cfg     config.Config
	Client  *acme.Client
	Scanner *acme.Scanner
	Builder acme.Builder
	Jobs    *jobs.Manager
	DNS     *dnsproviders.Store
	Masker  *secrets.Masker
	Started time.Time

	UIVersion  string
	ConfigPath string

	mu        sync.RWMutex
	certCache []certs.Cert
	scannedAt time.Time
}

// APIError is the standard error envelope.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type errEnvelope struct {
	Error APIError `json:"error"`
}

// writeJSON encodes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			log.Printf("api: encode response: %v", err)
		}
	}
}

// writeError emits the standard error envelope.
func writeError(w http.ResponseWriter, status int, code, message, details string) {
	writeJSON(w, status, errEnvelope{Error: APIError{Code: code, Message: message, Details: details}})
}

// NotFound emits a JSON 404 for unknown /api/* paths.
func NotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown API endpoint.", r.URL.Path)
}

// decode parses the JSON request body into dst.
func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// certs returns the cached certificate list, scanning if the cache is stale or
// forced.
func (h *Handlers) certs(force bool) ([]certs.Cert, error) {
	h.mu.RLock()
	fresh := !force && time.Since(h.scannedAt) < 30*time.Second && h.certCache != nil
	cached := h.certCache
	h.mu.RUnlock()
	if fresh {
		return cached, nil
	}
	list, err := h.Scanner.Scan()
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	h.certCache = list
	h.scannedAt = time.Now()
	h.mu.Unlock()
	return list, nil
}

// invalidateCerts forces the next certificate read to re-scan.
func (h *Handlers) invalidateCerts() {
	h.mu.Lock()
	h.certCache = nil
	h.scannedAt = time.Time{}
	h.mu.Unlock()
}

func (h *Handlers) findCert(id string) (certs.Cert, bool) {
	list, err := h.certs(false)
	if err != nil {
		return certs.Cert{}, false
	}
	for _, c := range list {
		if c.ID == id || c.MainDomain == id {
			return c, true
		}
	}
	return certs.Cert{}, false
}
