package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/certs"
	"github.com/bright-color/acmesh-ui/internal/jobs"
	"github.com/bright-color/acmesh-ui/internal/validate"
)

// maskInstall returns a copy of the cert with any reload command in the install
// config masked (it may embed secrets).
func maskInstall(c certs.Cert) certs.Cert {
	if c.Install != nil && c.Install.ReloadCmd != "" {
		cp := *c.Install
		cp.ReloadCmd = c.Install.ReloadCmd // reload commands are templates; keep visible
		c.Install = &cp
	}
	return c
}

// ListCerts handles GET /api/certs.
func (h *Handlers) ListCerts(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("refresh") == "1"
	list, err := h.certs(force)
	if err != nil {
		writeError(w, http.StatusBadGateway, "SCAN_FAILED",
			"Could not read the acme.sh home directory.",
			"Configured home: "+h.Cfg.Acme.Home+" ("+err.Error()+")")
		return
	}
	out := make([]certs.Cert, 0, len(list))
	for _, c := range list {
		out = append(out, maskInstall(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"certs": out})
}

// GetCert handles GET /api/certs/{id}.
func (h *Handlers) GetCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := h.findCert(id)
	if !ok {
		writeError(w, http.StatusNotFound, "CERT_NOT_FOUND", "Certificate not found.", "id: "+id)
		return
	}
	// Attach recent jobs for this domain.
	var related []jobs.Job
	if all, err := h.Jobs.List(); err == nil {
		for _, j := range all {
			if j.Domain == c.MainDomain {
				related = append(related, j)
			}
			if len(related) >= 10 {
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"cert": maskInstall(c), "jobs": related})
}

// IssueRequest is the POST /api/certs payload.
type IssueRequest struct {
	Domains       []string `json:"domains"`
	Challenge     string   `json:"challenge"` // webroot|standalone|dns|dns-manual
	Webroot       string   `json:"webroot"`
	DNSProviderID string   `json:"dns_provider_id"`
	DNSCode       string   `json:"dns_code"`  // used with UseSaved
	UseSaved      bool     `json:"use_saved"` // use acme.sh-saved credentials (no env)
	KeyType       string   `json:"key_type"`
	CA            string   `json:"ca"`
	Staging       bool     `json:"staging"`
	Force         bool     `json:"force"`
	Preview       bool     `json:"preview"` // if true, only return the command preview
}

// IssueCert handles POST /api/certs.
func (h *Handlers) IssueCert(w http.ResponseWriter, r *http.Request) {
	var req IssueRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.", err.Error())
		return
	}

	spec := acme.IssueSpec{
		Domains:   req.Domains,
		Challenge: acme.Challenge(req.Challenge),
		Webroot:   req.Webroot,
		KeyType:   req.KeyType,
		CA:        req.CA,
		Staging:   req.Staging,
		Force:     req.Force,
	}

	var secretValues []string
	if spec.Challenge == acme.ChallengeDNS {
		switch {
		case req.UseSaved:
			// Rely on credentials already stored in acme.sh's account.conf:
			// pass the provider code but no env, so acme.sh uses SAVED_*.
			if err := validate.ProviderCode(req.DNSCode); err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_DNS_CODE",
					"A valid DNS provider code is required to use saved credentials.", err.Error())
				return
			}
			spec.DNSCode = req.DNSCode
		case req.DNSProviderID != "":
			code, env, err := h.DNS.DecryptedEnv(req.DNSProviderID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "DNS_PROVIDER_NOT_FOUND",
					"The selected DNS provider could not be loaded.", err.Error())
				return
			}
			spec.DNSCode = code
			spec.DNSEnv = env
			for _, v := range env {
				secretValues = append(secretValues, v)
			}
		default:
			writeError(w, http.StatusBadRequest, "DNS_PROVIDER_REQUIRED",
				"A DNS provider must be selected for the DNS-01 challenge.", "")
			return
		}
	}

	cmd, err := h.Builder.Issue(spec)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ISSUE_SPEC", "The certificate request is invalid.", err.Error())
		return
	}

	preview := h.Masker.Mask(cmd.PreviewArgs(h.Client.Binary))
	if req.Preview {
		writeJSON(w, http.StatusOK, map[string]any{"preview": preview, "action": cmd.Action})
		return
	}

	// cmd.Env already carries the DNS provider variables; secretValues are
	// registered with the masker so they never appear in logs.
	job, err := h.Jobs.Submit(jobs.Request{
		Type:         "issue",
		Domain:       firstDomain(req.Domains),
		Command:      cmd,
		SecretValues: secretValues,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOB_SUBMIT_FAILED", "Could not start the issuance job.", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": job.ID, "preview": preview})
}

// RenewCert handles POST /api/certs/{id}/renew.
func (h *Handlers) RenewCert(w http.ResponseWriter, r *http.Request) {
	h.renew(w, r, false)
}

// ForceRenewCert handles POST /api/certs/{id}/force-renew.
func (h *Handlers) ForceRenewCert(w http.ResponseWriter, r *http.Request) {
	h.renew(w, r, true)
}

func (h *Handlers) renew(w http.ResponseWriter, r *http.Request, force bool) {
	id := r.PathValue("id")
	c, ok := h.findCert(id)
	if !ok {
		writeError(w, http.StatusNotFound, "CERT_NOT_FOUND", "Certificate not found.", "id: "+id)
		return
	}
	cmd, err := h.Builder.Renew(c.MainDomain, force)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_RENEW", "Could not build the renew command.", err.Error())
		return
	}
	jobType := "renew"
	if force {
		jobType = "force-renew"
	}
	job, err := h.Jobs.Submit(jobs.Request{Type: jobType, Domain: c.MainDomain, Command: cmd})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOB_SUBMIT_FAILED", "Could not start the renew job.", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": job.ID})
}

// DeleteCert handles DELETE /api/certs/{id}. It runs `acme.sh --remove` (which
// stops renewal) and, when purge=1, also removes the on-disk domain directory.
func (h *Handlers) DeleteCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := h.findCert(id)
	if !ok {
		writeError(w, http.StatusNotFound, "CERT_NOT_FOUND", "Certificate not found.", "id: "+id)
		return
	}
	purge := r.URL.Query().Get("purge") == "1" || r.URL.Query().Get("purge") == "true"

	cmd, err := h.Builder.Remove(c.MainDomain, c.Ecc)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REMOVE", "Could not build the remove command.", err.Error())
		return
	}
	req := jobs.Request{Type: "remove", Domain: c.MainDomain, Command: cmd}

	if purge {
		if !dirWithin(c.DomainDir, h.Cfg.Acme.Home) {
			writeError(w, http.StatusBadRequest, "INVALID_PURGE",
				"The certificate directory is not inside the acme.sh home; refusing to delete files.",
				"dir: "+c.DomainDir)
			return
		}
		req.PurgeDir = c.DomainDir
	}

	job, err := h.Jobs.Submit(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOB_SUBMIT_FAILED", "Could not start the remove job.", err.Error())
		return
	}
	h.invalidateCerts()
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": job.ID, "purged": purge})
}

// dirWithin reports whether dir is strictly inside base (and not base itself).
func dirWithin(dir, base string) bool {
	if dir == "" || base == "" {
		return false
	}
	d := filepath.Clean(dir)
	b := filepath.Clean(base)
	if d == b || strings.Contains(dir, "..") {
		return false
	}
	return strings.HasPrefix(d, b+string(filepath.Separator))
}

// RenewAll handles POST /api/certs/renew-all.
func (h *Handlers) RenewAll(w http.ResponseWriter, r *http.Request) {
	cmd := h.Builder.RenewAll()
	job, err := h.Jobs.Submit(jobs.Request{Type: "renew-all", Command: cmd})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOB_SUBMIT_FAILED", "Could not start renew-all.", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": job.ID})
}

func firstDomain(d []string) string {
	if len(d) > 0 {
		return d[0]
	}
	return ""
}
