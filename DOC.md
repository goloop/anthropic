# anthropic - reference

The full reference for the `anthropic` package: the client, the shared
`goloop/ai` request and response model, generation, streaming, tools, images,
and the native Anthropic endpoints.

Ukrainian version: **[DOC.UK.md](DOC.UK.md)**.

## Contents

- [Mental model](#mental-model)
- [Creating a client](#creating-a-client)
- [Generate](#generate)
- [Stream](#stream)
- [Tools](#tools)
- [Images](#images)
- [Token counting](#token-counting)
- [Models](#models)
- [Message batches](#message-batches)
- [Options](#options)
- [Errors](#errors)

## Mental model

`anthropic.Client` implements `ai.Client`, the provider-agnostic contract from
`github.com/goloop/ai`. The same `ai.Request`, `ai.Response`, `ai.Message` and
`ai.Tool` types are used across every goloop AI provider, so switching provider
means switching the constructor, not the call sites.

The endpoints that Anthropic does not share with other providers (token
counting, models, batches) are native methods on `anthropic.Client` and are not
part of the shared interface.

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/anthropic"
)
```

## Creating a client

```go
c := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))

c = anthropic.New(apiKey,
	anthropic.WithMaxTokens(1024),
	anthropic.WithTimeout(30*time.Second),
)
```

`New` needs an API key; everything else has a default (base URL
`https://api.anthropic.com`, `anthropic-version` `2023-06-01`, `max_tokens`
`1024`, a 60s timeout and two retries).

## Generate

`Generate` sends one request and returns the whole response.

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:     anthropic.ModelClaudeSonnet5,
	MaxTokens: 256,
	System:    "You are concise.",
	Messages: []ai.Message{
		ai.UserText("Name three primary colors."),
	},
})

resp.Text()       // all text parts joined
resp.ToolCalls()  // any tool calls the model made
resp.Usage        // input and output tokens
resp.StopReason   // "end_turn", "max_tokens", "tool_use", ...
```

A `Message` is a `Role` and a list of content `Part` values (`ai.Text`,
`ai.Image`, `ai.ToolUse`, `ai.ToolResult`). `ai.UserText` and
`ai.AssistantText` build single-text messages. A `RoleSystem` message is folded
into the top-level system prompt.

## Stream

`Stream` returns `iter.Seq2[ai.Chunk, error]`. Text arrives as chunks with
`Text` set; a finished tool call arrives as a chunk with `ToolCall` set; the
final chunk has `Done` true and carries `Usage`.

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		return err
	}
	fmt.Print(chunk.Text)
	if chunk.Done {
		total := chunk.Usage
		_ = total
	}
}
```

Stop early simply by breaking out of the range; the underlying response is
closed for you.

## Tools

Declare tools with a JSON Schema for their input. The model may answer with tool
calls instead of, or alongside, text.

```go
req.Tools = []ai.Tool{{
	Name:        "get_weather",
	Description: "Get the current weather for a city.",
	Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
}}
req.ToolChoice = ai.ToolAuto // ToolAuto, ToolNone or ToolRequired
```

Handle the call, then send the result back as a `RoleTool` message whose
`ai.ToolResult.ID` matches the `ai.ToolUse.ID`:

```go
call := resp.ToolCalls()[0]
result := runTool(call) // your code
req.Messages = append(req.Messages,
	ai.Message{Role: ai.RoleAssistant, Parts: []ai.Part{call}},
	ai.Message{Role: ai.RoleTool, Parts: []ai.Part{
		ai.ToolResult{ID: call.ID, Content: result},
	}},
)
```

## Images

Provide inline bytes with a MIME type, or a URL:

```go
ai.Image{MIME: "image/png", Data: pngBytes}
ai.Image{URL: "https://example.com/photo.jpg"}
```

Images are content parts inside a user message, next to text parts.

## Token counting

```go
n, err := c.CountTokens(ctx, req)
```

Returns the input-token count for a request without generating anything.

## Models

```go
models, err := c.Models(ctx)
m, err := c.GetModel(ctx, "claude-sonnet-5")
```

`Model` reports `ID`, `DisplayName`, `CreatedAt` and `Type`.

## Message batches

Submit many requests for asynchronous processing at a lower cost.

```go
batch, err := c.CreateBatch(ctx, []anthropic.BatchItem{
	{CustomID: "a", Request: reqA},
	{CustomID: "b", Request: reqB},
})

batch, err = c.GetBatch(ctx, batch.ID)      // poll ProcessingStatus
results, err := c.BatchResults(ctx, batch)  // once ended

batches, err := c.ListBatches(ctx)
batch, err = c.CancelBatch(ctx, batch.ID)
```

`BatchResults` returns one `BatchResult` per request, correlated by
`CustomID`, with the raw result JSON. It returns `ErrNoResults` if the batch has
not finished.

## Options

The shared options are the same across every goloop AI provider:

- `WithBaseURL(url)` - override the API base URL.
- `WithHTTPClient(client)` - use your own `*http.Client`.
- `WithTimeout(d)` - per-request timeout when no custom client is set.
- `WithMaxRetries(n)` - retries on 429 and 5xx (default 2).
- `WithHeader(key, value)` - add a header to every request.

Anthropic-specific:

- `WithVersion(v)` - override the `anthropic-version` header.
- `WithBeta(features...)` - set `anthropic-beta` feature flags.
- `WithMaxTokens(n)` - default `max_tokens` when a request leaves it unset.

## Errors

A non-success HTTP response becomes an `*ai.APIError` with `Status`, `Type`,
`Message` and the raw body. Use `errors.As` to inspect it:

```go
var apiErr *ai.APIError
if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
	// back off
}
```

Requests missing a model or messages fail before the network with
`ai.ErrNoModel` or `ai.ErrNoMessages`.
