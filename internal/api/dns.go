package api

import (
	"net/http"

	"github.com/bright-color/acmesh-ui/internal/dnsproviders"
)

// ListDNSProviders handles GET /api/dns-providers. It returns stored providers
// (secrets masked) and the catalogue of known provider codes.
func (h *Handlers) ListDNSProviders(w http.ResponseWriter, r *http.Request) {
	list, err := h.DNS.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DNS_LIST_FAILED", "Could not list DNS providers.", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": list,
		"known":     dnsproviders.KnownProviders,
	})
}

// GetDNSProvider handles GET /api/dns-providers/{id}.
func (h *Handlers) GetDNSProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok, err := h.DNS.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DNS_GET_FAILED", "Could not load provider.", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "DNS_NOT_FOUND", "DNS provider not found.", "id: "+id)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"provider": p})
}

// CreateDNSProvider handles POST /api/dns-providers.
func (h *Handlers) CreateDNSProvider(w http.ResponseWriter, r *http.Request) {
	var in dnsproviders.Input
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.", err.Error())
		return
	}
	p, err := h.DNS.Create(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, "DNS_CREATE_FAILED", "Could not create DNS provider.", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"provider": p})
}

// UpdateDNSProvider handles PUT /api/dns-providers/{id}.
func (h *Handlers) UpdateDNSProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in dnsproviders.Input
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.", err.Error())
		return
	}
	p, err := h.DNS.Update(id, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, "DNS_UPDATE_FAILED", "Could not update DNS provider.", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"provider": p})
}

// DeleteDNSProvider handles DELETE /api/dns-providers/{id}.
func (h *Handlers) DeleteDNSProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.DNS.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "DNS_DELETE_FAILED", "Could not delete DNS provider.", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
