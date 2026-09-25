package conformance

import (
	"encoding/json"
	protocol "github.com/kaeawc/spectra-protocol/protocol/v1"
	"testing"
)

func TestCases(t *testing.T) {
	cases, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 30 {
		t.Fatalf("only %d cases", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var got error
			switch tc.Kind {
			case "request":
				var req protocol.Request
				if got = json.Unmarshal(tc.Input, &req); got == nil {
					got = req.Validate()
				}
				if tc.ErrorCode != "" {
					if tc.Valid {
						if got != nil {
							t.Fatalf("valid envelope: %v", got)
						}
						if tc.ErrorCode != protocol.CodeUnsupportedOperation {
							t.Fatalf("unexpected downstream code: %s", tc.ErrorCode)
						}
					} else if code := protocol.RequestErrorCode(got); code != tc.ErrorCode {
						t.Fatalf("code %s, want %s", code, tc.ErrorCode)
					}
				}
			case "response":
				var req protocol.Request
				if got = json.Unmarshal(tc.Request, &req); got != nil {
					break
				}
				if got = req.Validate(); got != nil {
					break
				}
				var resp protocol.Response
				if got = json.Unmarshal(tc.Input, &resp); got == nil {
					got = resp.ValidateFor(req)
				}
			case "manifest":
				var m protocol.CapabilityManifest
				if got = json.Unmarshal(tc.Input, &m); got != nil {
					break
				}
				if got = m.Validate(); got != nil {
					break
				}
				hasV1 := false
				for _, v := range m.ProtocolVersions {
					if v == protocol.Version {
						hasV1 = true
					}
				}
				if !hasV1 {
					_, got = protocol.Negotiate(m, protocol.OperationInspect)
				}
			case "result":
				var resp protocol.Response
				if got = json.Unmarshal(tc.Input, &resp); got == nil {
					_, got = protocol.DecodeResult(resp, tc.Operation)
				}
				if !tc.Valid && tc.ErrorCode != "" && protocol.CodeOf(got) != tc.ErrorCode {
					t.Fatalf("code %s, want %s: %v", protocol.CodeOf(got), tc.ErrorCode, got)
				}
			default:
				t.Fatalf("unknown kind %q", tc.Kind)
			}
			if (got == nil) != tc.Valid {
				t.Fatalf("valid=%v, got %v", tc.Valid, got)
			}
		})
	}
	for _, kind := range []string{"request", "response", "manifest", "result"} {
		selected, err := CasesOf(kind)
		if err != nil || len(selected) == 0 {
			t.Fatalf("CasesOf(%q): %d, %v", kind, len(selected), err)
		}
	}
}
