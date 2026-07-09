package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMessagesNative(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req MessagesRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		if req.TopK == nil || *req.TopK != 40 {
			t.Errorf("top_k = %v", req.TopK)
		}
		if req.Thinking == nil || req.Thinking.BudgetTokens != 1024 {
			t.Errorf("thinking = %+v", req.Thinking)
		}
		if req.Metadata == nil || req.Metadata.UserID != "u1" {
			t.Errorf("metadata = %+v", req.Metadata)
		}
		// cache_control must be present on the first content block.
		if !strings.Contains(string(body), `"cache_control":{"type":"ephemeral"}`) {
			t.Errorf("cache_control missing: %s", body)
		}
		io.WriteString(w, `{"id":"1","model":"m","content":[{"type":"text","text":"hi"}],`+
			`"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2,`+
			`"cache_read_input_tokens":3}}`)
	})
	defer done()

	topK := 40
	resp, err := c.Messages(context.Background(), &MessagesRequest{
		Model:     "m",
		MaxTokens: 100,
		TopK:      &topK,
		Thinking:  &Thinking{Type: "enabled", BudgetTokens: 1024},
		Metadata:  &Metadata{UserID: "u1"},
		Messages: []MessageParam{{
			Role: "user",
			Content: []ContentBlock{{
				Type:         "text",
				Text:         "hi",
				CacheControl: &CacheControl{Type: "ephemeral"},
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "hi" {
		t.Errorf("content = %+v", resp.Content)
	}
	if resp.Usage.CacheReadInputTokens != 3 {
		t.Errorf("cache read tokens = %d", resp.Usage.CacheReadInputTokens)
	}
}

func TestMessagesNativeError(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"bad"}}`)
	})
	defer done()

	_, err := c.Messages(context.Background(), &MessagesRequest{
		Model: "m", MaxTokens: 1, Messages: []MessageParam{{Role: "user"}},
	})
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("err = %v", err)
	}
}

func TestMessagesStreamNative(t *testing.T) {
	events := []string{
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"a"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"b"}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var text strings.Builder
	var stopSeen bool
	for ev, err := range c.MessagesStream(context.Background(), &MessagesRequest{
		Model: "m", MaxTokens: 10, Messages: []MessageParam{{Role: "user"}},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		if ev.Type == "content_block_delta" && ev.Delta != nil {
			text.WriteString(ev.Delta.Text)
		}
		if ev.Type == "message_stop" {
			stopSeen = true
		}
	}
	if text.String() != "ab" || !stopSeen {
		t.Errorf("text = %q stop = %v", text.String(), stopSeen)
	}
}
