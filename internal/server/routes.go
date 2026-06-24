package server

import (
	"net/http"

	"github.com/bright-color/acmesh-ui/internal/api"
	"github.com/bright-color/acmesh-ui/internal/ui"
)

// routes builds the HTTP handler tree: JSON API under /api and the embedded UI
// for everything else.
func routes(h *api.Handlers) http.Handler {
	mux := http.NewServeMux()

	// Status & dashboard
	mux.HandleFunc("GET /api/status", h.Status)
	mux.HandleFunc("GET /api/dashboard", h.Dashboard)
	mux.HandleFunc("GET /api/system", h.System)
	mux.HandleFunc("POST /api/scan", h.Scan)
	mux.HandleFunc("GET /api/acme/version", h.AcmeVersion)
	mux.HandleFunc("GET /api/acme/check", h.AcmeCheck)

	// Certificates
	mux.HandleFunc("GET /api/certs", h.ListCerts)
	mux.HandleFunc("POST /api/certs", h.IssueCert)
	mux.HandleFunc("POST /api/certs/renew-all", h.RenewAll)
	mux.HandleFunc("GET /api/certs/{id}", h.GetCert)
	mux.HandleFunc("POST /api/certs/{id}/renew", h.RenewCert)
	mux.HandleFunc("POST /api/certs/{id}/force-renew", h.ForceRenewCert)
	mux.HandleFunc("POST /api/certs/{id}/install", h.InstallCert)
	mux.HandleFunc("POST /api/certs/{id}/deploy", h.DeployCert)

	// Jobs
	mux.HandleFunc("GET /api/jobs", h.ListJobs)
	mux.HandleFunc("GET /api/jobs/{id}", h.GetJob)
	mux.HandleFunc("GET /api/jobs/{id}/logs", h.JobLogs)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", h.CancelJob)

	// DNS providers
	mux.HandleFunc("GET /api/dns-providers", h.ListDNSProviders)
	mux.HandleFunc("POST /api/dns-providers", h.CreateDNSProvider)
	mux.HandleFunc("GET /api/dns-providers/{id}", h.GetDNSProvider)
	mux.HandleFunc("PUT /api/dns-providers/{id}", h.UpdateDNSProvider)
	mux.HandleFunc("DELETE /api/dns-providers/{id}", h.DeleteDNSProvider)

	// Settings
	mux.HandleFunc("GET /api/settings", h.Settings)
	mux.HandleFunc("PUT /api/settings", h.UpdateSettings)

	// Unknown API path -> JSON 404 (avoid serving the SPA shell for /api/*).
	mux.HandleFunc("/api/", api.NotFound)

	// Embedded UI (catch-all).
	mux.Handle("/", ui.Handler())

	return mux
}
