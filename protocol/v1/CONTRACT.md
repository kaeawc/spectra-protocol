# Spectra wire contract v1

The wire version is `v1`. Messages are one JSON object per NDJSON line. The final line may omit its newline. Unknown top-level envelope fields are ignored for forward compatibility. Request frames are limited to 1 MiB and response frames to 16 MiB, excluding the newline. A writer checks the encoded JSON size before writing. A reader returns `ErrMessageTooLarge` once a line exceeds its configured byte limit. After `ReadMessage` returns `ErrMessageTooLarge`, the stream position is undefined, so an implementation must send at most one `message_too_large` error and then close the connection.

A request has `protocol_version`, `request_id`, `operation`, optional `timeout_ms`, and optional object `params`. The version must be `v1`. Request IDs contain 1–128 ASCII characters from `[A-Za-z0-9._:-]`. Operations match `^[a-z][a-z0-9_.]{0,63}$`; an unknown but well-formed operation is a valid envelope and receives `unsupported_operation` during dispatch. Timeout is 0–600000 ms. Version mismatch maps to `unsupported_protocol_version`; other request validation failures map to `invalid_request`. `RequestErrorCode(nil)` returns an empty code.

A response has the same version and request ID, plus exactly one of a non-null JSON `result` or an `error`. `ValidateFor` also requires a non-empty request ID equal to the paired request ID, even when the paired ID is empty. Error codes match `^[a-z][a-z0-9_]{0,63}$`; unknown well-formed codes remain valid. Error messages contain 1–4096 bytes. `NewError` repairs invalid UTF-8 and truncates at a rune boundary. The defined codes are `invalid_request`, `unsupported_protocol_version`, `unsupported_operation`, `message_too_large`, `permission_denied`, `unavailable`, `incompatible_spectra`, `timeout`, `execution_failed`, and `internal`.

A capability manifest advertises `protocol_versions`, `spectra_version`, `agent_version`, `operations`, and `limits`. Each operation has a unique well-formed name and optional result schema reference. Manifest validation requires a non-empty protocol version list and positive limits no higher than 1 MiB request, 16 MiB response, and 600000 ms timeout. Negotiation separately requires `v1`, the requested operation, and a supported advertised result schema. Failure codes are `unsupported_protocol_version`, `unsupported_operation`, and `incompatible_spectra`, respectively. `CodeOf` returns `internal` when no coded error exists.

Diagnostic results wrap `schema`, `spectra_version`, and non-null JSON `data`. `inspect` expects `spectra.inspect` version 1; `snapshot.create` expects `spectra.snapshot` version 1; `process.list` expects `spectra.process.list` version 1; `jvm.list` expects `spectra.jvm.list` version 1. Unknown or mismatched schemas return `incompatible_spectra`. A failure response, missing data, or invalid result JSON returns `invalid_request`. Health results contain `capabilities`, which must pass manifest validation. These envelopes do not define transport, authentication, execution, or local agent policy.

The embedded JSON fixture arrays under `conformance/testdata` are normative interoperability cases. A request case with `valid: true` and `error_code: "unsupported_operation"` passes envelope validation but requires that code from downstream dispatch. A manifest case lacking `v1` passes structural validation but fails negotiation. Consumers should run all cases and apply their paired requests when validating responses.

## Spectra capabilities document

`SpectraCapabilities` models the JSON emitted by `spectra capabilities --json`; its `schema` is `spectra.capabilities` version 1, bounded by `MaxCapabilitiesBytes` (1 MiB).
It contains the Spectra version, OS, architecture, and named `SpectraInterface` entries with argv, text or JSON output, and an optional result schema.
Validation requires non-empty platform/version fields, unique well-formed interface names, output-appropriate schemas, and exactly one JSON `capabilities` interface declaring this schema.
Every `Validate` failure uses `incompatible_spectra`, because a nonconforming document identifies an incompatible Spectra.
`ResultSchemaFor(op)` requires the mapped interface and the operation's supported result schema name and version; unmapped operations use `unsupported_operation`, while absent or mismatched interfaces use `incompatible_spectra`.

## Read-only list operations

`process.list` and `jvm.list` are read-only and take no parameters (`ProcessListParams` and `JVMListParams` have no fields); an agent rejects any supplied parameter as unknown with `invalid_request`, as it does for unknown fields of other operations.
`spectra.process.list` data is `{"sampled_at": "<RFC 3339>", "processes": [...]}`: `sampled_at` is optional and, when present, an RFC 3339 timestamp string (fractional seconds allowed) of when the rows were collected; each `processes` row is exactly the object `spectra process --json --classify` prints for one process.
`spectra.jvm.list` data is `{"jvms": [...]}`: each entry is exactly the object `spectra jvm --json` prints for one JVM.
`DecodeResult` requires the data to be a JSON object whose `processes` or `jvms` member is a JSON array (possibly empty, never null) and, for `sampled_at`, a valid timestamp string; any other shape returns `invalid_request`. Row contents belong to Spectra and are not validated, and unknown data members are ignored.
Neither operation maps to a `SpectraCapabilities` interface, so `ResultSchemaFor` returns `unsupported_operation` for them; the agent decides availability itself and advertises them in its manifest only when it can serve them.
