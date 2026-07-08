package anthropic_test

import (
	"encoding/json"
	"fmt"

	"github.com/goloop/ai"
	"github.com/goloop/anthropic"
)

func ExampleNew() {
	c := anthropic.New("sk-ant-...")
	_ = c // use c.Generate, c.Stream, ...
	fmt.Println(anthropic.ModelClaude37SonnetLatest)
	// Output: claude-3-7-sonnet-latest
}

// ExampleClient_Generate builds a request. Sending it needs a real API key, so
// this example only shows the shape.
func ExampleClient_Generate() {
	req := &ai.Request{
		Model:     anthropic.ModelClaude35HaikuLatest,
		MaxTokens: 128,
		Messages: []ai.Message{
			ai.UserText("Name the capital of France."),
		},
	}
	fmt.Println(req.Model, len(req.Messages))
	// Output: claude-3-5-haiku-latest 1
}

// ExampleTool shows a tool definition passed with a request.
func ExampleTool() {
	tool := ai.Tool{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
	}
	fmt.Println(tool.Name)
	// Output: get_weather
}
