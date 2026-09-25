// Package v1 defines the versioned, transport-neutral Spectra diagnostic contract.
package v1

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Version identifies this protocol major version.
const Version = "v1"

// Operation is a diagnostic action the remote agent may offer.
type Operation string

const (
	OperationHealth         Operation = "health"
	OperationInspect        Operation = "inspect"
	OperationSnapshotCreate Operation = "snapshot.create"
)

var errUnsupportedProtocolVersion = errors.New("unsupported protocol version")

func validOperation(op Operation) bool {
	s := string(op)
	if len(s) == 0 || len(s) > 64 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func validRequestID(id string) bool {
	if len(id) == 0 || len(id) > MaxRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == ':' || c == '-') {
			return false
		}
	}
	return true
}

// Request is one typed diagnostic request.
type Request struct {
	ProtocolVersion string          `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Operation       Operation       `json:"operation"`
	TimeoutMS       int             `json:"timeout_ms,omitempty"`
	Params          json.RawMessage `json:"params,omitempty"`
}

// Validate checks the request envelope; operation-specific policy belongs to the agent.
func (r Request) Validate() error {
	if r.ProtocolVersion != Version {
		return fmt.Errorf("protocol version %q: %w", r.ProtocolVersion, errUnsupportedProtocolVersion)
	}
	if !validRequestID(r.RequestID) {
		return fmt.Errorf("invalid request_id")
	}
	if !validOperation(r.Operation) {
		return fmt.Errorf("invalid operation")
	}
	if r.TimeoutMS < 0 || r.TimeoutMS > MaxTimeoutMS {
		return fmt.Errorf("invalid timeout_ms")
	}
	if len(r.Params) > 0 {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(r.Params, &obj); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
		if obj == nil {
			return fmt.Errorf("params must be a JSON object")
		}
	}
	return nil
}

// RequestErrorCode maps request validation failures to wire error codes; nil maps to an empty code.
func RequestErrorCode(err error) ErrorCode {
	if err == nil {
		return ""
	}
	if errors.Is(err, errUnsupportedProtocolVersion) {
		return CodeUnsupportedProtocolVersion
	}
	return CodeInvalidRequest
}

// Response is returned for every request.
type Response struct {
	ProtocolVersion string          `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           *Error          `json:"error,omitempty"`
}

// Validate checks the response envelope and its success or failure shape.
func (r Response) Validate() error {
	if r.ProtocolVersion != Version {
		return fmt.Errorf("protocol version %q: %w", r.ProtocolVersion, errUnsupportedProtocolVersion)
	}
	if (len(r.Result) == 0) == (r.Error == nil) {
		return fmt.Errorf("response must contain exactly one of result or error")
	}
	if r.Error != nil {
		if err := r.Error.Validate(); err != nil {
			return fmt.Errorf("response error: %w", err)
		}
		return nil
	}
	if !validNonNullJSON(r.Result) {
		return fmt.Errorf("result must be non-null JSON")
	}
	return nil
}

// ValidateFor checks the response and requires a non-empty matching request ID.
func (r Response) ValidateFor(req Request) error {
	if err := r.Validate(); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	if r.RequestID == "" || r.RequestID != req.RequestID {
		return fmt.Errorf("response request_id does not match request")
	}
	return nil
}

// Error is a stable failure shape for callers.
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// HealthResult is the result payload for OperationHealth.
type HealthResult struct {
	Capabilities CapabilityManifest `json:"capabilities"`
}

// InspectParams is the parameter payload for OperationInspect.
type InspectParams struct {
	AppPaths []string `json:"app_paths"`
}

// SnapshotCreateParams is the parameter payload for OperationSnapshotCreate.
type SnapshotCreateParams struct {
	IncludeApps bool `json:"include_apps"`
}
