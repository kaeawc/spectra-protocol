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

func TestDecodeListResults(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
		raw  string
		code ErrorCode
	}{
		{"process list", OperationProcessList, `{"schema":{"name":"spectra.process.list","version":1},"spectra_version":"1.0","data":{"sampled_at":"2026-11-24T10:00:00.123Z","processes":[{"pid":1}]}}`, ""},
		{"process list without sampled_at", OperationProcessList, `{"schema":{"name":"spectra.process.list","version":1},"spectra_version":"1.0","data":{"processes":[]}}`, ""},
		{"jvm list", OperationJVMList, `{"schema":{"name":"spectra.jvm.list","version":1},"spectra_version":"1.0","data":{"jvms":[{"pid":42}]}}`, ""},
		{"process list missing array", OperationProcessList, `{"schema":{"name":"spectra.process.list","version":1},"spectra_version":"1.0","data":{}}`, CodeInvalidRequest},
		{"process list null array", OperationProcessList, `{"schema":{"name":"spectra.process.list","version":1},"spectra_version":"1.0","data":{"processes":null}}`, CodeInvalidRequest},
		{"process list bare array", OperationProcessList, `{"schema":{"name":"spectra.process.list","version":1},"spectra_version":"1.0","data":[]}`, CodeInvalidRequest},
		{"process list bad sampled_at", OperationProcessList, `{"schema":{"name":"spectra.process.list","version":1},"spectra_version":"1.0","data":{"sampled_at":"yesterday","processes":[]}}`, CodeInvalidRequest},
		{"jvm list object rows", OperationJVMList, `{"schema":{"name":"spectra.jvm.list","version":1},"spectra_version":"1.0","data":{"jvms":{}}}`, CodeInvalidRequest},
		{"jvm list wrong schema", OperationJVMList, `{"schema":{"name":"spectra.process.list","version":1},"spectra_version":"1.0","data":{"processes":[]}}`, CodeIncompatibleSpectra},
		{"process list future version", OperationProcessList, `{"schema":{"name":"spectra.process.list","version":2},"spectra_version":"1.0","data":{"processes":[]}}`, CodeIncompatibleSpectra},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeResult(Response{ProtocolVersion: Version, RequestID: "x", Result: []byte(tc.raw)}, tc.op)
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if CodeOf(err) != tc.code {
				t.Fatalf("code %s, want %s: %v", CodeOf(err), tc.code, err)
			}
		})
	}
}
