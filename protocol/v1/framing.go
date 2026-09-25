package v1

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrMessageTooLarge indicates that a frame exceeds its configured byte limit.
var ErrMessageTooLarge = errors.New("message too large")

// ReadMessage reads one bounded NDJSON line, accepting a final unterminated line.
func ReadMessage(r *bufio.Reader, limit int) ([]byte, error) {
	if limit < 0 {
		return nil, fmt.Errorf("negative message limit")
	}
	line := make([]byte, 0, min(limit+1, 4096))
	for {
		fragment, err := r.ReadSlice('\n')
		hasNewline := len(fragment) > 0 && fragment[len(fragment)-1] == '\n'
		content := fragment
		if hasNewline {
			content = fragment[:len(fragment)-1]
		}
		if len(content) > limit-len(line) {
			return nil, ErrMessageTooLarge
		}
		line = append(line, content...)
		if hasNewline {
			return line, nil
		}
		if err == io.EOF {
			if len(line) == 0 {
				return nil, io.EOF
			}
			return line, nil
		}
		if err != nil && err != bufio.ErrBufferFull {
			return nil, fmt.Errorf("read message: %w", err)
		}
	}
}

// DecodeRequest decodes and validates a request while ignoring unknown envelope fields.
func DecodeRequest(line []byte) (Request, error) {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return Request{}, fmt.Errorf("decode request: %w", err)
	}
	if err := req.Validate(); err != nil {
		return Request{}, fmt.Errorf("validate request: %w", err)
	}
	return req, nil
}

// DecodeResponse decodes and validates a response while ignoring unknown envelope fields.
func DecodeResponse(line []byte) (Response, error) {
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}
	if err := resp.Validate(); err != nil {
		return Response{}, fmt.Errorf("validate response: %w", err)
	}
	return resp, nil
}

// WriteMessage writes one size-checked JSON value followed by one newline in one Write call.
func WriteMessage(w io.Writer, v any, limit int) error {
	if limit < 0 {
		return fmt.Errorf("negative message limit")
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	if len(encoded) > limit {
		return ErrMessageTooLarge
	}
	encoded = append(encoded, '\n')
	n, err := w.Write(encoded)
	if err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	if n != len(encoded) {
		return io.ErrShortWrite
	}
	return nil
}
