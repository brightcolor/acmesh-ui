package jobs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/secrets"
	"github.com/bright-color/acmesh-ui/internal/storage"
)

func newManagerWithHome(t *testing.T, home string) *Manager {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	client := acme.NewClient("/bin/false", home, acme.Builder{})
	return NewManager(client, secrets.NewMasker(), db, 1, time.Minute)
}

func TestPurgeDirSafety(t *testing.T) {
	home := t.TempDir()
	m := newManagerWithHome(t, home)

	// A directory inside the home is removed.
	inside := filepath.Join(home, "example.com")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inside, "example.com.cer"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.purgeDir(inside); err != nil {
		t.Fatalf("expected purge inside home to succeed: %v", err)
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("directory was not removed")
	}

	// The home itself must never be purged.
	if err := m.purgeDir(home); err == nil {
		t.Fatalf("expected refusal to purge the home itself")
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("home must still exist: %v", err)
	}

	// A directory outside the home must be refused.
	outside := t.TempDir()
	if err := m.purgeDir(outside); err == nil {
		t.Fatalf("expected refusal to purge outside the home")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside dir must still exist: %v", err)
	}

	// Traversal is refused.
	if err := m.purgeDir(filepath.Join(home, "..", "evil")); err == nil {
		t.Fatalf("expected refusal for '..' traversal")
	}
}

func TestPurgeDirNoHome(t *testing.T) {
	m := newManagerWithHome(t, "")
	if err := m.purgeDir("/anything"); err == nil {
		t.Fatalf("expected refusal when home is not configured")
	}
}
