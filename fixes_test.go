package anthropic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/goloop/ai"
)

// BUG-01: a mid-stream error event must carry a meaningful status, not the
// 200 of the still-open HTTP response.
func TestStreamErrorEventStatus(t *testing.T) {
	events := []string{
		`event: error`,
		`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
		``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var gotErr error
	for _, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			gotErr = err
			break
		}
	}
	var apiErr *ai.APIError
	if !errors.As(gotErr, &apiErr) {
		t.Fatalf("err = %v", gotErr)
	}
	if apiErr.Status != 529 {
		t.Errorf("status = %d, want 529 (overloaded)", apiErr.Status)
	}
	if apiErr.Message != "Overloaded" {
		t.Errorf("message = %q", apiErr.Message)
	}
}

// BUG-02: a stream truncated before message_stop must surface an error, not
// end silently.
func TestStreamTruncated(t *testing.T) {
	events := []string{
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}`,
		``,
		// connection ends here - no message_stop
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var gotErr error
	var doneSeen bool
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			gotErr = err
			break
		}
		if chunk.Done {
			doneSeen = true
		}
	}
	if doneSeen {
		t.Error("truncated stream should not report Done")
	}
	if !errors.Is(gotErr, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want ErrUnexpectedEOF", gotErr)
	}
}

func TestStreamErrorStatusMapping(t *testing.T) {
	cases := map[string]int{
		"overloaded_error":      529,
		"rate_limit_error":      http.StatusTooManyRequests,
		"invalid_request_error": http.StatusBadRequest,
		"authentication_error":  http.StatusUnauthorized,
		"unknown_thing":         0,
	}
	for typ, want := range cases {
		if got := streamErrorStatus(typ); got != want {
			t.Errorf("streamErrorStatus(%q) = %d, want %d", typ, got, want)
		}
	}
}
