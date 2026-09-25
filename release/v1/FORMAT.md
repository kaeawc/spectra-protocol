# Spectra signed release metadata, version 1

This document defines the wire format for `github.com/kaeawc/spectra-protocol/release/v1`.
Spectra core produces the manifest and detached signature. Spectra Proxy verifies
both files and selects an artifact before downloading or installing anything.

## Files and signatures

The manifest is a UTF-8 JSON object with fields `schema`, `product`, `version`,
`published_at`, `key_id`, `capabilities_schema_version`, and `artifacts`.
Its `schema` is exactly `spectra.release/1`; `product` is exactly `spectra`.
`published_at` is a JSON timestamp accepted by Go's `time.Time` decoder.
The signature file is a separate JSON object with `schema`, `key_id`, and
`signature`; its `schema` is exactly `spectra.release-signature/1`.
Unknown fields and data after either JSON value are rejected.

`signature` is standard padded base64 encoding of a 64-byte Ed25519 signature.
The signed message is the byte sequence `spectra-release-manifest-v1\n` followed
immediately by the exact manifest file bytes. No JSON normalization, trimming,
or reserialization occurs. Any byte change invalidates the signature.
An Ed25519 public key is distributed as `ed25519:` followed by standard padded
base64 of its 32 bytes. Its `key_id` is the lowercase hex encoding of the first
eight bytes of SHA-256 of those 32 bytes.

Both the manifest and signature input files are capped at `1 << 20` bytes.
The signature JSON should be tiny; the same 1 MiB cap prevents unbounded
signature parsing and provides one simple metadata input limit.

## Verification order and trust

Verifiers MUST perform these steps in order:

1. Enforce both input size caps.
2. Strictly decode the signature JSON and check its schema.
3. Resolve `signature.key_id` against locally trusted public keys. A missing
   or incorrectly identified key is `ErrUntrustedKey`.
4. Verify Ed25519 over the domain prefix plus **raw** manifest bytes.
   Malformed signature encoding or failed verification is `ErrBadSignature`.
5. Only after verification, strictly decode the manifest JSON.
6. Validate all manifest fields and artifacts.
7. Require `manifest.key_id == signature.key_id`. A mismatch is
   `ErrInvalidManifest`: the authenticated manifest is internally inconsistent
   with the key used to verify it.
8. Return the validated manifest for platform selection.

Malformed or invalid manifest JSON is `ErrInvalidManifest`. A verifier MUST NOT
parse the manifest before cryptographic verification, even to read its key ID.
The trust store comes from the verifier's own configuration; metadata does not
grant trust to a key.

## Manifest constraints

`version` is `vMAJOR.MINOR.PATCH` with an optional SemVer prerelease. Numeric
components have no leading zeroes. Build metadata is forbidden. Precedence
follows SemVer section 11, including numeric versus alphanumeric prerelease
identifiers. Major, minor, and patch must fit in Go `int`.

`key_id` is exactly 16 lowercase hexadecimal characters.
`capabilities_schema_version` is an integer at least 1. There is at least one
artifact. Each artifact has exactly one `os` (`darwin` or `linux`) and `arch`
(`amd64` or `arm64`) pair; pairs cannot repeat. `format` is exactly `tar.gz`.
`size` is greater than zero and at most `512 << 20` bytes. `sha256` is exactly
64 lowercase hexadecimal characters and covers the artifact file bytes.

`path` is a single relative file name, 1 to 255 bytes long. It cannot start
with `.`, contain `/`, `\\`, `:`, or `%`, or equal `.` or `..`. Every byte must
match `[A-Za-z0-9._-]`. Platform selection uses an exact `os`/`arch` match;
absence of a match is `ErrUnsupportedPlatform`. After download, the verifier
checks exactly the declared byte count and SHA-256 digest before installation.

## Test fixtures

**TEST-ONLY / NOT FOR PRODUCTION.** The checked-in fixtures use an Ed25519 key
derived from the 32-byte seed `sha256("spectra-release-test-key")`.
Its public key is
`ed25519:5UZFRPeIBtPPQ3YyMlZ07u/tGoxTagFx2gk7aSANkH8=`.
Run `go test ./release/v1/... -run TestGenerateFixtures -update` to regenerate
the deterministic fixtures. Normal test runs only read them.
