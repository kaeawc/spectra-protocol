# spectra-protocol

`spectra-protocol` is the versioned, transport-neutral contract shared by
[Spectra](https://github.com/kaeawc/spectra), Spectra's local diagnostics CLI,
and [Spectra Proxy](https://github.com/kaeawc/spectra-proxy).

| Project | Owns |
| --- | --- |
| `spectra-protocol` | Versioned requests, results, errors, capabilities, and compatibility fixtures. Standard library only. |
| `spectra` | Local diagnostics and machine-readable output. It has no proxy or remote transport. |
| `spectra-proxy` | Controller, target agent, authentication, transport, installation, and updates. It depends on a released protocol version. |

Import the contract from:

```go
import "github.com/kaeawc/spectra-protocol/protocol/v1"
```

This package has no transport, authentication, installation, or command-
execution primitives.

## Versioning

Release the Go module with semantic version tags (`vX.Y.Z`). While the module
is `v0.x`, minor versions may change the Go API. Wire changes to protocol `v1`
must remain backward compatible for decoding; otherwise, bump the protocol
version. The wire protocol version string is separate from the Go module
version.

Run `make ci` before submitting changes or cutting a release.
