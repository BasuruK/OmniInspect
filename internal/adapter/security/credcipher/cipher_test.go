package credcipher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestCipher(t *testing.T) *Cipher {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "omniview.key")
	c, err := New(NewFileKeyProvider(keyPath))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c := newTestCipher(t)

	const secret = "super-secret-password"
	token, err := c.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !strings.HasPrefix(token, tokenPrefix) {
		t.Fatalf("token missing prefix: %q", token)
	}
	if strings.Contains(token, secret) {
		t.Fatal("ciphertext leaks plaintext")
	}

	got, err := c.Decrypt(token)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != secret {
		t.Fatalf("round-trip mismatch: got %q want %q", got, secret)
	}
}

func TestEncryptEmptyIsUnchanged(t *testing.T) {
	c := newTestCipher(t)
	token, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if token != "" {
		t.Fatalf("empty plaintext should stay empty, got %q", token)
	}
}

func TestDecryptLegacyPlaintextPassthrough(t *testing.T) {
	c := newTestCipher(t)
	const legacy = "plaintext-from-old-bolt-file"
	got, err := c.Decrypt(legacy)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != legacy {
		t.Fatalf("legacy passthrough mismatch: got %q want %q", got, legacy)
	}
}

func TestDecryptDifferentKeyFails(t *testing.T) {
	c1 := newTestCipher(t)
	c2 := newTestCipher(t) // different temp key

	token, err := c1.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := c2.Decrypt(token); err == nil {
		t.Fatal("expected decrypt with wrong key to fail")
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	c := newTestCipher(t)
	a, _ := c.Encrypt("secret")
	b, _ := c.Encrypt("secret")
	if a == b {
		t.Fatal("expected distinct ciphertexts for repeated encryption (nonce reuse)")
	}
}

func TestFileKeyProviderPersistsAndPermissions(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "omniview.key")
	p := NewFileKeyProvider(keyPath)

	k1, err := p.Key()
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	if len(k1) != keySize {
		t.Fatalf("key size = %d, want %d", len(k1), keySize)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != fileKeyPerm {
		t.Fatalf("key file perm = %o, want %o", perm, fileKeyPerm)
	}

	k2, err := p.Key()
	if err != nil {
		t.Fatalf("Key (reload): %v", err)
	}
	if string(k1) != string(k2) {
		t.Fatal("key changed across reloads; expected stable key")
	}
}
