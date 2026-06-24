package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bright-color/acmesh-ui/internal/updater"
)

// restartFallback exits non-zero so a service manager (systemd Restart=on-failure)
// brings the process back up with the new binary when in-process re-exec fails.
func restartFallback() {
	os.Exit(1)
}

// update check results are cached briefly to avoid hammering the GitHub API.
var (
	updMu     sync.Mutex
	updCache  updater.CheckResult
	updCached time.Time
)

// UpdateCheck handles GET /api/update/check.
func (h *Handlers) UpdateCheck(w http.ResponseWriter, r *http.Request) {
	updMu.Lock()
	if time.Since(updCached) < 5*time.Minute && updCache.Latest != "" {
		res := updCache
		updMu.Unlock()
		writeJSON(w, http.StatusOK, res)
		return
	}
	updMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := updater.Check(ctx, h.UIVersion)
	if err != nil {
		writeError(w, http.StatusBadGateway, "UPDATE_CHECK_FAILED",
			"Could not check for updates.", err.Error())
		return
	}
	res.RestartSupported = updater.RestartSupported()

	updMu.Lock()
	updCache = res
	updCached = time.Now()
	updMu.Unlock()

	writeJSON(w, http.StatusOK, res)
}

// UpdateApply handles POST /api/update/apply. On success it replaces the binary
// and schedules a process restart (re-exec) so the new version takes over. The
// response is sent BEFORE the restart so the UI can begin polling.
func (h *Handlers) UpdateApply(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	tag, err := updater.Apply(ctx, "")
	if err != nil {
		writeError(w, http.StatusBadGateway, "UPDATE_FAILED", "The update could not be installed.", err.Error())
		return
	}

	restart := updater.RestartSupported()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"version":    tag,
		"restarting": restart,
	})

	if !restart {
		log.Printf("update: installed %s; manual restart required", tag)
		return
	}

	// Flush, then restart shortly after so the HTTP response reaches the client.
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		log.Printf("update: installed %s, restarting...", tag)
		time.Sleep(800 * time.Millisecond)
		if err := updater.Restart(); err != nil {
			log.Printf("update: restart failed: %v (exiting so the service manager restarts us)", err)
			// As a fallback, exit non-zero so systemd Restart=on-failure kicks in.
			restartFallback()
		}
	}()
}
