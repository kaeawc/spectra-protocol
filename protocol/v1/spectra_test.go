package v1

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeSpectraCapabilities(t *testing.T) {
	data := []byte(`{"schema":{"name":"spectra.capabilities","version":1},"spectra_version":"v1","os":"linux","arch":"amd64","interfaces":[{"name":"capabilities","argv":["capabilities","--json"],"output":"json","result_schema":{"name":"spectra.capabilities","version":1}}]}`)
	decoded, err := DecodeSpectraCapabilities(append(data, []byte(" \n")...))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decoded.ResultSchemaFor(OperationHealth); CodeOf(err) != CodeUnsupportedOperation {
		t.Fatalf("health error = %v", err)
	}
	if _, err := DecodeSpectraCapabilities(append(data, []byte("garbage")...)); CodeOf(err) != CodeIncompatibleSpectra {
		t.Fatalf("trailing data error = %v", err)
	}
	oversized := make([]byte, MaxCapabilitiesBytes+1)
	_, err = DecodeSpectraCapabilities(oversized)
	if CodeOf(err) != CodeIncompatibleSpectra || !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("oversize error = %v", err)
	}
	if strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("oversized document decoded instead of enforcing size limit: %v", err)
	}
}

func TestSpectraCapabilitiesValidate(t *testing.T) {
	capabilities := SpectraCapabilities{
		Schema: SchemaRef{Name: SchemaCapabilities, Version: 1}, SpectraVersion: "v1", OS: "linux", Arch: "amd64",
		Interfaces: []SpectraInterface{{Name: InterfaceCapabilities, Argv: []string{"capabilities", "--json"}, Output: OutputJSON, ResultSchema: &SchemaRef{Name: SchemaCapabilities, Version: 1}}},
	}
	if err := capabilities.Validate(); err != nil {
		t.Fatal(err)
	}
	capabilities.Interfaces[0].Name = "bad/name"
	if err := capabilities.Validate(); CodeOf(err) != CodeIncompatibleSpectra {
		t.Fatalf("validation error = %v", err)
	}
}
