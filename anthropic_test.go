package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	c := New("key", WithBaseURL(srv.URL), WithMaxRetries(0))
	return c, srv.Close
}

func newTestClientRaw(
	t *testing.T, h http.HandlerFunc, opts ...Option,
) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	base := append([]Option{WithBaseURL(srv.URL), WithMaxRetries(0)}, opts...)
	return New("key", base...), srv.Close
}

func TestGenerateText(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "key" {
			t.Errorf("missing x-api-key")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing anthropic-version")
		}
		var req wireRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "m" {
			t.Errorf("model = %q", req.Model)
		}
		io.WriteString(w, `{"model":"m","content":[{"type":"text","text":"hello"}],`+
			`"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`)
	})
	defer done()

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:    "m",
		Messages: []ai.Message{ai.UserText("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "hello" {
		t.Errorf("text = %q", resp.Text())
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 2 {
		t.Errorf("usage = %+v", resp.Usage)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop = %q", resp.StopReason)
	}
}

func TestGenerateToolUse(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"model":"m","content":[{"type":"tool_use","id":"tu_1",`+
			`"name":"get_weather","input":{"city":"Kyiv"}}],"stop_reason":"tool_use",`+
			`"usage":{"input_tokens":10,"output_tokens":5}}`)
	})
	defer done()

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:    "m",
		Messages: []ai.Message{ai.UserText("weather?")},
		Tools:    []ai.Tool{{Name: "get_weather"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := resp.ToolCalls()
	if len(calls) != 1 || calls[0].Name != "get_weather" {
		t.Fatalf("calls = %+v", calls)
	}
	if string(calls[0].Input) != `{"city":"Kyiv"}` {
		t.Errorf("input = %s", calls[0].Input)
	}
}

func TestStream(t *testing.T) {
	events := []string{
		`data: {"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":0}}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}`,
		``,
		`data: {"type":"message_delta","usage":{"output_tokens":7}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var text strings.Builder
	var usage ai.Usage
	var doneSeen bool
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model:    "m",
		Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(chunk.Text)
		if chunk.Done {
			doneSeen = true
			if chunk.Usage != nil {
				usage = *chunk.Usage
			}
		}
	}
	if text.String() != "Hello" {
		t.Errorf("text = %q", text.String())
	}
	if !doneSeen {
		t.Errorf("no done chunk")
	}
	if usage.InputTokens != 5 || usage.OutputTokens != 7 {
		t.Errorf("usage = %+v", usage)
	}
}

func TestStreamToolCall(t *testing.T) {
	events := []string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_9","name":"lookup"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"42}"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var call *ai.ToolUse
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model:    "m",
		Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		if chunk.ToolCall != nil {
			call = chunk.ToolCall
		}
	}
	if call == nil || call.Name != "lookup" {
		t.Fatalf("call = %+v", call)
	}
	if string(call.Input) != `{"q":42}` {
		t.Errorf("input = %s", call.Input)
	}
}

func TestError(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`)
	})
	defer done()

	_, err := c.Generate(context.Background(), &ai.Request{
		Model:    "m",
		Messages: []ai.Message{ai.UserText("hi")},
	})
	var apiErr *ai.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *ai.APIError, got %v", err)
	}
	if apiErr.Status != http.StatusBadRequest || apiErr.Message != "bad" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestCountTokens(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/count_tokens") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"input_tokens":11}`)
	})
	defer done()

	n, err := c.CountTokens(context.Background(), &ai.Request{
		Model:    "m",
		Messages: []ai.Message{ai.UserText("count me")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 11 {
		t.Errorf("tokens = %d", n)
	}
}

func TestBuildRequestMapping(t *testing.T) {
	c := New("key")
	temp := 0.3
	wr, err := c.buildRequest(&ai.Request{
		Model:       "m",
		System:      "sys",
		Temperature: &temp,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Parts: []ai.Part{
				ai.Text{Text: "hi"},
				ai.Image{MIME: "image/png", Data: []byte{1, 2, 3}},
			}},
			{Role: ai.RoleTool, Parts: []ai.Part{
				ai.ToolResult{ID: "tu_1", Content: "42"},
			}},
		},
		Tools:      []ai.Tool{{Name: "t"}},
		ToolChoice: ai.ToolRequired,
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	if wr.System != "sys" {
		t.Errorf("system = %q", wr.System)
	}
	if wr.MaxTokens != DefaultMaxTokens {
		t.Errorf("max_tokens = %d", wr.MaxTokens)
	}
	if len(wr.Messages) != 2 {
		t.Fatalf("messages = %d", len(wr.Messages))
	}
	img := wr.Messages[0].Content[1]
	if img.Type != "image" || img.Source == nil || img.Source.Data != "AQID" {
		t.Errorf("image = %+v", img.Source)
	}
	if wr.Messages[1].Role != "user" || wr.Messages[1].Content[0].Type != "tool_result" {
		t.Errorf("tool_result = %+v", wr.Messages[1])
	}
	if wr.ToolChoice == nil || wr.ToolChoice.Type != "any" {
		t.Errorf("tool_choice = %+v", wr.ToolChoice)
	}
	if len(wr.Tools) != 1 || string(wr.Tools[0].InputSchema) != `{"type":"object"}` {
		t.Errorf("tools = %+v", wr.Tools)
	}
}

func TestValidate(t *testing.T) {
	c := New("key")
	_, err := c.Generate(context.Background(), &ai.Request{Model: "m"})
	if !errors.Is(err, ai.ErrNoMessages) {
		t.Errorf("want ErrNoMessages, got %v", err)
	}
}
