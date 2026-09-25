// Command spectra-release creates and verifies signed Spectra release metadata.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	release "github.com/kaeawc/spectra-protocol/release/v1"
)

type command func([]string, io.Writer, io.Writer, func(string) string) int

var commands = map[string]command{"keygen": keygenCommand, "manifest": manifestCommand, "sign": signCommand, "verify": verifyCommand}

func main() {
	name, args := "", []string(nil)
	if len(os.Args) > 1 {
		name, args = os.Args[1], os.Args[2:]
	}
	fn, ok := commands[name]
	if !ok {
		fn = usageCommand
		args = []string{name}
	}
	os.Exit(fn(args, os.Stdout, os.Stderr, os.Getenv))
}

func usageCommand(args []string, _, stderr io.Writer, _ func(string) string) int {
	if len(args) == 1 && args[0] != "" {
		return fail(stderr, args[0], 2, errors.New("unknown subcommand"))
	}
	fmt.Fprintln(stderr, "spectra-release: usage: spectra-release <keygen|manifest|sign|verify> [flags]")
	return 2
}

func fail(stderr io.Writer, sub string, code int, err error) int {
	fmt.Fprintf(stderr, "spectra-release %s: %v\n", sub, err)
	return code
}

func flagSet(name string, stderr io.Writer) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return f
}

func parseFlags(f *flag.FlagSet, args []string, stderr io.Writer, sub string) bool {
	if err := f.Parse(args); err != nil {
		fail(stderr, sub, 2, err)
		return false
	}
	return true
}

func require(stderr io.Writer, sub string, fields ...string) bool {
	for _, field := range fields {
		if field == "" {
			fail(stderr, sub, 2, errors.New("required flag value is missing"))
			return false
		}
	}
	return true
}

func keygenCommand(args []string, stdout, stderr io.Writer, _ func(string) string) int {
	f := flagSet("keygen", stderr)
	privOut := f.String("private-out", "", "private seed output path")
	pubOut := f.String("public-out", "", "public key output path")
	if !parseFlags(f, args, stderr, "keygen") {
		return 2
	}
	if f.NArg() != 0 {
		return fail(stderr, "keygen", 2, errors.New("unexpected positional arguments"))
	}
	if !require(stderr, "keygen", *privOut, *pubOut) {
		return 2
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fail(stderr, "keygen", 1, fmt.Errorf("generate key: %w", err))
	}
	privData := []byte(base64.StdEncoding.EncodeToString(priv.Seed()) + "\n")
	if err := createExclusive(*privOut, privData, 0600); err != nil {
		return fail(stderr, "keygen", 1, err)
	}
	if err := createExclusive(*pubOut, []byte(release.FormatPublicKey(pub)+"\n"), 0600); err != nil {
		_ = os.Remove(*privOut)
		return fail(stderr, "keygen", 1, err)
	}
	fmt.Fprintln(stdout, release.KeyID(pub))
	return 0
}

func createExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write %q: %w", path, err)
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close %q: %w", path, err)
	}
	return nil
}

func manifestCommand(args []string, stdout, stderr io.Writer, _ func(string) string) int {
	f := flagSet("manifest", stderr)
	version := f.String("version", "", "release version")
	publicKey := f.String("public-key", "", "ed25519 public key")
	capVersion := f.Int("capabilities-schema-version", 0, "capabilities schema version")
	published := f.String("published-at", "", "RFC3339 publication time")
	out := f.String("out", "", "manifest output path")
	if !parseFlags(f, args, stderr, "manifest") {
		return 2
	}
	if !require(stderr, "manifest", *version, *publicKey, *published, *out) {
		return 2
	}
	if *capVersion == 0 {
		return fail(stderr, "manifest", 2, errors.New("--capabilities-schema-version is required and must be non-zero"))
	}
	if f.NArg() == 0 {
		return fail(stderr, "manifest", 2, errors.New("at least one artifact path is required"))
	}
	when, err := time.Parse(time.RFC3339, *published)
	if err != nil {
		return fail(stderr, "manifest", 2, fmt.Errorf("invalid --published-at: %w", err))
	}
	trusted, err := release.ParsePublicKey(*publicKey)
	if err != nil {
		return fail(stderr, "manifest", 2, fmt.Errorf("invalid --public-key: %w", err))
	}
	artifacts := make([]release.Artifact, 0, f.NArg())
	for _, path := range f.Args() {
		a, err := readArtifact(path, *version)
		if err != nil {
			return fail(stderr, "manifest", 1, err)
		}
		artifacts = append(artifacts, a)
	}
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].OS != artifacts[j].OS {
			return artifacts[i].OS < artifacts[j].OS
		}
		return artifacts[i].Arch < artifacts[j].Arch
	})
	m := release.Manifest{Schema: release.ManifestSchema, Product: "spectra", Version: *version, PublishedAt: when, KeyID: trusted.ID, CapabilitiesSchemaVersion: *capVersion, Artifacts: artifacts}
	if err := m.Validate(); err != nil {
		return fail(stderr, "manifest", 1, err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fail(stderr, "manifest", 1, fmt.Errorf("marshal manifest: %w", err))
	}
	data = append(data, '\n')
	if err := atomicWrite(*out, data); err != nil {
		return fail(stderr, "manifest", 1, err)
	}
	return 0
}

func readArtifact(path, version string) (release.Artifact, error) {
	name := filepath.Base(path)
	parts := strings.Split(name, "_")
	if len(parts) != 4 || parts[0] != "spectra" || parts[3] == "" || !strings.HasSuffix(parts[3], ".tar.gz") {
		return release.Artifact{}, fmt.Errorf("artifact %q filename must match spectra_<version>_<os>_<arch>.tar.gz", name)
	}
	arch := strings.TrimSuffix(parts[3], ".tar.gz")
	if parts[1] != version {
		return release.Artifact{}, fmt.Errorf("artifact %q version does not match --version %q", name, version)
	}
	if parts[2] != "darwin" && parts[2] != "linux" {
		return release.Artifact{}, fmt.Errorf("artifact %q has unsupported OS %q", name, parts[2])
	}
	if arch != "amd64" && arch != "arm64" {
		return release.Artifact{}, fmt.Errorf("artifact %q has unsupported architecture %q", name, arch)
	}
	f, err := os.Open(path)
	if err != nil {
		return release.Artifact{}, fmt.Errorf("open artifact %q: %w", name, err)
	}
	h := sha256.New()
	n, copyErr := io.Copy(h, f)
	closeErr := f.Close()
	if copyErr != nil {
		return release.Artifact{}, fmt.Errorf("read artifact %q: %w", name, copyErr)
	}
	if closeErr != nil {
		return release.Artifact{}, fmt.Errorf("close artifact %q: %w", name, closeErr)
	}
	return release.Artifact{OS: parts[2], Arch: arch, Path: name, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n, Format: "tar.gz"}, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".spectra-release-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temporary output: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temporary output: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("replace %q: %w", path, err)
	}
	return nil
}

func signCommand(args []string, stdout, stderr io.Writer, env func(string) string) int {
	f := flagSet("sign", stderr)
	manifest := f.String("manifest", "", "manifest path")
	out := f.String("out", "", "signature output path")
	keyFile := f.String("private-key-file", "", "private seed file")
	if !parseFlags(f, args, stderr, "sign") {
		return 2
	}
	if f.NArg() != 0 {
		return fail(stderr, "sign", 2, errors.New("unexpected positional arguments"))
	}
	if !require(stderr, "sign", *manifest, *out) {
		return 2
	}
	raw, err := os.ReadFile(*manifest)
	if err != nil {
		return fail(stderr, "sign", 1, fmt.Errorf("read manifest: %w", err))
	}
	// A provided key file takes precedence over the environment variable.
	keyText := env("SPECTRA_RELEASE_ED25519_KEY")
	if *keyFile != "" {
		b, err := os.ReadFile(*keyFile)
		if err != nil {
			return fail(stderr, "sign", 1, fmt.Errorf("read private key file: %w", err))
		}
		keyText = string(b)
	}
	seed, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(keyText))
	if err != nil || len(seed) != ed25519.SeedSize {
		return fail(stderr, "sign", 1, errors.New("no valid private key seed available"))
	}
	sig, err := release.Sign(raw, ed25519.NewKeyFromSeed(seed))
	if err != nil {
		return fail(stderr, "sign", 1, err)
	}
	if err := atomicWrite(*out, sig); err != nil {
		return fail(stderr, "sign", 1, err)
	}
	return 0
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func verifyCommand(args []string, stdout, stderr io.Writer, env func(string) string) int {
	result, err := verify(args)
	if err != nil {
		code := 1
		var usage usageError
		if errors.As(err, &usage) {
			code = 2
		}
		return fail(stderr, "verify", code, err)
	}
	fmt.Fprintln(stdout, result)
	return 0
}

type usageError struct{ error }

func (e usageError) Unwrap() error { return e.error }

func verify(args []string) (string, error) {
	f := flagSet("verify", io.Discard)
	manifest := f.String("manifest", "", "manifest path")
	signature := f.String("signature", "", "signature path")
	artifactsDir := f.String("artifacts-dir", "", "artifact directory")
	var keys stringList
	f.Var(&keys, "trusted-key", "trusted ed25519 public key (repeatable)")
	if err := f.Parse(args); err != nil {
		return "", usageError{err}
	}
	if f.NArg() != 0 {
		return "", usageError{errors.New("unexpected positional arguments")}
	}
	if *manifest == "" || *signature == "" || len(keys) == 0 {
		return "", usageError{errors.New("--manifest, --signature, and at least one --trusted-key are required")}
	}
	trusted := make([]release.TrustedKey, 0, len(keys))
	for _, value := range keys {
		key, err := release.ParsePublicKey(value)
		if err != nil {
			return "", fmt.Errorf("invalid --trusted-key: %w", err)
		}
		trusted = append(trusted, key)
	}
	manifestBytes, err := os.ReadFile(*manifest)
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	signatureBytes, err := os.ReadFile(*signature)
	if err != nil {
		return "", fmt.Errorf("read signature: %w", err)
	}
	m, err := release.Verify(manifestBytes, signatureBytes, trusted)
	if err != nil {
		return "", err
	}
	if *artifactsDir != "" {
		for _, artifact := range m.Artifacts {
			path := filepath.Join(*artifactsDir, artifact.Path)
			file, err := os.Open(path)
			if err != nil {
				return "", fmt.Errorf("open artifact %q: %w", artifact.Path, err)
			}
			err = release.VerifyArtifactDigest(file, artifact)
			closeErr := file.Close()
			if err != nil {
				return "", fmt.Errorf("artifact %q: %w", artifact.Path, err)
			}
			if closeErr != nil {
				return "", fmt.Errorf("close artifact %q: %w", artifact.Path, closeErr)
			}
		}
	}
	return fmt.Sprintf("ok %s %s", m.Version, m.KeyID), nil
}
