package jobs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/secrets"
	"github.com/bright-color/acmesh-ui/internal/storage"
)

// writeFakeAcme creates a fake acme.sh shell script that echoes its args and a
// secret, then exits with the given code. Skipped on Windows.
func writeFakeAcme(t *testing.T, exitCode int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake acme.sh integration uses a shell script; skipped on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "acme.sh")
	script := "#!/bin/sh\necho \"running with args: $@\"\necho \"token is $CF_Token\"\nexit " + itoaTest(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	return string(rune('0' + n))
}

func newTestManager(t *testing.T, acmePath string) *Manager {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	masker := secrets.NewMasker()
	client := acme.NewClient(acmePath, "", acme.Builder{})
	return NewManager(client, masker, db, 2, 5*time.Second)
}

func waitForJob(t *testing.T, m *Manager, id string, want Status) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, ok, err := m.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if ok && j.Status == want {
			return j
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach status %s", id, want)
	return Job{}
}

func TestJobSuccessAndMasking(t *testing.T) {
	acmePath := writeFakeAcme(t, 0)
	m := newTestManager(t, acmePath)

	cmd := acme.Command{Action: "issue", Args: []string{"--issue", "-d", "example.com", "--dns", "dns_cf"}}
	job, err := m.Submit(Request{
		Type:         "issue",
		Domain:       "example.com",
		Command:      cmd,
		ExtraEnv:     []string{"CF_Token=topsecretvalue"},
		SecretValues: []string{"topsecretvalue"},
	})
	if err != nil {
		t.Fatal(err)
	}
	done := waitForJob(t, m, job.ID, StatusSuccess)
	joined := ""
	for _, l := range done.Log {
		joined += l + "\n"
	}
	if contains := indexOfStr(joined, "topsecretvalue"); contains >= 0 {
		t.Fatalf("secret leaked into job log: %q", joined)
	}
	if indexOfStr(joined, secrets.Redaction) < 0 {
		t.Fatalf("expected redaction marker in log: %q", joined)
	}
}

func TestJobFailureClassification(t *testing.T) {
	acmePath := writeFakeAcme(t, 1)
	m := newTestManager(t, acmePath)
	cmd := acme.Command{Action: "renew", Args: []string{"--renew", "-d", "example.com"}}
	job, err := m.Submit(Request{Type: "renew", Domain: "example.com", Command: cmd})
	if err != nil {
		t.Fatal(err)
	}
	done := waitForJob(t, m, job.ID, StatusFailed)
	if done.ExitCode != 1 {
		t.Fatalf("exit code: got %d", done.ExitCode)
	}
	if done.Summary == "" {
		t.Fatalf("expected a failure summary")
	}
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
