package api

import (
	"net/http"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/jobs"
)

// InstallRequest is the POST /api/certs/{id}/install payload.
type InstallRequest struct {
	KeyFile       string `json:"key_file"`
	FullchainFile string `json:"fullchain_file"`
	CertFile      string `json:"cert_file"`
	CAFile        string `json:"ca_file"`
	ReloadName    string `json:"reload_name"` // must match a configured reload template
	Preview       bool   `json:"preview"`
}

// InstallCert handles POST /api/certs/{id}/install.
func (h *Handlers) InstallCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := h.findCert(id)
	if !ok {
		writeError(w, http.StatusNotFound, "CERT_NOT_FOUND", "Certificate not found.", "id: "+id)
		return
	}
	var req InstallRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.", err.Error())
		return
	}

	var reloadCmd []string
	reloadAllowed := false
	if req.ReloadName != "" {
		tmpl, found := h.reloadTemplate(req.ReloadName)
		if !found {
			writeError(w, http.StatusBadRequest, "RELOAD_NOT_ALLOWED",
				"The selected reload command is not in the allowed template list.",
				"name: "+req.ReloadName)
			return
		}
		reloadCmd = tmpl
		reloadAllowed = true
	}

	spec := acme.InstallSpec{
		Domain:        c.MainDomain,
		KeyFile:       req.KeyFile,
		FullchainFile: req.FullchainFile,
		CertFile:      req.CertFile,
		CAFile:        req.CAFile,
		ReloadCmd:     reloadCmd,
	}
	cmd, err := h.Builder.InstallCert(spec, h.Cfg.Security.CertBasePaths, reloadAllowed)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INSTALL", "The install request is invalid.", err.Error())
		return
	}

	preview := h.Masker.Mask(cmd.PreviewArgs(h.Client.Binary))
	if req.Preview {
		writeJSON(w, http.StatusOK, map[string]any{"preview": preview})
		return
	}
	job, err := h.Jobs.Submit(jobs.Request{Type: "install-cert", Domain: c.MainDomain, Command: cmd})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOB_SUBMIT_FAILED", "Could not start the install job.", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": job.ID, "preview": preview})
}

// reloadTemplate returns the argv for a configured reload template by name.
func (h *Handlers) reloadTemplate(name string) ([]string, bool) {
	for _, rc := range h.Cfg.Reloads {
		if rc.Name == name {
			out := make([]string, len(rc.Command))
			copy(out, rc.Command)
			return out, true
		}
	}
	return nil, false
}

// DeployRequest is the POST /api/certs/{id}/deploy payload.
type DeployRequest struct {
	Hook          string `json:"hook"`
	DNSProviderID string `json:"dns_provider_id"` // optional, supplies env for the hook
	Preview       bool   `json:"preview"`
}

// DeployCert handles POST /api/certs/{id}/deploy.
func (h *Handlers) DeployCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := h.findCert(id)
	if !ok {
		writeError(w, http.StatusNotFound, "CERT_NOT_FOUND", "Certificate not found.", "id: "+id)
		return
	}
	var req DeployRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.", err.Error())
		return
	}

	var env map[string]string
	var secretValues []string
	if req.DNSProviderID != "" {
		_, e, err := h.DNS.DecryptedEnv(req.DNSProviderID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "DNS_PROVIDER_NOT_FOUND", "Deploy env provider not found.", err.Error())
			return
		}
		env = e
		for _, v := range e {
			secretValues = append(secretValues, v)
		}
	}

	cmd, err := h.Builder.Deploy(c.MainDomain, req.Hook, env)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DEPLOY", "The deploy request is invalid.", err.Error())
		return
	}
	preview := h.Masker.Mask(cmd.PreviewArgs(h.Client.Binary))
	if req.Preview {
		writeJSON(w, http.StatusOK, map[string]any{"preview": preview})
		return
	}
	job, err := h.Jobs.Submit(jobs.Request{Type: "deploy", Domain: c.MainDomain, Command: cmd, SecretValues: secretValues})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOB_SUBMIT_FAILED", "Could not start the deploy job.", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": job.ID, "preview": preview})
}
