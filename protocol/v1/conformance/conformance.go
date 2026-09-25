// Package conformance embeds wire-contract cases for Go and downstream implementations.
package conformance

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/kaeawc/spectra-protocol/protocol/v1"
)

// Case describes one expected wire-contract outcome.
type Case struct {
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	Input     json.RawMessage `json:"input"`
	Request   json.RawMessage `json:"request,omitempty"`
	Operation v1.Operation    `json:"operation,omitempty"`
	Valid     bool            `json:"valid"`
	ErrorCode v1.ErrorCode    `json:"error_code,omitempty"`
}

//go:embed testdata/*.json
var fixtures embed.FS

var files = []string{"requests.json", "responses.json", "manifests.json", "results.json", "spectra_capabilities.json"}

// Cases loads all embedded conformance fixtures in stable file order.
func Cases() ([]Case, error) {
	var all []Case
	for _, file := range files {
		raw, err := fixtures.ReadFile("testdata/" + file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		var cases []Case
		if err := json.Unmarshal(raw, &cases); err != nil {
			return nil, fmt.Errorf("decode %s: %w", file, err)
		}
		all = append(all, cases...)
	}
	return all, nil
}

// CasesOf returns embedded fixtures for one kind.
func CasesOf(kind string) ([]Case, error) {
	all, err := Cases()
	if err != nil {
		return nil, err
	}
	var selected []Case
	for _, c := range all {
		if c.Kind == kind {
			selected = append(selected, c)
		}
	}
	return selected, nil
}
