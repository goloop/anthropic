//go:build integration

// Integration smoke tests hit the live Anthropic API. They are excluded from
// the normal build and run only with the "integration" tag and a real key:
//
//	ANTHROPIC_API_KEY=... go test -tags integration -run Integration ./...
package anthropic_test

import (
	"cmp"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/goloop/ai"
	"github.com/goloop/anthropic"
)

// integrationModel is the model the smoke tests use. Override it with the
// ANTHROPIC_MODEL env var; the default is a current, inexpensive model so the
// tests do not break when older model names are retired.
var integrationModel = cmp.Or(os.Getenv("ANTHROPIC_MODEL"), anthropic.ModelClaudeHaiku45)

func integrationClient(t *testing.T) *anthropic.Client {
	t.Helper()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("set ANTHROPIC_API_KEY to run integration tests")
	}
	return anthropic.New(key)
}

func TestIntegrationGenerate(t *testing.T) {
	c := integrationClient(t)
	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:     integrationModel,
		MaxTokens: 16,
		Messages:  []ai.Message{ai.UserText("Reply with exactly one word: pong")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() == "" {
		t.Fatal("empty text")
	}
	t.Logf("generate: %q (in=%d out=%d)", resp.Text(), resp.Usage.InputTokens, resp.Usage.OutputTokens)
}

func TestIntegrationStream(t *testing.T) {
	c := integrationClient(t)
	var text string
	var done bool
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model:     integrationModel,
		MaxTokens: 32,
		Messages:  []ai.Message{ai.UserText("Count from 1 to 5.")},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		text += chunk.Text
		if chunk.Done {
			done = true
		}
	}
	if text == "" || !done {
		t.Fatalf("text=%q done=%v", text, done)
	}
	t.Logf("stream: %q done=%v", text, done)
}

func TestIntegrationTools(t *testing.T) {
	c := integrationClient(t)
	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:     integrationModel,
		MaxTokens: 128,
		Messages:  []ai.Message{ai.UserText("What is the weather in Kyiv? Use the tool.")},
		Tools: []ai.Tool{{
			Name:        "get_weather",
			Description: "Get the current weather for a city.",
			Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() == "" && len(resp.ToolCalls()) == 0 {
		t.Fatal("neither text nor tool call")
	}
	t.Logf("tools: text=%q calls=%d", resp.Text(), len(resp.ToolCalls()))
}

func TestIntegrationMessagesNative(t *testing.T) {
	c := integrationClient(t)
	resp, err := c.Messages(context.Background(), &anthropic.MessagesRequest{
		Model:     integrationModel,
		MaxTokens: 16,
		Messages: []anthropic.MessageParam{{
			Role:    "user",
			Content: []anthropic.ContentBlock{{Type: "text", Text: "Reply with one word: pong"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Content) == 0 {
		t.Fatal("empty content")
	}
	t.Logf("messages: stop=%q (in=%d out=%d)", resp.StopReason, resp.Usage.InputTokens, resp.Usage.OutputTokens)
}

func TestIntegrationCountTokens(t *testing.T) {
	c := integrationClient(t)
	n, err := c.CountTokens(context.Background(), &ai.Request{
		Model:    integrationModel,
		Messages: []ai.Message{ai.UserText("How many tokens is this?")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n <= 0 {
		t.Fatalf("token count = %d", n)
	}
	t.Logf("count_tokens: %d", n)
}
