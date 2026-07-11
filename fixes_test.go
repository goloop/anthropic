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

// A tool call whose accumulated input_json_delta is not valid JSON must
// surface an error instead of yielding an unparseable json.RawMessage.
func TestStreamInvalidToolArgs(t *testing.T) {
	events := []string{
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"do_it"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{not json"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var gotErr error
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			gotErr = err
			break
		}
		if chunk.ToolCall != nil {
			t.Error("invalid tool args must not yield a tool call")
		}
	}
	if gotErr == nil {
		t.Fatal("want error for invalid tool-call JSON, got nil")
	}
}

// A malformed SSE JSON payload in the native stream must surface as an error
// rather than being silently skipped.
func TestMessagesStreamMalformedJSON(t *testing.T) {
	events := []string{
		`event: garbage`,
		`data: {not valid json`,
		``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var gotErr error
	for _, err := range c.MessagesStream(context.Background(), &MessagesRequest{
		Model: "m", MaxTokens: 10,
		Messages: []MessageParam{{Role: "user",
			Content: []ContentBlock{{Type: "text", Text: "hi"}}}},
	}) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("want error for malformed SSE JSON, got nil")
	}
}

// Public methods that take a request pointer must return an error, not panic,
// on a nil argument.
func TestNilRequestGuards(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be reached for a nil request")
	})
	defer done()

	if _, err := c.Messages(context.Background(), nil); !errors.Is(err, ai.ErrNoRequest) {
		t.Errorf("Messages(nil) err = %v, want ErrNoRequest", err)
	}
	var streamErr error
	for _, err := range c.MessagesStream(context.Background(), nil) {
		streamErr = err
		break
	}
	if !errors.Is(streamErr, ai.ErrNoRequest) {
		t.Errorf("MessagesStream(nil) err = %v, want ErrNoRequest", streamErr)
	}
	if _, err := c.BatchResults(context.Background(), nil); !errors.Is(err, ErrNoResults) {
		t.Errorf("BatchResults(nil) err = %v, want ErrNoResults", err)
	}
}

// A response body larger than the read ceiling must error rather than being
// silently truncated or exhausting memory.
func TestResponseBodyCapped(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "")
		buf := make([]byte, 1<<20)
		for i := 0; i <= maxResponseBytes/len(buf)+1; i++ {
			if _, err := w.Write(buf); err != nil {
				return
			}
		}
	})
	defer done()

	_, err := c.GetModel(context.Background(), "m")
	if err == nil {
		t.Fatal("want error for oversized response body, got nil")
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
