package v1

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrorCode is a stable wire failure code.
type ErrorCode string

const (
	CodeInvalidRequest             ErrorCode = "invalid_request"
	CodeUnsupportedProtocolVersion ErrorCode = "unsupported_protocol_version"
	CodeUnsupportedOperation       ErrorCode = "unsupported_operation"
	CodeMessageTooLarge            ErrorCode = "message_too_large"
	CodePermissionDenied           ErrorCode = "permission_denied"
	CodeUnavailable                ErrorCode = "unavailable"
	CodeIncompatibleSpectra        ErrorCode = "incompatible_spectra"
	CodeTimeout                    ErrorCode = "timeout"
	CodeExecutionFailed            ErrorCode = "execution_failed"
	CodeInternal                   ErrorCode = "internal"
)

// Known reports whether the code is defined by this version.
func (c ErrorCode) Known() bool {
	switch c {
	case CodeInvalidRequest, CodeUnsupportedProtocolVersion, CodeUnsupportedOperation, CodeMessageTooLarge, CodePermissionDenied, CodeUnavailable, CodeIncompatibleSpectra, CodeTimeout, CodeExecutionFailed, CodeInternal:
		return true
	}
	return false
}

// WellFormed accepts future codes using the stable wire-code grammar.
func (c ErrorCode) WellFormed() bool {
	s := string(c)
	if len(s) == 0 || len(s) > 64 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		b := s[i]
		if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_') {
			return false
		}
	}
	return true
}

// Validate checks the wire error code and operator-facing message.
func (e Error) Validate() error {
	if !e.Code.WellFormed() {
		return fmt.Errorf("invalid error code %q", e.Code)
	}
	if len(e.Message) == 0 || len(e.Message) > MaxErrorMessageBytes {
		return fmt.Errorf("invalid error message length")
	}
	return nil
}

// NewError builds an error with a UTF-8-safe bounded message.
func NewError(code ErrorCode, message string) *Error {
	message = strings.ToValidUTF8(message, "\ufffd")
	if len(message) > MaxErrorMessageBytes {
		message = message[:MaxErrorMessageBytes]
		for !utf8.ValidString(message) {
			message = message[:len(message)-1]
		}
	}
	return &Error{Code: code, Message: message}
}
