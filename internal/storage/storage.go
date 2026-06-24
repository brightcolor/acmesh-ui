// Package storage provides a small bbolt-backed key/value store for DNS
// provider configs and job history. bbolt is pure Go, so the binary needs no
// cgo and stays a single self-contained artifact.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names.
const (
	BucketDNSProviders = "dns_providers"
	BucketJobs         = "jobs"
)

// Store wraps a bbolt database.
type Store struct {
	db *bolt.DB
}

// Open opens (creating if needed) the database at path. Parent directories are
// created with 0700.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", path, err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range []string{BucketDNSProviders, BucketJobs} {
			if _, err := tx.CreateBucketIfNotExists([]byte(b)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// PutJSON stores value (JSON-encoded) under key in bucket.
func (s *Store) PutJSON(bucket, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Put([]byte(key), data)
	})
}

// GetJSON loads key from bucket into dest. Returns false if not found.
func (s *Store) GetJSON(bucket, key string, dest any) (bool, error) {
	found := false
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(bucket)).Get([]byte(key))
		if v == nil {
			return nil
		}
		found = true
		return json.Unmarshal(v, dest)
	})
	return found, err
}

// Delete removes key from bucket.
func (s *Store) Delete(bucket, key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Delete([]byte(key))
	})
}

// ForEach iterates every value in bucket, calling fn with the raw JSON bytes.
func (s *Store) ForEach(bucket string, fn func(key string, raw []byte) error) error {
	return s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).ForEach(func(k, v []byte) error {
			return fn(string(k), v)
		})
	})
}
