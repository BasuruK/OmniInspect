package domain

// ==========================================
// Credential Cipher Seam
// ==========================================

// CredentialCipher transforms sensitive credential strings for at-rest storage.
// Implementations live in the infrastructure layer (e.g. an AES-GCM cipher), and
// are injected at the composition root via SetCredentialCipher. Keeping only the
// interface here preserves the domain layer's independence from infrastructure.
//
// Decrypt must be backward compatible: when given a value it does not recognise as
// its own ciphertext (e.g. a legacy plaintext credential), it must return the input
// unchanged so existing persisted data keeps working.
type CredentialCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// identityCipher is the default no-op cipher used when none has been configured.
// It leaves credentials untouched, preserving the previous plaintext behaviour.
type identityCipher struct{}

func (identityCipher) Encrypt(s string) (string, error) { return s, nil }
func (identityCipher) Decrypt(s string) (string, error) { return s, nil }

// credentialCipher holds the active cipher used by domain marshaling. It defaults
// to a no-op so the domain remains usable without infrastructure wiring (e.g. tests).
var credentialCipher CredentialCipher = identityCipher{}

// SetCredentialCipher installs the cipher used to protect credentials at rest.
// Passing nil resets to the no-op identity cipher. It is intended to be called once
// during application startup from the composition root.
func SetCredentialCipher(c CredentialCipher) {
	if c == nil {
		credentialCipher = identityCipher{}
		return
	}
	credentialCipher = c
}
