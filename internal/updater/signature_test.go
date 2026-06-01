package updater

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestVerifyChecksumFileSignatureDisabledWhenNoKey(t *testing.T) {
	defer func(prev ed25519.PublicKey) { signaturePublicKey = prev }(signaturePublicKey)
	signaturePublicKey = nil

	verified, err := verifyChecksumFileSignature(&githubRelease{}, "checksums.txt", "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verified {
		t.Fatal("expected verification to be skipped when no key configured")
	}
}

func TestVerifyChecksumFileSignatureRequiresAssetWhenKeySet(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	defer func(prev ed25519.PublicKey) { signaturePublicKey = prev }(signaturePublicKey)
	signaturePublicKey = pub

	// Release has no .sig asset -> fail-closed.
	_, err = verifyChecksumFileSignature(&githubRelease{}, "checksums.txt", "data")
	if err == nil {
		t.Fatal("expected error when signature asset is missing")
	}
}

func TestDecodeSignatureFormats(t *testing.T) {
	sig := make([]byte, ed25519.SignatureSize)
	for i := range sig {
		sig[i] = byte(i)
	}

	cases := map[string][]byte{
		"raw":    sig,
		"base64": []byte(base64.StdEncoding.EncodeToString(sig)),
		"hex":    []byte(hex.EncodeToString(sig)),
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decodeSignature(payload)
			if err != nil {
				t.Fatalf("decodeSignature: %v", err)
			}
			if string(got) != string(sig) {
				t.Fatal("decoded signature mismatch")
			}
		})
	}
}

func TestDecodeSignatureRejectsWrongSize(t *testing.T) {
	if _, err := decodeSignature([]byte("not-a-signature")); err == nil {
		t.Fatal("expected error for malformed signature")
	}
}

func TestEd25519VerifyValidAndInvalid(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	msg := []byte("checksum-file-contents")
	sig := ed25519.Sign(priv, msg)

	if !ed25519.Verify(pub, msg, sig) {
		t.Fatal("valid signature failed to verify")
	}
	if ed25519.Verify(pub, []byte("tampered"), sig) {
		t.Fatal("tampered message verified")
	}
}
