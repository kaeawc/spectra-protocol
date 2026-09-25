package release

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const signatureDomain = "spectra-release-manifest-v1\n"

type Signature struct {
	Schema    string `json:"schema"`
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

type TrustedKey struct {
	ID  string
	Key ed25519.PublicKey
}

// KeyID is the lowercase hex encoding of the first eight SHA-256 bytes.
func KeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:8])
}

// ParsePublicKey parses an ed25519: prefixed, standard-base64 public key.
func ParsePublicKey(s string) (TrustedKey, error) {
	const prefix = "ed25519:"
	if len(s) < len(prefix) || s[:len(prefix)] != prefix {
		return TrustedKey{}, fmt.Errorf("public key prefix: %w", ErrUntrustedKey)
	}
	pub, err := base64.StdEncoding.Strict().DecodeString(s[len(prefix):])
	if err != nil {
		return TrustedKey{}, fmt.Errorf("public key base64: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return TrustedKey{}, fmt.Errorf("public key length: %w", ErrUntrustedKey)
	}
	return TrustedKey{ID: KeyID(pub), Key: ed25519.PublicKey(pub)}, nil
}

// FormatPublicKey returns the public key in ParsePublicKey's accepted form.
func FormatPublicKey(pub ed25519.PublicKey) string {
	return "ed25519:" + base64.StdEncoding.EncodeToString(pub)
}

func decodeStrict(data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return fmt.Errorf("trailing JSON data: %w", err)
	}
	return nil
}

func signedMessage(manifestBytes []byte) []byte {
	message := make([]byte, 0, len(signatureDomain)+len(manifestBytes))
	message = append(message, signatureDomain...)
	return append(message, manifestBytes...)
}

// Sign signs the exact validated manifest bytes with the release domain prefix.
func Sign(manifestBytes []byte, priv ed25519.PrivateKey) ([]byte, error) {
	if len(manifestBytes) > MaxManifestBytes {
		return nil, fmt.Errorf("manifest size: %w", ErrInvalidManifest)
	}
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("private key length: %w", ErrUntrustedKey)
	}
	var manifest Manifest
	if err := decodeStrict(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("manifest JSON: %w: %w", ErrInvalidManifest, err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("manifest validation: %w", err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	if manifest.KeyID != KeyID(pub) {
		return nil, fmt.Errorf("manifest key_id differs from signing key: %w", ErrInvalidManifest)
	}
	sig := Signature{
		Schema:    SignatureSchema,
		KeyID:     manifest.KeyID,
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signedMessage(manifestBytes))),
	}
	data, err := json.Marshal(sig)
	if err != nil {
		return nil, fmt.Errorf("encode signature JSON: %w", err)
	}
	return data, nil
}

// Verify authenticates raw bytes before it decodes or validates the manifest.
func Verify(manifestBytes, signatureBytes []byte, trusted []TrustedKey) (Manifest, error) {
	// The order of these checks is security critical; see FORMAT.md.
	if len(signatureBytes) > MaxManifestBytes || len(manifestBytes) > MaxManifestBytes {
		return Manifest{}, fmt.Errorf("release metadata size: %w", ErrInvalidManifest)
	}
	var sig Signature
	if err := decodeStrict(signatureBytes, &sig); err != nil {
		return Manifest{}, fmt.Errorf("signature JSON: %w: %w", ErrBadSignature, err)
	}
	if sig.Schema != SignatureSchema {
		return Manifest{}, fmt.Errorf("signature schema: %w", ErrBadSignature)
	}
	var pub ed25519.PublicKey
	for _, key := range trusted {
		if key.ID == sig.KeyID && len(key.Key) == ed25519.PublicKeySize && KeyID(key.Key) == key.ID {
			pub = key.Key
			break
		}
	}
	if pub == nil {
		return Manifest{}, fmt.Errorf("signature key_id %q: %w", sig.KeyID, ErrUntrustedKey)
	}
	rawSig, err := base64.StdEncoding.Strict().DecodeString(sig.Signature)
	if err != nil || len(rawSig) != ed25519.SignatureSize {
		return Manifest{}, fmt.Errorf("signature encoding or length: %w", ErrBadSignature)
	}
	if !ed25519.Verify(pub, signedMessage(manifestBytes), rawSig) {
		return Manifest{}, fmt.Errorf("ed25519 verification: %w", ErrBadSignature)
	}
	var manifest Manifest
	if err := decodeStrict(manifestBytes, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("manifest JSON: %w: %w", ErrInvalidManifest, err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("manifest validation: %w", err)
	}
	if manifest.KeyID != sig.KeyID {
		return Manifest{}, fmt.Errorf("manifest key_id differs from signature key_id: %w", ErrInvalidManifest)
	}
	return manifest, nil
}
