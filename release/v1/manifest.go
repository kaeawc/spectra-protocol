package release

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"
)

const (
	ManifestSchema   = "spectra.release/1"
	SignatureSchema  = "spectra.release-signature/1"
	MaxManifestBytes = 1 << 20
	MaxArtifactBytes = 512 << 20
)

var (
	ErrUntrustedKey        = errors.New("untrusted release key")
	ErrBadSignature        = errors.New("bad release signature")
	ErrInvalidManifest     = errors.New("invalid release manifest")
	ErrUnsupportedPlatform = errors.New("unsupported release platform")
)

// ErrArtifactMismatch indicates that an artifact's size or digest does not match its manifest.
var ErrArtifactMismatch = errors.New("artifact digest or size mismatch")

type Artifact struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Format string `json:"format"`
}

type Manifest struct {
	Schema                    string     `json:"schema"`
	Product                   string     `json:"product"`
	Version                   string     `json:"version"`
	PublishedAt               time.Time  `json:"published_at"`
	KeyID                     string     `json:"key_id"`
	CapabilitiesSchemaVersion int        `json:"capabilities_schema_version"`
	Artifacts                 []Artifact `json:"artifacts"`
}

var safeFileName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func lowerHex(s string, bytes int) bool {
	if len(s) != bytes*2 {
		return false
	}
	for i := range s {
		if !((s[i] >= '0' && s[i] <= '9') || (s[i] >= 'a' && s[i] <= 'f')) {
			return false
		}
	}
	return true
}

// Validate checks all fields that control release selection and installation.
func (m Manifest) Validate() error {
	if m.Schema != ManifestSchema || m.Product != "spectra" {
		return fmt.Errorf("schema or product: %w", ErrInvalidManifest)
	}
	if _, err := ParseVersion(m.Version); err != nil {
		return fmt.Errorf("version: %w", err)
	}
	if !lowerHex(m.KeyID, 8) {
		return fmt.Errorf("key_id: %w", ErrInvalidManifest)
	}
	if m.CapabilitiesSchemaVersion < 1 {
		return fmt.Errorf("capabilities_schema_version: %w", ErrInvalidManifest)
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("artifacts: %w", ErrInvalidManifest)
	}
	seen := make(map[string]bool, len(m.Artifacts))
	for i, a := range m.Artifacts {
		if a.OS != "darwin" && a.OS != "linux" {
			return fmt.Errorf("artifact %d os: %w", i, ErrInvalidManifest)
		}
		if a.Arch != "amd64" && a.Arch != "arm64" {
			return fmt.Errorf("artifact %d arch: %w", i, ErrInvalidManifest)
		}
		pair := a.OS + "/" + a.Arch
		if seen[pair] {
			return fmt.Errorf("artifact %d duplicate platform %s: %w", i, pair, ErrInvalidManifest)
		}
		seen[pair] = true
		if len(a.Path) == 0 || len(a.Path) > 255 || a.Path[0] == '.' || a.Path == ".." || !safeFileName.MatchString(a.Path) {
			return fmt.Errorf("artifact %d path: %w", i, ErrInvalidManifest)
		}
		if !lowerHex(a.SHA256, sha256.Size) {
			return fmt.Errorf("artifact %d sha256: %w", i, ErrInvalidManifest)
		}
		if a.Size <= 0 || a.Size > MaxArtifactBytes {
			return fmt.Errorf("artifact %d size: %w", i, ErrInvalidManifest)
		}
		if a.Format != "tar.gz" {
			return fmt.Errorf("artifact %d format: %w", i, ErrInvalidManifest)
		}
	}
	return nil
}

// ArtifactFor selects the artifact for an exact GOOS/GOARCH pair.
func (m Manifest) ArtifactFor(goos, goarch string) (Artifact, error) {
	for _, a := range m.Artifacts {
		if a.OS == goos && a.Arch == goarch {
			return a, nil
		}
	}
	return Artifact{}, fmt.Errorf("%s/%s: %w", goos, goarch, ErrUnsupportedPlatform)
}

// VerifyArtifactDigest checks the exact declared size and SHA-256 digest.
func VerifyArtifactDigest(r io.Reader, a Artifact) error {
	if a.Size <= 0 || a.Size > MaxArtifactBytes || !lowerHex(a.SHA256, sha256.Size) {
		return fmt.Errorf("artifact metadata: %w", ErrInvalidManifest)
	}
	want, err := hex.DecodeString(a.SHA256)
	if err != nil {
		return fmt.Errorf("artifact digest: %w", err)
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(r, a.Size+1))
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	if n != a.Size {
		return fmt.Errorf("artifact size %d, expected %d: %w", n, a.Size, ErrArtifactMismatch)
	}
	if subtle.ConstantTimeCompare(h.Sum(nil), want) != 1 {
		return fmt.Errorf("artifact sha256: %w", ErrArtifactMismatch)
	}
	return nil
}
