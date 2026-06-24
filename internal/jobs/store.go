package jobs

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/bright-color/acmesh-ui/internal/storage"
)

// Status is a job lifecycle state.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSuccess   Status = "success"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Job is a single acme.sh action run in the background.
type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`   // issue, renew, install-cert, deploy, ...
	Domain    string    `json:"domain"` // main domain if applicable
	Status    Status    `json:"status"`
	User      string    `json:"user,omitempty"` // populated once auth exists
	QueuedAt  time.Time `json:"queued_at"`
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	ExitCode  int       `json:"exit_code"`
	// PreviewCmd is the acme.sh command line with secrets masked.
	PreviewCmd string `json:"preview_cmd"`
	// SafeArgs are the argv without secret env (env is never stored).
	SafeArgs []string `json:"safe_args"`
	// Log is the full masked output, persisted once the job ends.
	Log     []string `json:"log,omitempty"`
	Error   string   `json:"error,omitempty"`
	Summary string   `json:"summary,omitempty"`

	// Running is set at read time for live status; it is not authoritative
	// in the persisted record.
	Running bool `json:"running"`
}

// Duration returns how long the job ran (or has been running).
func (j Job) Duration() time.Duration {
	if j.StartedAt.IsZero() {
		return 0
	}
	end := j.EndedAt
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(j.StartedAt)
}

// jobStore persists jobs to bbolt.
type jobStore struct {
	db *storage.Store
}

func (s *jobStore) save(j Job) error {
	return s.db.PutJSON(storage.BucketJobs, j.ID, j)
}

func (s *jobStore) get(id string) (Job, bool, error) {
	var j Job
	ok, err := s.db.GetJSON(storage.BucketJobs, id, &j)
	return j, ok, err
}

func (s *jobStore) list() ([]Job, error) {
	var out []Job
	err := s.db.ForEach(storage.BucketJobs, func(_ string, raw []byte) error {
		var j Job
		if err := json.Unmarshal(raw, &j); err != nil {
			return err
		}
		// Don't ship full logs in list views.
		j.Log = nil
		out = append(out, j)
		return nil
	})
	sort.Slice(out, func(i, j2 int) bool { return out[i].QueuedAt.After(out[j2].QueuedAt) })
	return out, err
}

// prune deletes finished jobs older than retention.
func (s *jobStore) prune(retention time.Duration) error {
	cutoff := time.Now().Add(-retention)
	var toDelete []string
	err := s.db.ForEach(storage.BucketJobs, func(key string, raw []byte) error {
		var j Job
		if err := json.Unmarshal(raw, &j); err != nil {
			return nil
		}
		if !j.EndedAt.IsZero() && j.EndedAt.Before(cutoff) {
			toDelete = append(toDelete, key)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, k := range toDelete {
		_ = s.db.Delete(storage.BucketJobs, k)
	}
	return nil
}
