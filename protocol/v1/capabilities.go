package v1

import (
	"errors"
	"fmt"
)

// SchemaRef identifies a diagnostic result schema.
type SchemaRef struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// OperationCapability describes one offered operation.
type OperationCapability struct {
	Name         Operation  `json:"name"`
	ResultSchema *SchemaRef `json:"result_schema,omitempty"`
}

// Limits are the target's advertised message and timeout ceilings.
type Limits struct {
	MaxRequestBytes  int64 `json:"max_request_bytes"`
	MaxResponseBytes int64 `json:"max_response_bytes"`
	MaxTimeoutMS     int   `json:"max_timeout_ms"`
}

// CapabilityManifest advertises protocol versions, operations, and limits.
type CapabilityManifest struct {
	ProtocolVersions []string              `json:"protocol_versions"`
	SpectraVersion   string                `json:"spectra_version"`
	AgentVersion     string                `json:"agent_version"`
	Operations       []OperationCapability `json:"operations"`
	Limits           Limits                `json:"limits"`
}

// Validate checks manifest structure and local maximum limits.
func (m CapabilityManifest) Validate() error {
	if len(m.ProtocolVersions) == 0 {
		return fmt.Errorf("protocol_versions is required")
	}
	seen := make(map[Operation]bool)
	for _, op := range m.Operations {
		if !validOperation(op.Name) {
			return fmt.Errorf("invalid operation %q", op.Name)
		}
		if seen[op.Name] {
			return fmt.Errorf("duplicate operation %q", op.Name)
		}
		seen[op.Name] = true
	}
	if m.Limits.MaxRequestBytes <= 0 || m.Limits.MaxRequestBytes > MaxRequestBytes {
		return fmt.Errorf("invalid max_request_bytes")
	}
	if m.Limits.MaxResponseBytes <= 0 || m.Limits.MaxResponseBytes > MaxResponseBytes {
		return fmt.Errorf("invalid max_response_bytes")
	}
	if m.Limits.MaxTimeoutMS <= 0 || m.Limits.MaxTimeoutMS > MaxTimeoutMS {
		return fmt.Errorf("invalid max_timeout_ms")
	}
	return nil
}

// Supports returns the capability for an offered operation.
func (m CapabilityManifest) Supports(op Operation) (OperationCapability, bool) {
	for _, c := range m.Operations {
		if c.Name == op {
			return c, true
		}
	}
	return OperationCapability{}, false
}

// CodedError carries a stable wire code while preserving its cause.
type CodedError struct {
	Code ErrorCode
	Err  error
}

// Error returns the underlying error text.
func (e *CodedError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == nil {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

// Unwrap returns the cause.
func (e *CodedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CodeOf finds a coded error in the cause chain, defaulting to CodeInternal.
func CodeOf(err error) ErrorCode {
	var coded *CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return CodeInternal
}

// Negotiate checks protocol, operation, and advertised result schema support.
func Negotiate(m CapabilityManifest, op Operation) (OperationCapability, error) {
	found := false
	for _, version := range m.ProtocolVersions {
		if version == Version {
			found = true
			break
		}
	}
	if !found {
		return OperationCapability{}, &CodedError{CodeUnsupportedProtocolVersion, fmt.Errorf("protocol %s unavailable", Version)}
	}
	capability, ok := m.Supports(op)
	if !ok {
		return OperationCapability{}, &CodedError{CodeUnsupportedOperation, fmt.Errorf("operation %q unavailable", op)}
	}
	if capability.ResultSchema != nil {
		if err := CheckResultSchema(*capability.ResultSchema); err != nil {
			return OperationCapability{}, fmt.Errorf("result schema: %w", err)
		}
	}
	return capability, nil
}
