package jobs

import "sync"

// logBuffer accumulates masked log lines for a running job and fans them out to
// live subscribers (used for Server-Sent Events).
type logBuffer struct {
	mu      sync.Mutex
	lines   []string
	subs    map[int]chan string
	nextSub int
	closed  bool
}

func newLogBuffer() *logBuffer {
	return &logBuffer{subs: make(map[int]chan string)}
}

// Append records a line and pushes it to all subscribers.
func (b *logBuffer) Append(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.lines = append(b.lines, line)
	for _, ch := range b.subs {
		select {
		case ch <- line:
		default: // drop for slow consumers; full log is persisted anyway
		}
	}
}

// Snapshot returns a copy of all lines collected so far.
func (b *logBuffer) Snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

// Subscribe returns a channel of future lines plus the backlog, and an
// unsubscribe func. The bool is false if the buffer is already closed.
func (b *logBuffer) Subscribe() (backlog []string, ch chan string, cancel func(), ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	backlog = make([]string, len(b.lines))
	copy(backlog, b.lines)
	if b.closed {
		return backlog, nil, func() {}, false
	}
	id := b.nextSub
	b.nextSub++
	c := make(chan string, 256)
	b.subs[id] = c
	cancel = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if ch, exists := b.subs[id]; exists {
			delete(b.subs, id)
			close(ch)
		}
	}
	return backlog, c, cancel, true
}

// Close stops the buffer and closes all subscriber channels.
func (b *logBuffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, ch := range b.subs {
		close(ch)
		delete(b.subs, id)
	}
}
