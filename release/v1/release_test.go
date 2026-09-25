package release

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "regenerate testdata fixtures")

func testKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("spectra-release-test-key"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func testManifest() Manifest {
	pub := testKey().Public().(ed25519.PublicKey)
	return Manifest{
		Schema: ManifestSchema, Product: "spectra", Version: "v1.2.3",
		PublishedAt: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
		KeyID:       KeyID(pub), CapabilitiesSchemaVersion: 1,
		Artifacts: []Artifact{{OS: "darwin", Arch: "arm64", Path: "spectra-darwin-arm64.tar.gz",
			SHA256: strings.Repeat("a", 64), Size: 123, Format: "tar.gz"}},
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGenerateFixtures(t *testing.T) {
	if !*update {
		t.Skip("pass -update to regenerate fixtures")
	}
	manifest := append(mustJSON(t, testManifest()), '\n')
	sig, err := Sign(manifest, testKey())
	if err != nil {
		t.Fatal(err)
	}
	// Change one byte in the version field: v1.2.3 becomes v1.2.4.
	tampered := bytes.Replace(manifest, []byte(`"version":"v1.2.3"`), []byte(`"version":"v1.2.4"`), 1)
	if bytes.Equal(tampered, manifest) {
		t.Fatal("fixture version field missing")
	}
	wrong := Signature{Schema: SignatureSchema, KeyID: strings.Repeat("0", 16), Signature: sigString(t, sig)}
	files := map[string][]byte{
		"valid.manifest.json":    manifest,
		"valid.sig.json":         append(sig, '\n'),
		"tampered.manifest.json": tampered,
		"wrong-key.sig.json":     append(mustJSON(t, wrong), '\n'),
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join("testdata", name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func sigString(t *testing.T, data []byte) string {
	t.Helper()
	var s Signature
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	return s.Signature
}

func TestCheckedInFixtures(t *testing.T) {
	pub := testKey().Public().(ed25519.PublicKey)
	const published = "ed25519:5UZFRPeIBtPPQ3YyMlZ07u/tGoxTagFx2gk7aSANkH8="
	if got := FormatPublicKey(pub); got != published {
		t.Fatalf("published test public key mismatch: %s", got)
	}
	key, err := ParsePublicKey(published)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Verify(fixture(t, "valid.manifest.json"), fixture(t, "valid.sig.json"), []TrustedKey{key})
	if err != nil || m.Version != "v1.2.3" {
		t.Fatalf("valid fixture: manifest=%+v error=%v", m, err)
	}
	// tampered.manifest.json differs from valid.manifest.json by exactly one
	// byte: the version patch digit changes from 3 to 4.
	valid, tampered := fixture(t, "valid.manifest.json"), fixture(t, "tampered.manifest.json")
	differences := 0
	for i := range valid {
		if i >= len(tampered) || valid[i] != tampered[i] {
			differences++
		}
	}
	if len(valid) != len(tampered) || differences != 1 {
		t.Fatalf("tampered fixture has %d byte differences", differences)
	}
	if _, err := Verify(tampered, fixture(t, "valid.sig.json"), []TrustedKey{key}); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("tampered fixture: %v", err)
	}
	if _, err := Verify(valid, fixture(t, "wrong-key.sig.json"), []TrustedKey{key}); !errors.Is(err, ErrUntrustedKey) {
		t.Fatalf("wrong-key fixture: %v", err)
	}
}

func signedRaw(t *testing.T, manifest []byte) []byte {
	t.Helper()
	s := Signature{Schema: SignatureSchema, KeyID: testManifest().KeyID,
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(testKey(), signedMessage(manifest)))}
	return mustJSON(t, s)
}

func TestInvalidManifests(t *testing.T) {
	base := testManifest()
	cases := []struct {
		name string
		edit func(*Manifest)
	}{
		{"parent path", func(m *Manifest) { m.Artifacts[0].Path = "../spectra.tar.gz" }},
		{"URL path", func(m *Manifest) { m.Artifacts[0].Path = "https://x/y" }},
		{"slash path", func(m *Manifest) { m.Artifacts[0].Path = "a/b" }},
		{"empty path", func(m *Manifest) { m.Artifacts[0].Path = "" }},
		{"backslash path", func(m *Manifest) { m.Artifacts[0].Path = `a\b` }},
		{"duplicate platform", func(m *Manifest) { m.Artifacts = append(m.Artifacts, m.Artifacts[0]) }},
		{"short sha256", func(m *Manifest) { m.Artifacts[0].SHA256 = "abc" }},
		{"nonhex sha256", func(m *Manifest) { m.Artifacts[0].SHA256 = strings.Repeat("z", 64) }},
		{"zero size", func(m *Manifest) { m.Artifacts[0].Size = 0 }},
		{"oversize", func(m *Manifest) { m.Artifacts[0].Size = MaxArtifactBytes + 1 }},
		{"missing v", func(m *Manifest) { m.Version = "1.2.3" }},
		{"missing patch", func(m *Manifest) { m.Version = "v1.2" }},
		{"build metadata", func(m *Manifest) { m.Version = "v1.2.3+meta" }},
	}
	key := TrustedKey{ID: base.KeyID, Key: testKey().Public().(ed25519.PublicKey)}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			m.Artifacts = append([]Artifact(nil), base.Artifacts...)
			tc.edit(&m)
			if err := m.Validate(); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("Validate: %v", err)
			}
			raw := mustJSON(t, m)
			if _, err := Verify(raw, signedRaw(t, raw), []TrustedKey{key}); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("Verify: %v", err)
			}
		})
	}
	valid := mustJSON(t, base)
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"unknown field", bytes.Replace(valid, []byte(`"product":"spectra"`), []byte(`"product":"spectra","unexpected":true`), 1)},
		{"trailing garbage", append(append([]byte(nil), valid...), []byte(" garbage")...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Verify(tc.raw, signedRaw(t, tc.raw), []TrustedKey{key}); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("Verify: %v", err)
			}
		})
	}
}

func TestVerificationOrderAndBinding(t *testing.T) {
	key := TrustedKey{ID: testManifest().KeyID, Key: testKey().Public().(ed25519.PublicKey)}
	invalid := []byte(`not JSON`)
	if _, err := Verify(invalid, signedRaw(t, mustJSON(t, testManifest())), []TrustedKey{key}); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("manifest parsed before signature verified: %v", err)
	}
	other := testManifest()
	other.KeyID = strings.Repeat("0", 16)
	raw := mustJSON(t, other)
	if _, err := Verify(raw, signedRaw(t, raw), []TrustedKey{key}); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("signed manifest/signature key mismatch: %v", err)
	}
	if _, err := Sign(raw, testKey()); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("Sign key mismatch: %v", err)
	}
	if _, err := Verify(make([]byte, MaxManifestBytes+1), []byte("x"), nil); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("manifest size cap: %v", err)
	}
	if _, err := Verify(nil, make([]byte, MaxManifestBytes+1), nil); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("signature size cap: %v", err)
	}
	valid := mustJSON(t, testManifest())
	signature := signedRaw(t, valid)
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"unknown signature field", bytes.Replace(signature, []byte(`"schema":`), []byte(`"unexpected":true,"schema":`), 1)},
		{"trailing signature data", append(append([]byte(nil), signature...), []byte(" garbage")...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Verify(valid, tc.raw, []TrustedKey{key}); !errors.Is(err, ErrBadSignature) {
				t.Fatalf("signature strict decode: %v", err)
			}
		})
	}
	if _, err := Verify(invalid, signature, nil); !errors.Is(err, ErrUntrustedKey) {
		t.Fatalf("trust lookup must precede manifest parsing: %v", err)
	}
	if _, err := Sign(append(valid, []byte(" garbage")...), testKey()); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("Sign strict decode: %v", err)
	}
}

func TestVersionPrecedence(t *testing.T) {
	ordered := []string{"v1.0.0-alpha", "v1.0.0-alpha.1", "v1.0.0-alpha.beta", "v1.0.0-beta", "v1.0.0-beta.2", "v1.0.0-beta.11", "v1.0.0-rc.1", "v1.0.0"}
	for i := 0; i < len(ordered)-1; i++ {
		got, err := CompareVersions(ordered[i], ordered[i+1])
		if err != nil || got != -1 {
			t.Fatalf("%s vs %s: %d, %v", ordered[i], ordered[i+1], got, err)
		}
	}
	for _, bad := range []string{"v01.2.3", "v1.2.3-01", "v1.2.3-", "v1.2.3+a", "v1.2.3-a..b"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	got, err := CompareVersions("v1.0.0-999999999999999999999999999999999", "v1.0.0-a")
	if err != nil || got != -1 {
		t.Fatalf("numeric versus alpha: %d, %v", got, err)
	}
}

type countingReader struct {
	data []byte
	read int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n := copy(p, r.data[r.read:])
	r.read += n
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func TestArtifactSelectionAndDigest(t *testing.T) {
	m := testManifest()
	if _, err := m.ArtifactFor("linux", "amd64"); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("unsupported platform: %v", err)
	}
	if _, err := m.ArtifactFor("darwin", "arm64"); err != nil {
		t.Fatal(err)
	}
	content := []byte("artifact")
	sum := sha256.Sum256(content)
	a := m.Artifacts[0]
	a.Size = int64(len(content))
	a.SHA256 = hex.EncodeToString(sum[:])
	if err := VerifyArtifactDigest(bytes.NewReader(content), a); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{content[:len(content)-1], append(append([]byte(nil), content...), '!'), []byte("artifacT")} {
		r := &countingReader{data: data}
		if err := VerifyArtifactDigest(r, a); err == nil {
			t.Fatalf("accepted %q", data)
		}
		if r.read > len(content)+1 {
			t.Fatalf("read %d bytes", r.read)
		}
	}
}
