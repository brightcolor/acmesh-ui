// Package secrets provides secret encryption at rest and a central masking
// component. Masking is applied to UI output, API responses, job logs and
// stdout/stderr so that DNS tokens, API keys and passwords never leak.
package secrets

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Redaction is the placeholder written in place of a secret.
const Redaction = "********"

// envSecretKeyPattern matches environment variable NAMES that conventionally
// hold secrets (used when masking "KEY=value" style lines).
var envSecretKeyPattern = regexp.MustCompile(`(?i)(token|secret|key|pass|pwd|password|api|auth|credential)`)

// kvLinePattern captures `NAME=value` assignments (acme.sh env, account.conf).
var kvLinePattern = regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*['"]?([^'"\r\n]*)['"]?\s*$`)

// headerPattern captures HTTP Authorization / Bearer style secrets.
var headerPattern = regexp.MustCompile(`(?i)(authorization|bearer|x-auth-[a-z-]*|x-api-[a-z-]*)\s*[:=]\s*\S+`)

// Masker holds a set of literal secret values that must always be redacted,
// in addition to heuristic pattern based redaction. It is safe for concurrent
// use.
type Masker struct {
	mu       sync.RWMutex
	literals map[string]struct{}
}

// NewMasker returns an empty masker.
func NewMasker() *Masker {
	return &Masker{literals: make(map[string]struct{})}
}

// Add registers one or more literal secret values to always redact. Empty and
// very short values (< 4 chars) are ignored to avoid masking unrelated text.
func (m *Masker) Add(values ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range values {
		v = strings.TrimSpace(v)
		if len(v) < 4 {
			continue
		}
		m.literals[v] = struct{}{}
	}
}

// Mask returns s with every registered literal secret and every heuristically
// detected secret replaced by the redaction placeholder.
func (m *Masker) Mask(s string) string {
	if s == "" {
		return s
	}

	// 1. Redact registered literals (longest first to avoid partial overlaps).
	m.mu.RLock()
	lits := make([]string, 0, len(m.literals))
	for l := range m.literals {
		lits = append(lits, l)
	}
	m.mu.RUnlock()
	sort.Slice(lits, func(i, j int) bool { return len(lits[i]) > len(lits[j]) })
	for _, l := range lits {
		s = strings.ReplaceAll(s, l, Redaction)
	}

	// 2. Redact Authorization / Bearer headers.
	s = headerPattern.ReplaceAllStringFunc(s, func(match string) string {
		idx := strings.IndexAny(match, ":=")
		if idx < 0 {
			return Redaction
		}
		return match[:idx+1] + " " + Redaction
	})

	// 3. Redact NAME=value lines where NAME looks like a secret.
	s = kvLinePattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := kvLinePattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		name, val := sub[1], sub[2]
		if val == "" || !envSecretKeyPattern.MatchString(name) {
			return match
		}
		return name + "=" + Redaction
	})

	return s
}

// MaskValue redacts a single secret value entirely. Used for API responses
// where the field is known to be a secret.
func MaskValue(s string) string {
	if s == "" {
		return ""
	}
	return Redaction
}

// MaskValueHint redacts a value but keeps the first and last 2 characters
// visible (only for non-trivial lengths). Used in the UI when the operator
// opts in to a partial hint.
func MaskValueHint(s string) string {
	if len(s) <= 6 {
		return MaskValue(s)
	}
	return s[:2] + Redaction + s[len(s)-2:]
}

// IsSecretEnvName reports whether an environment variable name conventionally
// holds a secret. DNS provider env handling uses this to decide which fields
// to encrypt and mask.
func IsSecretEnvName(name string) bool {
	return envSecretKeyPattern.MatchString(name)
}
