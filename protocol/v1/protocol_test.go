package v1

import "testing"

func TestRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want bool
	}{
		{"valid", Request{ProtocolVersion: Version, RequestID: "req-1", Operation: OperationHealth}, false},
		{"version", Request{ProtocolVersion: "v2", RequestID: "req-1", Operation: OperationHealth}, true},
		{"request id", Request{ProtocolVersion: Version, Operation: OperationHealth}, true},
		{"operation", Request{ProtocolVersion: Version, RequestID: "req-1"}, true},
		{"negative timeout", Request{ProtocolVersion: Version, RequestID: "req-1", Operation: OperationHealth, TimeoutMS: -1}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if (err != nil) != tc.want {
				t.Fatalf("Validate() error = %v, want error: %t", err, tc.want)
			}
		})
	}
}
