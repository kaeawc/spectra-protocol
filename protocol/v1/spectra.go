package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

const (
	SchemaCapabilities        = "spectra.capabilities"
	CapabilitiesSchemaVersion = 1
	MaxCapabilitiesBytes      = 1 << 20
)

const (
	InterfaceVersion      = "version"
	InterfaceInspect      = "inspect"
	InterfaceSnapshot     = "snapshot"
	InterfaceCapabilities = "capabilities"
)

const (
	OutputText = "text"
	OutputJSON = "json"
)

// SpectraInterface describes one command exposed by the Spectra CLI.
type SpectraInterface struct {
	Name         string     `json:"name"`
	Argv         []string   `json:"argv"`
	Output       string     `json:"output"`
	ResultSchema *SchemaRef `json:"result_schema,omitempty"`
}

// SpectraCapabilities is the JSON document emitted by `spectra capabilities --json`.
type SpectraCapabilities struct {
	Schema         SchemaRef          `json:"schema"`
	SpectraVersion string             `json:"spectra_version"`
	OS             string             `json:"os"`
	Arch           string             `json:"arch"`
	Interfaces     []SpectraInterface `json:"interfaces"`
}

var interfaceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

// Validate checks whether this document conforms to the Spectra capabilities contract.
// An unparseable or nonconforming document is, by definition, an incompatible Spectra;
// every validation failure therefore uses CodeIncompatibleSpectra, not CodeInvalidRequest.
func (c SpectraCapabilities) Validate() error {
	fail := func(message string) error {
		return &CodedError{Code: CodeIncompatibleSpectra, Err: fmt.Errorf("%s", message)}
	}
	if c.Schema.Name != SchemaCapabilities {
		return fail(fmt.Sprintf("schema name %q, want %q", c.Schema.Name, SchemaCapabilities))
	}
	if c.Schema.Version != CapabilitiesSchemaVersion {
		return fail(fmt.Sprintf("schema version %d, want %d", c.Schema.Version, CapabilitiesSchemaVersion))
	}
	if c.SpectraVersion == "" || c.OS == "" || c.Arch == "" {
		return fail("spectra_version, os, and arch are required")
	}
	if len(c.Interfaces) == 0 {
		return fail("at least one interface is required")
	}
	seen := make(map[string]bool, len(c.Interfaces))
	capabilitiesCount := 0
	for _, iface := range c.Interfaces {
		if !interfaceNamePattern.MatchString(iface.Name) {
			return fail(fmt.Sprintf("invalid interface name %q", iface.Name))
		}
		if seen[iface.Name] {
			return fail(fmt.Sprintf("duplicate interface %q", iface.Name))
		}
		seen[iface.Name] = true
		if iface.Name == InterfaceCapabilities {
			capabilitiesCount++
		}
		switch iface.Output {
		case OutputJSON:
			if iface.ResultSchema == nil || iface.ResultSchema.Name == "" || iface.ResultSchema.Version < 1 {
				return fail(fmt.Sprintf("JSON interface %q requires a named result schema with positive version", iface.Name))
			}
		case OutputText:
			if iface.ResultSchema != nil {
				return fail(fmt.Sprintf("text interface %q must not declare a result schema", iface.Name))
			}
		default:
			return fail(fmt.Sprintf("invalid output %q for interface %q", iface.Output, iface.Name))
		}
	}
	capability, ok := c.Interface(InterfaceCapabilities)
	if capabilitiesCount != 1 || !ok || capability.Output != OutputJSON || capability.ResultSchema == nil || *capability.ResultSchema != (SchemaRef{Name: SchemaCapabilities, Version: CapabilitiesSchemaVersion}) {
		return fail("capabilities interface must emit spectra.capabilities version 1 JSON")
	}
	return nil
}

// Interface returns the named interface when present.
func (c SpectraCapabilities) Interface(name string) (SpectraInterface, bool) {
	for _, iface := range c.Interfaces {
		if iface.Name == name {
			return iface, true
		}
	}
	return SpectraInterface{}, false
}

// InterfaceForOperation maps supported protocol operations to CLI interfaces.
func InterfaceForOperation(op Operation) (string, bool) {
	switch op {
	case OperationInspect:
		return InterfaceInspect, true
	case OperationSnapshotCreate:
		return InterfaceSnapshot, true
	default:
		return "", false
	}
}

// ResultSchemaFor checks that the CLI interface exposes the result schema expected by op.
func (c SpectraCapabilities) ResultSchemaFor(op Operation) (SchemaRef, error) {
	name, ok := InterfaceForOperation(op)
	if !ok {
		return SchemaRef{}, &CodedError{Code: CodeUnsupportedOperation, Err: fmt.Errorf("no Spectra interface for operation %q", op)}
	}
	iface, found := c.Interface(name)
	expectedName, _ := ResultSchemaName(op)
	expectedVersion, _ := SupportedResultSchemaVersion(expectedName)
	if !found || iface.ResultSchema == nil || iface.ResultSchema.Name != expectedName || iface.ResultSchema.Version != expectedVersion {
		foundSchema := "<missing>"
		if found && iface.ResultSchema != nil {
			foundSchema = fmt.Sprintf("%s/%d", iface.ResultSchema.Name, iface.ResultSchema.Version)
		}
		return SchemaRef{}, &CodedError{Code: CodeIncompatibleSpectra, Err: fmt.Errorf("interface %q result schema %s, want %s/%d", name, foundSchema, expectedName, expectedVersion)}
	}
	return *iface.ResultSchema, nil
}

// DecodeSpectraCapabilities decodes, bounds, and validates a capabilities document.
func DecodeSpectraCapabilities(data []byte) (SpectraCapabilities, error) {
	if len(data) > MaxCapabilitiesBytes {
		return SpectraCapabilities{}, &CodedError{Code: CodeIncompatibleSpectra, Err: fmt.Errorf("capabilities document: %w", ErrMessageTooLarge)}
	}
	var decoded SpectraCapabilities
	if err := json.Unmarshal(data, &decoded); err != nil {
		return SpectraCapabilities{}, &CodedError{Code: CodeIncompatibleSpectra, Err: fmt.Errorf("decode capabilities: %w", err)}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return SpectraCapabilities{}, &CodedError{Code: CodeIncompatibleSpectra, Err: fmt.Errorf("decode capabilities: %w", err)}
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("trailing data after capabilities document")
		}
		return SpectraCapabilities{}, &CodedError{Code: CodeIncompatibleSpectra, Err: fmt.Errorf("decode capabilities: %w", err)}
	}
	if err := decoded.Validate(); err != nil {
		return SpectraCapabilities{}, err
	}
	return decoded, nil
}
