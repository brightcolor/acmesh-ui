package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bright-color/acmesh-ui/internal/jobs"
)

// ListJobs handles GET /api/jobs with optional status/domain filters.
func (h *Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	all, err := h.Jobs.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOBS_LIST_FAILED", "Could not list jobs.", err.Error())
		return
	}
	statusFilter := r.URL.Query().Get("status")
	domainFilter := r.URL.Query().Get("domain")
	out := make([]jobs.Job, 0, len(all))
	for _, j := range all {
		if statusFilter != "" && string(j.Status) != statusFilter {
			continue
		}
		if domainFilter != "" && !strings.Contains(j.Domain, domainFilter) {
			continue
		}
		out = append(out, j)
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
}

// GetJob handles GET /api/jobs/{id}.
func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j, ok, err := h.Jobs.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JOB_GET_FAILED", "Could not load job.", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found.", "id: "+id)
		return
	}
	j.Running = h.Jobs.IsRunning(id)
	writeJSON(w, http.StatusOK, map[string]any{"job": j})
}

// CancelJob handles POST /api/jobs/{id}/cancel.
func (h *Handlers) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.Jobs.Cancel(id) {
		writeError(w, http.StatusConflict, "JOB_NOT_RUNNING", "The job is not currently running.", "id: "+id)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
}

// JobLogs handles GET /api/jobs/{id}/logs.
//
// If the job is running it streams live lines via Server-Sent Events; otherwise
// it returns the persisted log as JSON.
func (h *Handlers) JobLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	backlog, ch, cancel, ok := h.Jobs.Subscribe(id)
	if !ok {
		// Not running: return the stored log.
		j, found, err := h.Jobs.Get(id)
		if err != nil || !found {
			writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found.", "id: "+id)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"lines": j.Log, "running": false, "status": j.Status})
		return
	}
	defer cancel()

	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		// No streaming support: drain and return what we have.
		writeJSON(w, http.StatusOK, map[string]any{"lines": backlog, "running": true})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for _, line := range backlog {
		writeSSE(w, "log", line)
	}
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case line, more := <-ch:
			if !more {
				writeSSE(w, "end", "")
				flusher.Flush()
				return
			}
			writeSSE(w, "log", line)
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event, data string) {
	// Multi-line data must be split into multiple data: fields.
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
