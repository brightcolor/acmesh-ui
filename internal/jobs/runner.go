package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// runCommand executes the acme.sh command for job, streaming masked output into
// lb and persisting the final state.
func (m *Manager) runCommand(ctx context.Context, job *Job, req Request, lb *logBuffer) {
	job.Status = StatusRunning
	job.StartedAt = time.Now()
	_ = m.store.save(*job)

	onLine := func(line string) {
		lb.Append(m.masker.Mask(line))
	}

	res, err := m.client.Run(ctx, req.Command, req.ExtraEnv, onLine)

	job.EndedAt = time.Now()
	job.Log = lb.Snapshot()
	job.ExitCode = res.ExitCode

	switch {
	case ctx.Err() == context.Canceled:
		job.Status = StatusCancelled
		job.Summary = "Job was cancelled by an operator."
	case ctx.Err() == context.DeadlineExceeded:
		job.Status = StatusFailed
		job.Error = "timeout exceeded"
		job.Summary = "acme.sh did not finish within the configured timeout."
	case err != nil:
		job.Status = StatusFailed
		job.Error = m.masker.Mask(err.Error())
		job.Summary = "Failed to execute acme.sh."
	case res.ExitCode != 0:
		job.Status = StatusFailed
		job.Summary = classifyFailure(res.ExitCode, job.Log)
	default:
		job.Status = StatusSuccess
		job.Summary = "Completed successfully."
	}

	// Always append stderr tail to log if not already streamed.
	if res.Stderr != "" && len(job.Log) == 0 {
		job.Log = append(job.Log, m.masker.Mask(res.Stderr))
	}

	// On success, optionally purge the certificate directory. The path is
	// re-validated against the acme.sh home so a bug elsewhere cannot delete an
	// arbitrary directory.
	if job.Status == StatusSuccess && req.PurgeDir != "" {
		if err := m.purgeDir(req.PurgeDir); err != nil {
			job.Log = append(job.Log, "purge: "+m.masker.Mask(err.Error()))
		} else {
			job.Log = append(job.Log, "purge: removed "+req.PurgeDir)
		}
	}

	_ = m.store.save(*job)

	// Notify the caller (e.g. to invalidate a cert cache) now that the final
	// state - including any purge - is persisted.
	if req.OnDone != nil {
		req.OnDone(*job)
	}
}

// purgeDir removes a certificate directory after re-checking that it lives
// strictly inside the acme.sh home and is not the home itself.
func (m *Manager) purgeDir(dir string) error {
	home := strings.TrimRight(m.client.Home, "/\\")
	clean := filepath.Clean(dir)
	if home == "" {
		return errors.New("acme home not configured; refusing to purge")
	}
	if clean == filepath.Clean(home) {
		return errors.New("refusing to purge the acme.sh home itself")
	}
	if !strings.HasPrefix(clean, filepath.Clean(home)+string(filepath.Separator)) {
		return errors.New("refusing to purge a directory outside the acme.sh home")
	}
	if strings.Contains(dir, "..") {
		return errors.New("refusing to purge a path containing '..'")
	}
	return os.RemoveAll(clean)
}

// classifyFailure produces an admin-friendly summary from exit code and log
// content, mapping common acme.sh failure patterns to actionable hints.
func classifyFailure(exitCode int, log []string) string {
	joined := strings.Join(log, "\n")
	for _, m := range failurePatterns {
		if strings.Contains(joined, m.needle) {
			return m.summary
		}
	}
	return "acme.sh exited with a non-zero status (" + strconv.Itoa(exitCode) + "). See the log for details."
}

type failurePattern struct {
	needle  string
	summary string
}

var failurePatterns = []failurePattern{
	{"Rate Limit", "Hit the CA rate limit. Wait, or use the staging server for testing."},
	{"rateLimited", "Hit the CA rate limit. Wait, or use the staging server for testing."},
	{"DNS problem", "DNS validation failed. Check the DNS provider credentials and that the record propagated."},
	{"Verify error", "Domain validation failed. Check the webroot/DNS challenge configuration."},
	{"Could not bind", "Standalone mode could not bind port 80 - it is likely already in use."},
	{"Address already in use", "Standalone mode could not bind port 80 - it is likely already in use."},
	{"webroot", "Webroot challenge failed. Check that the webroot path exists and is writable by acme.sh."},
	{"command not found", "A required tool was not found in PATH for the acme.sh hook."},
	{"reloadcmd", "The reload command failed after install. Check the service status."},
}
