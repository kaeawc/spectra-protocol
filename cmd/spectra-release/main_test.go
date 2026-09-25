package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	release "github.com/kaeawc/spectra-protocol/release/v1"
)

func run(cmd command, args []string, env func(string) string) (int, string, string) {
	var out, stderr strings.Builder
	code := cmd(args, &out, &stderr, env)
	return code, out.String(), stderr.String()
}

func TestKeygen(t *testing.T) {
	dir := t.TempDir()
	priv, pub := filepath.Join(dir, "private"), filepath.Join(dir, "public")
	code, out, errout := run(keygenCommand, []string{"--private-out", priv, "--public-out", pub}, func(string) string { return "" })
	if code != 0 || errout != "" {
		t.Fatalf("keygen = %d, %q", code, errout)
	}
	info, err := os.Stat(priv)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("private mode = %o", info.Mode().Perm())
	}
	seedText, err := os.ReadFile(priv)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(seedText)))
	if err != nil || len(seed) != ed25519.SeedSize {
		t.Fatalf("bad seed: %v", err)
	}
	trusted, err := release.ParsePublicKey(strings.TrimSpace(mustRead(t, pub)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != trusted.ID {
		t.Fatalf("key id output %q want %q", out, trusted.ID)
	}
	if code, _, stderr := run(keygenCommand, []string{"--private-out", priv, "--public-out", filepath.Join(dir, "other")}, func(string) string { return "" }); code != 1 || !strings.Contains(stderr, "exist") {
		t.Fatalf("overwrite = %d, %q", code, stderr)
	}
}

func TestReleaseWorkflow(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "private"), filepath.Join(dir, "public")
	if code, _, stderr := run(keygenCommand, []string{"--private-out", privatePath, "--public-out", publicPath}, func(string) string { return "" }); code != 0 {
		t.Fatalf("keygen: %d %s", code, stderr)
	}
	seedRaw, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	seedEncoded := strings.TrimSpace(string(seedRaw))
	trustedText := strings.TrimSpace(mustRead(t, publicPath))
	trusted, err := release.ParsePublicKey(trustedText)
	if err != nil {
		t.Fatal(err)
	}
	artifactsDir := filepath.Join(dir, "artifacts")
	if err := os.Mkdir(artifactsDir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"spectra_v1.2.3_darwin_arm64.tar.gz": []byte("fake-darwin-arm64"),
		"spectra_v1.2.3_linux_amd64.tar.gz":  []byte("fake-linux-amd64"),
	} {
		if err := os.WriteFile(filepath.Join(artifactsDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifestPath, signaturePath := filepath.Join(dir, "manifest.json"), filepath.Join(dir, "signature.json")
	manifestArgs := []string{"--version", "v1.2.3", "--public-key", trustedText, "--capabilities-schema-version", "1", "--published-at", "2026-09-24T12:00:00Z", "--out", manifestPath, filepath.Join(artifactsDir, "spectra_v1.2.3_linux_amd64.tar.gz"), filepath.Join(artifactsDir, "spectra_v1.2.3_darwin_arm64.tar.gz")}
	if code, _, stderr := run(manifestCommand, manifestArgs, func(string) string { return "" }); code != 0 {
		t.Fatalf("manifest: %d %s", code, stderr)
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest release.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Artifacts) != 2 || manifest.Artifacts[0].OS != "darwin" {
		t.Fatalf("artifacts not sorted: %+v", manifest.Artifacts)
	}
	if code, _, stderr := run(signCommand, []string{"--manifest", manifestPath, "--out", signaturePath}, func(k string) string {
		if k == "SPECTRA_RELEASE_ED25519_KEY" {
			return seedEncoded
		}
		return ""
	}); code != 0 {
		t.Fatalf("sign: %d %s", code, stderr)
	}
	verifyArgs := []string{"--manifest", manifestPath, "--signature", signaturePath, "--trusted-key", trustedText, "--artifacts-dir", artifactsDir}
	code, out, stderr := run(verifyCommand, verifyArgs, func(string) string { return "" })
	want := "ok v1.2.3 " + trusted.ID + "\n"
	if code != 0 || out != want {
		t.Fatalf("verify = %d %q stderr %q, want %q", code, out, stderr, want)
	}

	artifactPath := filepath.Join(artifactsDir, manifest.Artifacts[0].Path)
	if err := os.WriteFile(artifactPath, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := verify(verifyArgs); !errors.Is(err, release.ErrArtifactMismatch) {
		t.Fatalf("tampered artifact: %v", err)
	}

	otherPriv := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	wrongKey := release.FormatPublicKey(otherPriv.Public().(ed25519.PublicKey))
	wrongArgs := []string{"--manifest", manifestPath, "--signature", signaturePath, "--trusted-key", wrongKey}
	if code, _, stderr := run(verifyCommand, wrongArgs, func(string) string { return "" }); code != 1 || !strings.Contains(stderr, release.ErrUntrustedKey.Error()) {
		t.Fatalf("wrong key = %d %q", code, stderr)
	}
}

func TestManifestRejectsArtifactNames(t *testing.T) {
	dir := t.TempDir()
	priv := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	public := release.FormatPublicKey(priv.Public().(ed25519.PublicKey))
	for _, name := range []string{"bad_v1.2.3_linux_amd64.tar.gz", "spectra_v9.9.9_linux_amd64.tar.gz", "spectra_v1.2.3_linux_amd64.zip"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"--version", "v1.2.3", "--public-key", public, "--capabilities-schema-version", "1", "--published-at", time.Now().UTC().Format(time.RFC3339), "--out", filepath.Join(dir, "out.json"), path}
		if code, _, stderr := run(manifestCommand, args, func(string) string { return "" }); code != 1 || !strings.Contains(stderr, name) {
			t.Fatalf("name %q: %d %q", name, code, stderr)
		}
	}
}

func TestSignWithoutKey(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifest, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(signCommand, []string{"--manifest", manifest, "--out", filepath.Join(dir, "signature.json")}, func(string) string { return "" })
	if code != 1 || strings.TrimSpace(stderr) == "" || len(stderr) > 160 {
		t.Fatalf("sign without key: %d %q", code, stderr)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
