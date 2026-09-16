// Package v1 defines the versioned, transport-neutral contract between
// Spectra and Spectra Remote. It deliberately contains no transport,
// authentication, installation, or command-execution primitives.
package v1

import (
	"encoding/json"
	"fmt"
	"strings"
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

// CapabilityManifest tells a controller exactly which typed operations a
// target supports. A controller must not infer support from a version alone.
type CapabilityManifest struct {
	ProtocolVersion string      `json:"protocol_version"`
	SpectraVersion  string      `json:"spectra_version"`
	AgentVersion    string      `json:"agent_version"`
	Operations      []Operation `json:"operations"`
}

// Request is one typed diagnostic request. Params has the schema for
// Operation; it is never interpreted as an executable command or argument
// vector.
type Request struct {
	ProtocolVersion string          `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Operation       Operation       `json:"operation"`
	TimeoutMS       int             `json:"timeout_ms,omitempty"`
	Params          json.RawMessage `json:"params,omitempty"`
}

// Validate checks invariants common to every request. Operation-specific
// validation belongs to the target agent, which owns its local policy.
func (r Request) Validate() error {
	if r.ProtocolVersion != Version {
		return fmt.Errorf("protocol version %q is not supported", r.ProtocolVersion)
	}
	if strings.TrimSpace(r.RequestID) == "" {
		return fmt.Errorf("request id is required")
	}
	if r.Operation == "" {
		return fmt.Errorf("operation is required")
	}
	if r.TimeoutMS < 0 {
		return fmt.Errorf("timeout_ms must not be negative")
	}
	return nil
}

// Response is returned for every request. Error is a safe, operator-facing
// failure message; implementations must not put credentials in it.
type Response struct {
	ProtocolVersion string          `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           *Error          `json:"error,omitempty"`
}

// Error is a stable failure shape for callers.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
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
