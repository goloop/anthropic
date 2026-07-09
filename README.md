[![deps.dev](https://img.shields.io/badge/deps.dev-insights-4c8dbc)](https://deps.dev/go/github.com%2Fgoloop%2Fanthropic) [![License](https://img.shields.io/badge/license-MIT-brightgreen)](https://github.com/goloop/anthropic/blob/main/LICENSE) [![License](https://img.shields.io/badge/godoc-YES-green)](https://pkg.go.dev/github.com/goloop/anthropic) [![Stay with Ukraine](https://img.shields.io/static/v1?label=Stay%20with&message=Ukraine%20♥&color=ffD700&labelColor=0057B8&style=flat)](https://u24.gov.ua/)


# anthropic

`anthropic` is a Go client for the Anthropic (Claude) API. It implements the
`github.com/goloop/ai` interface, so it looks and works like every other goloop
AI provider, and adds Anthropic's native endpoints on top.

## Features

- Messages API: `Generate` for a single response, `Stream` for token-by-token
  output through `iter.Seq2`.
- Tool use (function calling), multimodal image input and system prompts.
- Native endpoints: token counting, model listing and the message batches API.
- Retries on 429 and 5xx with backoff; normalized, typed API errors.
- Depends only on `github.com/goloop/ai` and the standard library.

## Installation

```sh
go get github.com/goloop/anthropic
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/goloop/ai"
	"github.com/goloop/anthropic"
)

func main() {
	c := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:     anthropic.ModelClaudeSonnet5,
		MaxTokens: 256,
		Messages:  []ai.Message{ai.UserText("Say hello in one word.")},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Text())
}
```

## Streaming

`Stream` returns an iterator; range over it and stop whenever you like.

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		break
	}
	fmt.Print(chunk.Text)
	if chunk.Done && chunk.Usage != nil {
		fmt.Printf("\n[%d in / %d out]\n",
			chunk.Usage.InputTokens, chunk.Usage.OutputTokens)
	}
}
```

## Tools (function calling)

```go
req := &ai.Request{
	Model:     anthropic.ModelClaudeSonnet5,
	MaxTokens: 512,
	Messages:  []ai.Message{ai.UserText("What is the weather in Kyiv?")},
	Tools: []ai.Tool{{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		Schema: json.RawMessage(
			`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`,
		),
	}},
}

resp, _ := c.Generate(ctx, req)
for _, call := range resp.ToolCalls() {
	// run the tool for call.Name / call.Input, then send the result back as a
	// RoleTool message containing an ai.ToolResult with the same call.ID.
}
```

## Images

```go
img, _ := os.ReadFile("chart.png")
req := &ai.Request{
	Model:     anthropic.ModelClaudeSonnet5,
	MaxTokens: 512,
	Messages: []ai.Message{{
		Role: ai.RoleUser,
		Parts: []ai.Part{
			ai.Text{Text: "What does this chart show?"},
			ai.Image{MIME: "image/png", Data: img},
		},
	}},
}
```

## Native Messages API

For Anthropic-only options build a `MessagesRequest` and call `Messages` or
`MessagesStream`. This reaches settings the shared `ai.Request` does not model -
`TopK`, extended `Thinking`, `Metadata` and prompt caching via `CacheControl`:

```go
topK := 40
resp, _ := c.Messages(ctx, &anthropic.MessagesRequest{
	Model:     anthropic.ModelClaudeSonnet5,
	MaxTokens: 1024,
	TopK:      &topK,
	Thinking:  &anthropic.Thinking{Type: "enabled", BudgetTokens: 2048},
	Messages: []anthropic.MessageParam{{
		Role: "user",
		Content: []anthropic.ContentBlock{{
			Type:         "text",
			Text:         longSystemDoc,
			CacheControl: &anthropic.CacheControl{Type: "ephemeral"},
		}},
	}},
})
```

## The API at a glance

- `New(apiKey string, opts ...Option) *Client`
- `Generate(ctx, *ai.Request) (*ai.Response, error)`
- `Stream(ctx, *ai.Request) iter.Seq2[ai.Chunk, error]`
- `Messages(ctx, *MessagesRequest) (*MessagesResponse, error)`,
  `MessagesStream(ctx, *MessagesRequest) iter.Seq2[StreamEvent, error]`
- `CountTokens(ctx, *ai.Request) (int, error)`
- `Models(ctx) ([]Model, error)`, `GetModel(ctx, id) (*Model, error)`
- `CreateBatch`, `GetBatch`, `ListBatches`, `CancelBatch`, `BatchResults`
- Options: `WithBaseURL`, `WithHTTPClient`, `WithTimeout`, `WithMaxRetries`,
  `WithHeader`, `WithVersion`, `WithBeta`, `WithMaxTokens`

## Documentation

Full reference: **[DOC.md](DOC.md)** (Ukrainian: **[DOC.UK.md](DOC.UK.md)**).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT - see [LICENSE](LICENSE).
