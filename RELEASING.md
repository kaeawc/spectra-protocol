# Releasing

1. Ensure `main` is green.
2. Run `make ci` locally.
3. Choose the next semantic version, such as `v0.2.0`.
4. Create an annotated tag:

   ```sh
   git tag -a vX.Y.Z -m "spectra-protocol vX.Y.Z"
   ```

5. Push the tag: `git push origin vX.Y.Z`.
6. Verify publication:

   ```sh
   GOPROXY=https://proxy.golang.org GOFLAGS=-mod=mod go list -m github.com/kaeawc/spectra-protocol@vX.Y.Z
   ```

7. Bump the dependency in `spectra-proxy`:

   ```sh
   go get github.com/kaeawc/spectra-protocol@vX.Y.Z && go mod tidy
   ```

Never move or delete a published tag. The Go checksum database makes published
tags immutable; fix forward with a new patch version instead.

## Compatibility fixtures

Any wire change must update the compatibility fixtures in the same release.
