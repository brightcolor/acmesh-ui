package dnsproviders

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/bright-color/acmesh-ui/internal/secrets"
	"github.com/bright-color/acmesh-ui/internal/storage"
	"github.com/bright-color/acmesh-ui/internal/validate"
)

// Store persists DNS providers, encrypting secret env values.
type Store struct {
	db     *storage.Store
	cipher *secrets.Cipher
}

// NewStore wires a provider store to the backing DB and cipher.
func NewStore(db *storage.Store, cipher *secrets.Cipher) *Store {
	return &Store{db: db, cipher: cipher}
}

// Input is the API-facing payload for create/update.
type Input struct {
	Name        string            `json:"name"`
	Code        string            `json:"code"`
	Description string            `json:"description"`
	Env         map[string]string `json:"env"` // name -> plaintext value
	SecretNames []string          `json:"secret_names,omitempty"`
}

// validateInput checks the provider input.
func validateInput(in Input) error {
	if in.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if err := validate.ProviderCode(in.Code); err != nil {
		return err
	}
	for k, v := range in.Env {
		if err := validate.EnvName(k); err != nil {
			return err
		}
		if err := validate.EnvValue(v); err != nil {
			return err
		}
	}
	return nil
}

func isSecret(name string, explicit []string) bool {
	for _, s := range explicit {
		if s == name {
			return true
		}
	}
	return secrets.IsSecretEnvName(name)
}

// Create stores a new provider and returns its masked representation.
func (s *Store) Create(in Input) (Provider, error) {
	if err := validateInput(in); err != nil {
		return Provider{}, err
	}
	now := time.Now()
	p := Provider{
		ID:          newID(),
		Name:        in.Name,
		Code:        in.Code,
		Description: in.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.applyEnv(&p, in); err != nil {
		return Provider{}, err
	}
	if err := s.db.PutJSON(storage.BucketDNSProviders, p.ID, p); err != nil {
		return Provider{}, err
	}
	return Masked(p), nil
}

// Update replaces an existing provider. Secret env values left empty keep their
// previous encrypted value (so the UI can submit masked forms).
func (s *Store) Update(id string, in Input) (Provider, error) {
	existing, ok, err := s.getRaw(id)
	if err != nil {
		return Provider{}, err
	}
	if !ok {
		return Provider{}, fmt.Errorf("provider %s not found", id)
	}
	if err := validateInput(in); err != nil {
		return Provider{}, err
	}
	existing.Name = in.Name
	existing.Code = in.Code
	existing.Description = in.Description
	existing.UpdatedAt = time.Now()

	prev := map[string]EnvVar{}
	for _, e := range existing.Env {
		prev[e.Name] = e
	}
	existing.Env = nil
	for name, val := range in.Env {
		secret := isSecret(name, in.SecretNames)
		if secret && val == "" {
			if old, ok := prev[name]; ok {
				existing.Env = append(existing.Env, old)
				continue
			}
		}
		ev, err := s.encodeEnv(name, val, secret)
		if err != nil {
			return Provider{}, err
		}
		existing.Env = append(existing.Env, ev)
	}
	sortEnv(existing.Env)
	if err := s.db.PutJSON(storage.BucketDNSProviders, id, existing); err != nil {
		return Provider{}, err
	}
	return Masked(existing), nil
}

func (s *Store) applyEnv(p *Provider, in Input) error {
	for name, val := range in.Env {
		ev, err := s.encodeEnv(name, val, isSecret(name, in.SecretNames))
		if err != nil {
			return err
		}
		p.Env = append(p.Env, ev)
	}
	sortEnv(p.Env)
	return nil
}

func (s *Store) encodeEnv(name, val string, secret bool) (EnvVar, error) {
	if !secret {
		return EnvVar{Name: name, Value: val, Secret: false}, nil
	}
	enc, err := s.cipher.Encrypt(val)
	if err != nil {
		return EnvVar{}, err
	}
	return EnvVar{Name: name, Value: enc, Secret: true}, nil
}

// Delete removes a provider.
func (s *Store) Delete(id string) error {
	return s.db.Delete(storage.BucketDNSProviders, id)
}

// Get returns the masked provider.
func (s *Store) Get(id string) (Provider, bool, error) {
	p, ok, err := s.getRaw(id)
	if err != nil || !ok {
		return Provider{}, ok, err
	}
	return Masked(p), true, nil
}

// List returns all providers, masked.
func (s *Store) List() ([]Provider, error) {
	var out []Provider
	err := s.db.ForEach(storage.BucketDNSProviders, func(_ string, raw []byte) error {
		var p Provider
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		out = append(out, Masked(p))
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, err
}

// DecryptedEnv returns the provider's env as a name->plaintext map for use in
// an acme.sh invocation. This is the only path that exposes clear secrets and
// is never sent to the UI/API.
func (s *Store) DecryptedEnv(id string) (string, map[string]string, error) {
	p, ok, err := s.getRaw(id)
	if err != nil || !ok {
		return "", nil, fmt.Errorf("provider %s not found", id)
	}
	env := make(map[string]string, len(p.Env))
	for _, e := range p.Env {
		if e.Secret {
			plain, err := s.cipher.Decrypt(e.Value)
			if err != nil {
				return "", nil, fmt.Errorf("decrypt %s: %w", e.Name, err)
			}
			env[e.Name] = plain
		} else {
			env[e.Name] = e.Value
		}
	}
	return p.Code, env, nil
}

func (s *Store) getRaw(id string) (Provider, bool, error) {
	var p Provider
	ok, err := s.db.GetJSON(storage.BucketDNSProviders, id, &p)
	return p, ok, err
}

// Masked returns a copy with secret values replaced by the redaction marker.
func Masked(p Provider) Provider {
	cp := p
	cp.Env = make([]EnvVar, len(p.Env))
	for i, e := range p.Env {
		if e.Secret {
			e.Value = secrets.Redaction
		}
		cp.Env[i] = e
	}
	return cp
}

func sortEnv(env []EnvVar) {
	sort.Slice(env, func(i, j int) bool { return env[i].Name < env[j].Name })
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
