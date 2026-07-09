package anthropic

import (
	"errors"
	"testing"

	"github.com/goloop/ai"
)

// FuzzParseError checks that error parsing never panics on arbitrary response
// bodies and always yields an *ai.APIError that preserves the status and the
// raw body.
func FuzzParseError(f *testing.F) {
	for _, s := range []string{
		`{"error":{"type":"overloaded_error","message":"busy"}}`,
		`{"type":"error"}`,
		`not json`,
		``,
		`{"error":null}`,
		`[]`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		err := parseError(500, []byte(body))
		var apiErr *ai.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("not an *ai.APIError: %T", err)
		}
		if apiErr.Status != 500 {
			t.Fatalf("status = %d", apiErr.Status)
		}
		if string(apiErr.Raw) != body {
			t.Fatalf("raw body not preserved: %q != %q", apiErr.Raw, body)
		}
	})
}
