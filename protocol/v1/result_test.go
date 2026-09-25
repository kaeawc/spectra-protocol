package v1

import (
	"encoding/json"
	"testing"
)

func TestDecodeHealthAndResult(t *testing.T) {
	manifest := testManifest()
	raw, err := json.Marshal(HealthResult{manifest})
	if err != nil {
		t.Fatal(err)
	}
	resp := Response{ProtocolVersion: Version, RequestID: "x", Result: raw}
	if _, err := DecodeHealth(resp); err != nil {
		t.Fatal(err)
	}
	resp.Result = []byte(`{"schema":{"name":"spectra.inspect","version":1},"spectra_version":"1.0","data":{}}`)
	if _, err := DecodeResult(resp, OperationInspect); err != nil {
		t.Fatal(err)
	}
	resp.Result = []byte(`{"schema":{"name":"spectra.inspect","version":1},"data":null}`)
	if _, err := DecodeResult(resp, OperationInspect); CodeOf(err) != CodeInvalidRequest {
		t.Fatal(err)
	}
}
