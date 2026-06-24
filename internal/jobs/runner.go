package jobs

import (
	"context"
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

	_ = m.store.save(*job)
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
