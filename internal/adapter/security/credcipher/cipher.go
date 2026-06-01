// Package credcipher provides at-rest encryption for sensitive credentials using
// AES-256-GCM. It implements domain.CredentialCipher and is wired into the domain
// layer at the composition root.
//
// The 256-bit master key is obtained from a KeyProvider. The default provider
// stores the key in a 0600-permission file alongside the application's BoltDB
// database, generating it on first use. The provider is an interface so an
// OS-keyring-backed implementation can be substituted later without changing the
// cipher or the domain layer.
package credcipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// tokenPrefix marks values produced by this cipher. Values without this prefix are
// treated as legacy plaintext and returned unchanged by Decrypt, preserving backward
// compatibility with data written before encryption was enabled.
const tokenPrefix = "enc:v1:"

// keySize is the AES-256 key length in bytes.
const keySize = 32

// ErrInvalidKeySize is returned when a provided key is not 32 bytes.
var ErrInvalidKeySize = errors.New("credential key must be 32 bytes")

// ErrNilKeyProvider is returned when New is called without a key provider.
var ErrNilKeyProvider = errors.New("credcipher: nil key provider")

// ErrMalformedToken is returned when a value with the cipher prefix cannot be decoded.
var ErrMalformedToken = errors.New("malformed credential token")

// KeyProvider supplies the 256-bit symmetric key used to encrypt credentials.
type KeyProvider interface {
	// Key returns the 32-byte master key, creating and persisting it on first use.
	Key() ([]byte, error)
}

// Cipher encrypts and decrypts credential strings with AES-256-GCM.
// It implements domain.CredentialCipher.
type Cipher struct {
	aead cipher.AEAD
}

// New constructs a Cipher using the key supplied by the given provider.
func New(provider KeyProvider) (*Cipher, error) {
	if provider == nil {
		return nil, fmt.Errorf("credcipher.New: %w", ErrNilKeyProvider)
	}
	key, err := provider.Key()
	if err != nil {
		return nil, fmt.Errorf("credcipher: load key: %w", err)
	}
	if len(key) != keySize {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("credcipher: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("credcipher: new gcm: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt returns a versioned, base64-encoded ciphertext token for the given
// plaintext. An empty plaintext is returned unchanged so empty credentials are not
// stored as ciphertext.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("credcipher: generate nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return tokenPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Values that do not carry the cipher prefix are assumed
// to be legacy plaintext and are returned unchanged.
func (c *Cipher) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, tokenPrefix) {
		// Legacy plaintext (written before encryption was enabled) or empty value.
		return ciphertext, nil
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, tokenPrefix))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMalformedToken, err)
	}
	nonceSize := c.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", ErrMalformedToken
	}
	nonce, sealed := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("credcipher: decrypt: %w", err)
	}
	return string(plaintext), nil
}
