package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// DiagnosticResult wraps a versioned diagnostic payload.
type DiagnosticResult struct {
	Schema         SchemaRef       `json:"schema"`
	SpectraVersion string          `json:"spectra_version"`
	Data           json.RawMessage `json:"data"`
}

const (
	// SchemaInspect identifies inspect results.
	SchemaInspect = "spectra.inspect"
	// SchemaSnapshot identifies snapshot results.
	SchemaSnapshot = "spectra.snapshot"
	// SchemaProcessList identifies process.list results.
	SchemaProcessList = "spectra.process.list"
	// SchemaJVMList identifies jvm.list results.
	SchemaJVMList = "spectra.jvm.list"
)

// ProcessListData is the data payload of a spectra.process.list result. Each
// process row is the object `spectra process --json --classify` prints; the
// protocol does not constrain row contents.
type ProcessListData struct {
	SampledAt string            `json:"sampled_at,omitempty"`
	Processes []json.RawMessage `json:"processes"`
}

// JVMListData is the data payload of a spectra.jvm.list result. Each entry is
// the object `spectra jvm --json` prints; the protocol does not constrain
// entry contents.
type JVMListData struct {
	JVMs []json.RawMessage `json:"jvms"`
}

// ResultSchemaName identifies the expected schema for an operation.
func ResultSchemaName(op Operation) (string, bool) {
	switch op {
	case OperationInspect:
		return SchemaInspect, true
	case OperationSnapshotCreate:
		return SchemaSnapshot, true
	case OperationProcessList:
		return SchemaProcessList, true
	case OperationJVMList:
		return SchemaJVMList, true
	}
	return "", false
}

// SupportedResultSchemaVersion returns the supported version for a known schema.
func SupportedResultSchemaVersion(name string) (int, bool) {
	switch name {
	case SchemaInspect, SchemaSnapshot, SchemaCapabilities, SchemaProcessList, SchemaJVMList:
		return 1, true
	}
	return 0, false
}

// CheckResultSchema rejects unknown or unsupported diagnostic schemas.
func CheckResultSchema(s SchemaRef) error {
	v, ok := SupportedResultSchemaVersion(s.Name)
	if !ok || s.Version != v {
		return &CodedError{CodeIncompatibleSpectra, fmt.Errorf("unsupported result schema %q version %d", s.Name, s.Version)}
	}
	return nil
}

func validNonNullJSON(raw json.RawMessage) bool {
	if len(raw) == 0 || !json.Valid(raw) {
		return false
	}
	return !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func successResult(resp Response) error {
	if err := resp.Validate(); err != nil {
		return &CodedError{CodeInvalidRequest, fmt.Errorf("response: %w", err)}
	}
	if resp.Error != nil {
		return &CodedError{CodeInvalidRequest, fmt.Errorf("response contains an error")}
	}
	return nil
}

// DecodeResult validates and decodes a successful diagnostic result.
func DecodeResult(resp Response, op Operation) (DiagnosticResult, error) {
	if err := successResult(resp); err != nil {
		return DiagnosticResult{}, err
	}
	expected, ok := ResultSchemaName(op)
	if !ok {
		return DiagnosticResult{}, &CodedError{CodeIncompatibleSpectra, fmt.Errorf("no result schema for operation %q", op)}
	}
	var decoded DiagnosticResult
	if err := json.Unmarshal(resp.Result, &decoded); err != nil {
		return DiagnosticResult{}, &CodedError{CodeInvalidRequest, fmt.Errorf("decode result: %w", err)}
	}
	if decoded.Schema.Name != expected {
		return DiagnosticResult{}, &CodedError{CodeIncompatibleSpectra, fmt.Errorf("schema %q does not match operation %q", decoded.Schema.Name, op)}
	}
	if err := CheckResultSchema(decoded.Schema); err != nil {
		return DiagnosticResult{}, fmt.Errorf("result: %w", err)
	}
	if !validNonNullJSON(decoded.Data) {
		return DiagnosticResult{}, &CodedError{CodeInvalidRequest, fmt.Errorf("diagnostic data must be non-null JSON")}
	}
	if err := checkDataShape(decoded.Schema.Name, decoded.Data); err != nil {
		return DiagnosticResult{}, &CodedError{CodeInvalidRequest, fmt.Errorf("%s data: %w", decoded.Schema.Name, err)}
	}
	return decoded, nil
}

// checkDataShape enforces the envelope of list schemas: an object carrying the
// named array. Row contents belong to Spectra and are not validated here.
func checkDataShape(schema string, data json.RawMessage) error {
	var field string
	switch schema {
	case SchemaProcessList:
		field = "processes"
	case SchemaJVMList:
		field = "jvms"
	default:
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil || obj == nil {
		return fmt.Errorf("must be a JSON object")
	}
	var rows []json.RawMessage
	if raw, ok := obj[field]; !ok || json.Unmarshal(raw, &rows) != nil || rows == nil {
		return fmt.Errorf("%s must be a JSON array", field)
	}
	if raw, ok := obj["sampled_at"]; ok && schema == SchemaProcessList {
		var at string
		if json.Unmarshal(raw, &at) != nil {
			return fmt.Errorf("sampled_at must be an RFC 3339 string")
		}
		if _, err := time.Parse(time.RFC3339, at); err != nil {
			return fmt.Errorf("sampled_at must be an RFC 3339 string")
		}
	}
	return nil
}

// DecodeHealth validates and decodes a successful health result and manifest.
func DecodeHealth(resp Response) (HealthResult, error) {
	if err := successResult(resp); err != nil {
		return HealthResult{}, err
	}
	var health HealthResult
	if err := json.Unmarshal(resp.Result, &health); err != nil {
		return HealthResult{}, fmt.Errorf("decode health: %w", err)
	}
	if err := health.Capabilities.Validate(); err != nil {
		return HealthResult{}, fmt.Errorf("health capabilities: %w", err)
	}
	return health, nil
}
