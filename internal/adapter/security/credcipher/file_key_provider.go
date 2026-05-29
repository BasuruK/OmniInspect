package credcipher

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// fileKeyPerm is the restrictive permission applied to the key file (owner rw only).
const fileKeyPerm os.FileMode = 0o600

// FileKeyProvider stores the master key in a file with 0600 permissions, generating
// a cryptographically random key on first use. The key file should live alongside
// the encrypted database so the two are managed together.
type FileKeyProvider struct {
	path string
}

// NewFileKeyProvider returns a provider backed by the given key-file path.
func NewFileKeyProvider(path string) *FileKeyProvider {
	return &FileKeyProvider{path: path}
}

// Key returns the master key, creating and persisting it on first use.
func (p *FileKeyProvider) Key() ([]byte, error) {
	if p == nil || strings.TrimSpace(p.path) == "" {
		return nil, errors.New("credcipher: empty key path")
	}

	data, err := os.ReadFile(p.path)
	if err == nil {
		key, decodeErr := hex.DecodeString(strings.TrimSpace(string(data)))
		if decodeErr != nil {
			return nil, fmt.Errorf("credcipher: decode key file %s: %w", p.path, decodeErr)
		}
		if len(key) != keySize {
			return nil, fmt.Errorf("credcipher: key file %s: %w", p.path, ErrInvalidKeySize)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("credcipher: read key file %s: %w", p.path, err)
	}

	return p.generate()
}

// generate creates a new random key and writes it atomically with 0600 permissions.
func (p *FileKeyProvider) generate() ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("credcipher: generate key: %w", err)
	}

	if dir := filepath.Dir(p.path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("credcipher: create key dir: %w", err)
		}
	}

	encoded := []byte(hex.EncodeToString(key))
	tmp := p.path + ".tmp"
	if err := os.WriteFile(tmp, encoded, fileKeyPerm); err != nil {
		return nil, fmt.Errorf("credcipher: write key file: %w", err)
	}
	if err := os.Rename(tmp, p.path); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("credcipher: persist key file: %w", err)
	}
	// Enforce permissions in case the file pre-existed with a looser mode.
	if err := os.Chmod(p.path, fileKeyPerm); err != nil {
		return nil, fmt.Errorf("credcipher: chmod key file: %w", err)
	}
	return key, nil
}
