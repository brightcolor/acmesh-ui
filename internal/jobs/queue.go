package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/secrets"
	"github.com/bright-color/acmesh-ui/internal/storage"
)

// Manager runs acme.sh commands as background jobs with bounded parallelism,
// live log streaming and cancellation.
type Manager struct {
	client  *acme.Client
	masker  *secrets.Masker
	store   *jobStore
	timeout time.Duration

	sem chan struct{}

	mu      sync.Mutex
	running map[string]*runState
}

type runState struct {
	cancel context.CancelFunc
	logs   *logBuffer
}

// Request describes a job to enqueue.
type Request struct {
	Type     string
	Domain   string
	Command  acme.Command
	ExtraEnv []string // additional KEY=VALUE (e.g. decrypted DNS secrets) - not persisted
	// SecretValues are registered with the masker so they never leak to logs.
	SecretValues []string
	// PurgeDir, if set, is a directory removed after the command succeeds (used
	// by certificate deletion). It is re-validated against the acme.sh home in
	// the runner before removal.
	PurgeDir string
}

// NewManager constructs a job manager.
func NewManager(client *acme.Client, masker *secrets.Masker, db *storage.Store, maxParallel int, timeout time.Duration) *Manager {
	if maxParallel < 1 {
		maxParallel = 1
	}
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	return &Manager{
		client:  client,
		masker:  masker,
		store:   &jobStore{db: db},
		timeout: timeout,
		sem:     make(chan struct{}, maxParallel),
		running: make(map[string]*runState),
	}
}

// Submit enqueues a job and returns its persisted record (status queued).
func (m *Manager) Submit(req Request) (Job, error) {
	// Register secret values so masking covers job output everywhere.
	m.masker.Add(req.SecretValues...)

	job := Job{
		ID:         newJobID(),
		Type:       req.Type,
		Domain:     req.Domain,
		Status:     StatusQueued,
		QueuedAt:   time.Now(),
		PreviewCmd: m.masker.Mask(req.Command.PreviewArgs(m.client.Binary)),
		SafeArgs:   req.Command.Args,
	}
	if err := m.store.save(job); err != nil {
		return Job{}, err
	}
	go m.execute(job, req)
	return job, nil
}

// execute waits for a worker slot, then runs the command.
func (m *Manager) execute(job Job, req Request) {
	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	lb := newLogBuffer()
	m.mu.Lock()
	m.running[job.ID] = &runState{cancel: cancel, logs: lb}
	m.mu.Unlock()

	defer func() {
		lb.Close()
		m.mu.Lock()
		delete(m.running, job.ID)
		m.mu.Unlock()
	}()

	m.runCommand(ctx, &job, req, lb)
}

// Cancel aborts a running job. Returns false if it is not currently running.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	rs, ok := m.running[id]
	m.mu.Unlock()
	if !ok {
		return false
	}
	rs.cancel()
	return true
}

// Get returns a stored job by id (full log included).
func (m *Manager) Get(id string) (Job, bool, error) {
	return m.store.get(id)
}

// List returns all jobs (without full logs), newest first.
func (m *Manager) List() ([]Job, error) {
	return m.store.list()
}

// Subscribe returns the live log backlog and a channel of new lines for a
// running job. ok is false when the job is not currently running (caller should
// fall back to the persisted log).
func (m *Manager) Subscribe(id string) (backlog []string, ch chan string, cancel func(), ok bool) {
	m.mu.Lock()
	rs, running := m.running[id]
	m.mu.Unlock()
	if !running {
		return nil, nil, nil, false
	}
	return rs.logs.Subscribe()
}

// IsRunning reports whether a job is currently executing.
func (m *Manager) IsRunning(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.running[id]
	return ok
}

// Prune removes finished jobs older than the retention window.
func (m *Manager) Prune(retention time.Duration) error {
	return m.store.prune(retention)
}

func newJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}
