package updater

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// releasePublicKeyB64 is the base64-encoded ed25519 public key (32 bytes) used to
// authenticate release artifacts. When empty, signature verification is skipped so
// that releases published before signing keys were provisioned continue to update.
//
// Once this constant is populated with the project's release signing public key,
// verification becomes fail-closed: an update is rejected unless the release ships a
// valid detached signature of its checksum file.
//
// To provision: generate an ed25519 keypair, keep the private key in the release
// signing secret store, base64-encode the public key here, and have CI publish a
// "<checksum-asset>.sig" asset containing the detached signature of the checksum
// file's bytes for every release.
const releasePublicKeyB64 = ""

// signaturePublicKey holds the decoded signing public key. Tests may override it.
var signaturePublicKey = mustDecodePublicKey(releasePublicKeyB64)

// signatureAssetSuffixes are the asset name suffixes searched for a detached
// signature of the checksum file.
var signatureAssetSuffixes = []string{".sig", ".ed25519"}

// errSignatureRequired indicates a signing key is configured but the release does
// not provide a signature asset.
var errSignatureRequired = errors.New("release signature asset not found")

// mustDecodePublicKey decodes a base64 ed25519 public key. It returns nil for an
// empty string (verification disabled) and panics on a malformed non-empty key,
// since that is a build-time configuration error.
func mustDecodePublicKey(b64 string) ed25519.PublicKey {
	b64 = strings.TrimSpace(b64)
	if b64 == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		panic(fmt.Sprintf("updater: invalid release public key: %v", err))
	}
	if len(raw) != ed25519.PublicKeySize {
		panic(fmt.Sprintf("updater: release public key must be %d bytes, got %d", ed25519.PublicKeySize, len(raw)))
	}
	return ed25519.PublicKey(raw)
}

// verifyChecksumFileSignature authenticates the checksum file's contents using a
// detached ed25519 signature published in the release.
//
// Returns (false, nil) when signature verification is disabled (no public key
// configured). Returns (true, nil) when the signature is valid. Returns (false, err)
// when a key is configured but the signature is missing, malformed, or invalid.
func verifyChecksumFileSignature(release *githubRelease, checksumAssetName, checksumData string) (bool, error) {
	if len(signaturePublicKey) == 0 {
		// Signing not yet provisioned; integrity is still covered by the checksum.
		return false, nil
	}

	sigURL := findSignatureAssetURL(release, checksumAssetName)
	if sigURL == "" {
		return false, fmt.Errorf("%w for %s", errSignatureRequired, checksumAssetName)
	}

	sigBytes, err := downloadSignature(sigURL)
	if err != nil {
		return false, fmt.Errorf("failed to download signature: %w", err)
	}

	if !ed25519.Verify(signaturePublicKey, []byte(checksumData), sigBytes) {
		return false, errors.New("release signature is invalid")
	}
	return true, nil
}

// findSignatureAssetURL locates the detached signature asset for the given checksum
// asset name, trying each known signature suffix.
func findSignatureAssetURL(release *githubRelease, checksumAssetName string) string {
	for _, suffix := range signatureAssetSuffixes {
		want := checksumAssetName + suffix
		for _, asset := range release.Assets {
			if asset.Name == want {
				return asset.BrowserDownloadURL
			}
		}
	}
	return ""
}

// downloadSignature fetches and decodes a detached signature. The signature may be
// stored raw (64 bytes), base64-encoded, or hex-encoded.
func downloadSignature(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Cap the read; signatures are tiny.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, err
	}
	return decodeSignature(data)
}

// decodeSignature normalises a signature payload into raw ed25519 signature bytes.
func decodeSignature(data []byte) ([]byte, error) {
	if len(data) == ed25519.SignatureSize {
		return data, nil
	}

	trimmed := strings.TrimSpace(string(data))
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	return nil, fmt.Errorf("signature must be %d bytes (raw, base64, or hex)", ed25519.SignatureSize)
}
