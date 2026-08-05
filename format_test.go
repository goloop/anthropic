package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

func askForFormat(system string, f *ai.Format) *ai.Request {
	return &ai.Request{
		Model:    "the-model",
		System:   system,
		Messages: []ai.Message{ai.UserText("Describe this article.")},
		Format:   f,
	}
}

// TestFormatIsEmulated covers the one driver with nothing native to map onto:
// the format is asked for in the system prompt, and the response says so, so a
// caller can tell an enforced answer from a requested one.
func TestFormatIsEmulated(t *testing.T) {
	c := &Client{maxTokens: 1024}

	wr, err := c.buildRequest(askForFormat("You are terse.",
		&ai.Format{Type: ai.FormatJSON}), false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(wr.System, "You are terse.") {
		t.Errorf("caller's system prompt was lost: %q", wr.System)
	}
	if !strings.Contains(wr.System, "single JSON value") {
		t.Errorf("system prompt does not carry the instruction: %q", wr.System)
	}
	if got := formatMode(&ai.Format{Type: ai.FormatJSON}); got != ai.FormatEmulated {
		t.Errorf("formatMode = %s, want emulated", got)
	}
}

// TestFormatSchemaReachesThePrompt checks a schema is spelled out for the
// model, since there is no field to put it in.
func TestFormatSchemaReachesThePrompt(t *testing.T) {
	schema := `{"type":"object","properties":{"a":{"type":"string"}}}`
	wr, err := (&Client{maxTokens: 1024}).buildRequest(askForFormat("",
		&ai.Format{Type: ai.FormatJSONSchema, Schema: json.RawMessage(schema)}), false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wr.System, schema) {
		t.Errorf("system prompt does not carry the schema: %q", wr.System)
	}
}

// TestNoFormatLeavesThePromptAlone is the boring half: a request that asks for
// nothing looks exactly as it did before.
func TestNoFormatLeavesThePromptAlone(t *testing.T) {
	c := &Client{maxTokens: 1024}
	for _, f := range []*ai.Format{nil, {Type: ai.FormatText}} {
		wr, err := c.buildRequest(askForFormat("You are terse.", f), false)
		if err != nil {
			t.Fatal(err)
		}
		if wr.System != "You are terse." {
			t.Errorf("system prompt was changed: %q", wr.System)
		}
		if got := formatMode(f); got != ai.FormatNone {
			t.Errorf("formatMode = %s, want none", got)
		}
	}
}
