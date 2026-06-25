package api

import (
	"net/http"
	"os"

	"github.com/bright-color/acmesh-ui/internal/config"
	"github.com/bright-color/acmesh-ui/internal/jobs"
)

// SettingsResponse is the read-only view of effective settings shown in the UI.
// Secrets and key material are never included.
type SettingsResponse struct {
	Server   config.Server          `json:"server"`
	Auth     config.Auth            `json:"auth"`
	Security securityView           `json:"security"`
	Acme     config.Acme            `json:"acme"`
	Jobs     config.Jobs            `json:"jobs"`
	UI       config.UI              `json:"ui"`
	Certs    config.Certs           `json:"certs"`
	Reloads  []config.ReloadCommand `json:"reload_commands"`
	DataDir  string                 `json:"data_dir"`
}

type securityView struct {
	AllowOpenWithoutAuth bool     `json:"allow_open_without_auth"`
	AllowFreeReloadCmd   bool     `json:"allow_free_reloadcmd"`
	CertBasePaths        []string `json:"cert_base_paths"`
	WebrootBasePaths     []string `json:"webroot_base_paths"`
	SecretKeyFile        string   `json:"secret_key_file"`
}

// Settings handles GET /api/settings. The configuration is read-only at runtime
// (changes require editing config.yaml and restarting), which the UI states.
func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	resp := SettingsResponse{
		Server: h.Cfg.Server,
		Auth:   h.Cfg.Auth,
		Security: securityView{
			AllowOpenWithoutAuth: h.Cfg.Security.AllowOpenWithoutAuth,
			AllowFreeReloadCmd:   h.Cfg.Security.AllowFreeReloadCmd,
			CertBasePaths:        h.Cfg.Security.CertBasePaths,
			WebrootBasePaths:     h.Cfg.Security.WebrootBasePaths,
			SecretKeyFile:        h.Cfg.Security.SecretKeyFile,
		},
		Acme:    h.Cfg.Acme,
		Jobs:    h.Cfg.Jobs,
		UI:      h.Cfg.UI,
		Certs:   h.Cfg.Certs,
		Reloads: h.Cfg.Reloads,
		DataDir: h.Cfg.Data.Dir,
	}
	writeJSON(w, http.StatusOK, resp)
}

// UpdateSettings handles PUT /api/settings. Version 1 treats the configuration
// as read-only at runtime and returns a clear, actionable error.
func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "SETTINGS_READ_ONLY",
		"Settings are read-only at runtime in this version.",
		"Edit config.yaml and restart the acmesh-ui service to apply changes.")
}

// SetDefaultCA handles POST /api/acme/default-ca. It runs
// `acme.sh --set-default-ca --server <ca>` as a job.
func (h *Handlers) SetDefaultCA(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CA string `json:"ca"`
	}
	if err := decode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.", err.Error())
		return
	}
	cmd, err := h.Builder.SetDefaultCA(body.CA)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CA", "The CA is not valid.", err.Error())
		return
	}
	job, err := h.Jobs.Submit(jobs.Request{Type: "set-default-ca", Command: cmd})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOB_SUBMIT_FAILED", "Could not start the job.", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": job.ID})
}

// SystemStatus handles GET /api/system. It surfaces filesystem and scheduler
// hints for the Systemstatus page.
type SystemStatus struct {
	AcmeBinary   string   `json:"acme_binary"`
	AcmeFound    bool     `json:"acme_found"`
	AcmeExec     bool     `json:"acme_exec"`
	AcmeVersion  string   `json:"acme_version"`
	AcmeHome     string   `json:"acme_home"`
	HomeReadable bool     `json:"home_readable"`
	DataDir      string   `json:"data_dir"`
	DataWritable bool     `json:"data_writable"`
	ConfigPath   string   `json:"config_path"`
	UIVersion    string   `json:"ui_version"`
	RenewalHints []string `json:"renewal_hints"`
}

// System handles GET /api/system.
func (h *Handlers) System(w http.ResponseWriter, r *http.Request) {
	chk := h.Client.Check(r.Context())
	st := SystemStatus{
		AcmeBinary:   h.Cfg.Acme.Binary,
		AcmeFound:    chk.BinaryExists,
		AcmeExec:     chk.Executable,
		AcmeVersion:  chk.Version,
		AcmeHome:     h.Cfg.Acme.Home,
		HomeReadable: chk.HomeReadable,
		DataDir:      h.Cfg.Data.Dir,
		DataWritable: isWritable(h.Cfg.Data.Dir),
		ConfigPath:   h.ConfigPath,
		UIVersion:    h.UIVersion,
		RenewalHints: detectRenewalMechanisms(h.Cfg.Acme.Home),
	}
	writeJSON(w, http.StatusOK, st)
}

func isWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// detectRenewalMechanisms reports any renewal automation we can detect so the
// operator is warned about multiple overlapping mechanisms. It is best-effort
// and never modifies the system.
func detectRenewalMechanisms(home string) []string {
	var hints []string
	// acme.sh installs a crontab entry; we can hint at the common locations.
	candidates := []string{
		"/etc/cron.d/acme.sh",
		"/etc/systemd/system/acme_sh.timer",
		home + "/account.conf",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			hints = append(hints, "found "+c)
		}
	}
	if len(hints) == 0 {
		hints = append(hints, "No acme.sh cron/systemd timer detected automatically. acme.sh usually installs its own crontab entry for the user it runs as - check 'crontab -l'.")
	} else {
		hints = append(hints, "acmesh-ui does not manage renewals itself; the existing acme.sh schedule remains authoritative.")
	}
	return hints
}
