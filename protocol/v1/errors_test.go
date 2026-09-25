package v1

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestErrorCodesAndTruncation(t *testing.T) {
	if !CodeInternal.Known() || !ErrorCode("future_code").WellFormed() || ErrorCode("future_code").Known() || ErrorCode("Bad-Code").WellFormed() {
		t.Fatal("code classification")
	}
	e := NewError(CodeInternal, strings.Repeat("a", MaxErrorMessageBytes-1)+"é")
	if len(e.Message) > MaxErrorMessageBytes || !utf8.ValidString(e.Message) || e.Validate() != nil {
		t.Fatalf("invalid bounded error: %+v", e)
	}
	if (Error{Code: CodeInternal, Message: ""}).Validate() == nil {
		t.Fatal("accepted empty message")
	}
}
