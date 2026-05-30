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

	key, err := p.readKeyFile()
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return p.generate()
}

func (p *FileKeyProvider) readKeyFile() ([]byte, error) {
	fileInfo, err := os.Stat(p.path)
	if err != nil {
		return nil, fmt.Errorf("credcipher: stat key file %s: %w", p.path, err)
	}
	if fileInfo.Mode().Perm() != fileKeyPerm {
		if err := os.Chmod(p.path, fileKeyPerm); err != nil {
			return nil, fmt.Errorf("credcipher: chmod key file %s: %w", p.path, err)
		}
	}

	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("credcipher: read key file %s: %w", p.path, err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("credcipher: decode key file %s: %w", p.path, err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("credcipher: key file %s: %w", p.path, ErrInvalidKeySize)
	}
	return key, nil
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
	tmpFile, err := os.CreateTemp(filepath.Dir(p.path), "."+filepath.Base(p.path)+".*.tmp")
	if err != nil {
		return nil, fmt.Errorf("credcipher: create temp key file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if err := tmpFile.Chmod(fileKeyPerm); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("credcipher: chmod temp key file: %w", err)
	}
	if _, err := tmpFile.Write(encoded); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("credcipher: write key file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("credcipher: sync temp key file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("credcipher: close temp key file: %w", err)
	}
	if err := os.Link(tmpPath, p.path); err != nil {
		if errors.Is(err, os.ErrExist) {
			existingKey, readErr := p.readKeyFile()
			if readErr != nil {
				return nil, readErr
			}
			if string(existingKey) != string(key) {
				return nil, fmt.Errorf("credcipher: key file %s already exists with different contents", p.path)
			}
			return existingKey, nil
		}
		return nil, fmt.Errorf("credcipher: persist key file %s: %w", p.path, err)
	}
	if err := os.Chmod(p.path, fileKeyPerm); err != nil {
		return nil, fmt.Errorf("credcipher: chmod key file: %w", err)
	}
	return key, nil
}
