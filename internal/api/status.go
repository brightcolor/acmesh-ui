package api

import (
	"net/http"
	"time"

	"github.com/bright-color/acmesh-ui/internal/certs"
	"github.com/bright-color/acmesh-ui/internal/jobs"
)

// StatusResponse is returned by GET /api/status.
type StatusResponse struct {
	UIVersion       string   `json:"ui_version"`
	AcmeVersion     string   `json:"acme_version"`
	AcmePath        string   `json:"acme_path"`
	AcmeHome        string   `json:"acme_home"`
	AcmeFound       bool     `json:"acme_found"`
	AuthMode        string   `json:"auth_mode"`
	AuthDisabled    bool     `json:"auth_disabled"`
	OpenBind        bool     `json:"open_bind"`
	Bind            string   `json:"bind"`
	Port            int      `json:"port"`
	Title           string   `json:"title"`
	UptimeSec       int64    `json:"uptime_sec"`
	ShowAuthWarning bool     `json:"show_auth_warning"`
	Warnings        []string `json:"warnings"`
}

// Status handles GET /api/status.
func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	chk := h.Client.Check(r.Context())
	resp := StatusResponse{
		UIVersion:       h.UIVersion,
		AcmeVersion:     chk.Version,
		AcmePath:        h.Cfg.Acme.Binary,
		AcmeHome:        h.Cfg.Acme.Home,
		AcmeFound:       chk.BinaryExists && chk.Executable,
		AuthMode:        h.Cfg.Auth.Mode,
		AuthDisabled:    h.Cfg.AuthDisabled(),
		OpenBind:        h.Cfg.IsOpenBind(),
		Bind:            h.Cfg.Server.Bind,
		Port:            h.Cfg.Server.Port,
		Title:           h.Cfg.UI.Title,
		UptimeSec:       int64(time.Since(h.Started).Seconds()),
		ShowAuthWarning: h.Cfg.UI.ShowAuthWarning,
		Warnings:        h.warnings(chk.BinaryExists && chk.Executable),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) warnings(acmeOK bool) []string {
	var w []string
	if h.Cfg.AuthDisabled() {
		w = append(w, "Internal authentication is disabled (auth.mode=none). Restrict access via VPN, SSH tunnel or a reverse proxy.")
	}
	if h.Cfg.IsOpenBind() && h.Cfg.AuthDisabled() {
		w = append(w, "The server is bound to a network-reachable address without authentication.")
	}
	if !acmeOK {
		w = append(w, "acme.sh was not found or is not executable. Check acme.binary in the configuration.")
	}
	return w
}

// AcmeVersion handles GET /api/acme/version.
func (h *Handlers) AcmeVersion(w http.ResponseWriter, r *http.Request) {
	v, err := h.Client.VersionString(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "ACME_VERSION_FAILED", "Could not determine acme.sh version.", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"version": v})
}

// AcmeCheck handles GET /api/acme/check.
func (h *Handlers) AcmeCheck(w http.ResponseWriter, r *http.Request) {
	chk := h.Client.Check(r.Context())
	if !chk.BinaryExists {
		writeJSON(w, http.StatusOK, chk) // still 200; the struct carries the error
		return
	}
	writeJSON(w, http.StatusOK, chk)
}

// DashboardResponse is returned by GET /api/dashboard.
type DashboardResponse struct {
	Status       StatusResponse `json:"status"`
	Total        int            `json:"total"`
	Valid        int            `json:"valid"`
	Expiring     int            `json:"expiring"`
	Expired      int            `json:"expired"`
	Errored      int            `json:"errored"`
	FailedJobs   int            `json:"failed_jobs"`
	RecentJobs   []jobs.Job     `json:"recent_jobs"`
	Expiring30   []certs.Cert   `json:"expiring_soon"`
	ExpiringDays int            `json:"expiring_days"`
}

// Dashboard handles GET /api/dashboard.
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	list, err := h.certs(false)
	if err != nil {
		// Degrade gracefully: still show status, zero certs.
		list = nil
	}

	resp := DashboardResponse{ExpiringDays: h.Cfg.Certs.ExpiringSoonDays}
	for _, c := range list {
		switch c.Status {
		case certs.StatusValid:
			resp.Valid++
		case certs.StatusExpiring:
			resp.Expiring++
			resp.Expiring30 = append(resp.Expiring30, maskInstall(c))
		case certs.StatusExpired:
			resp.Expired++
			resp.Expiring30 = append(resp.Expiring30, maskInstall(c))
		case certs.StatusError:
			resp.Errored++
		}
	}
	resp.Total = len(list)

	chk := h.Client.Check(r.Context())
	resp.Status = StatusResponse{
		UIVersion: h.UIVersion, AcmeVersion: chk.Version, AcmePath: h.Cfg.Acme.Binary,
		AcmeHome: h.Cfg.Acme.Home, AcmeFound: chk.BinaryExists && chk.Executable,
		AuthMode: h.Cfg.Auth.Mode, AuthDisabled: h.Cfg.AuthDisabled(),
		OpenBind: h.Cfg.IsOpenBind(), Bind: h.Cfg.Server.Bind, Port: h.Cfg.Server.Port,
		Title: h.Cfg.UI.Title, UptimeSec: int64(time.Since(h.Started).Seconds()),
		ShowAuthWarning: h.Cfg.UI.ShowAuthWarning,
		Warnings:        h.warnings(chk.BinaryExists && chk.Executable),
	}

	if all, err := h.Jobs.List(); err == nil {
		for i, j := range all {
			if j.Status == jobs.StatusFailed {
				resp.FailedJobs++
			}
			if i < 8 {
				resp.RecentJobs = append(resp.RecentJobs, j)
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// Scan handles POST /api/scan (force a rescan).
func (h *Handlers) Scan(w http.ResponseWriter, r *http.Request) {
	list, err := h.certs(true)
	if err != nil {
		writeError(w, http.StatusBadGateway, "SCAN_FAILED", "Could not scan the acme.sh home directory.", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(list)})
}
