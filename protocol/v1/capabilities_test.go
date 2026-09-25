package v1

import (
	"errors"
	"fmt"
	"testing"
)

func testManifest() CapabilityManifest {
	return CapabilityManifest{ProtocolVersions: []string{Version}, Operations: []OperationCapability{{Name: OperationInspect, ResultSchema: &SchemaRef{SchemaInspect, 1}}}, Limits: Limits{MaxRequestBytes, MaxResponseBytes, MaxTimeoutMS}}
}
func TestNegotiate(t *testing.T) {
	m := testManifest()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := Negotiate(m, OperationInspect); err != nil {
		t.Fatal(err)
	}
	if _, err := Negotiate(m, OperationHealth); CodeOf(err) != CodeUnsupportedOperation {
		t.Fatal(err)
	}
	m.ProtocolVersions = []string{"v2"}
	if _, err := Negotiate(m, OperationInspect); CodeOf(fmt.Errorf("wrapped: %w", err)) != CodeUnsupportedProtocolVersion {
		t.Fatal(err)
	}
	m = testManifest()
	m.Operations[0].ResultSchema.Version = 2
	if _, err := Negotiate(m, OperationInspect); CodeOf(err) != CodeIncompatibleSpectra {
		t.Fatal(err)
	}
	if CodeOf(errors.New("plain")) != CodeInternal {
		t.Fatal("fallback")
	}
}
