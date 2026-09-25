package v1

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type countingWriter struct {
	calls int
	data  []byte
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.calls++
	w.data = append(w.data, p...)
	return len(p), nil
}
func TestFraming(t *testing.T) {
	for _, tc := range []struct {
		input string
		limit int
		want  string
		err   error
	}{{"abc\n", 3, "abc", nil}, {"abc", 3, "abc", nil}, {"abc\n", 2, "", ErrMessageTooLarge}, {"", 2, "", io.EOF}} {
		got, err := ReadMessage(bufio.NewReaderSize(strings.NewReader(tc.input), 2), tc.limit)
		if !errors.Is(err, tc.err) || string(got) != tc.want {
			t.Fatalf("ReadMessage(%q): %q, %v", tc.input, got, err)
		}
	}
	w := &countingWriter{}
	if err := WriteMessage(w, map[string]int{"x": 1}, 7); err != nil || w.calls != 1 || !bytes.Equal(w.data, []byte("{\"x\":1}\n")) {
		t.Fatalf("write: %q %d %v", w.data, w.calls, err)
	}
	w = &countingWriter{}
	if !errors.Is(WriteMessage(w, map[string]int{"x": 1}, 6), ErrMessageTooLarge) || w.calls != 0 {
		t.Fatal("oversize write")
	}
	if _, err := DecodeRequest([]byte(`{"protocol_version":"v1","request_id":"x","operation":"inspect","future":1}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeResponse([]byte(`{"protocol_version":"v1","request_id":"x","result":{},"future":1}`)); err != nil {
		t.Fatal(err)
	}
}
