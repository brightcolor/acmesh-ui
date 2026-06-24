package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// cipherPrefix marks values produced by this package so we can detect whether
// a stored value is already encrypted.
const cipherPrefix = "enc:v1:"

// ErrNotEncrypted is returned by Decrypt when the input is not a ciphertext
// produced by Encrypt.
var ErrNotEncrypted = errors.New("value is not an encrypted secret")

// Cipher encrypts and decrypts secret values using AES-256-GCM. The key is
// derived from the contents of the configured secret key file.
type Cipher struct {
	gcm cipher.AEAD
}

// LoadOrCreateKey reads the secret key from path, creating a fresh random 32
// byte key (0600) if the file does not yet exist. Parent directories are
// created as needed.
func LoadOrCreateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) < 16 {
			return nil, fmt.Errorf("secret key file %s is too short (need >= 16 bytes)", path)
		}
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read secret key %s: %w", path, err)
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create secret key dir: %w", err)
		}
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("write secret key %s: %w", path, err)
	}
	return key, nil
}

// NewCipher builds a Cipher from raw key material of any length (it is hashed
// to a fixed 32 byte AES key).
func NewCipher(key []byte) (*Cipher, error) {
	sum := sha256.Sum256(key)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{gcm: gcm}, nil
}

// Encrypt returns a prefixed, base64 encoded ciphertext for plaintext. Empty
// input returns empty output.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return cipherPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. It returns ErrNotEncrypted for values that lack
// the ciphertext prefix.
func (c *Cipher) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if len(ciphertext) < len(cipherPrefix) || ciphertext[:len(cipherPrefix)] != cipherPrefix {
		return "", ErrNotEncrypted
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext[len(cipherPrefix):])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	ns := c.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	nonce, body := raw[:ns], raw[ns:]
	plain, err := c.gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

// IsEncrypted reports whether v was produced by Encrypt.
func IsEncrypted(v string) bool {
	return len(v) >= len(cipherPrefix) && v[:len(cipherPrefix)] == cipherPrefix
}
