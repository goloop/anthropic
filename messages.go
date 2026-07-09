package anthropic

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/goloop/ai"
)

// MessagesRequest is the native Messages API request body. The shared
// [Client.Generate] builds one from an [ai.Request]; build it directly to reach
// Anthropic-only options such as TopK, Thinking, Metadata and prompt caching
// via CacheControl.
type MessagesRequest struct {
	Model         string           `json:"model"`
	MaxTokens     int              `json:"max_tokens"`
	System        string           `json:"system,omitempty"`
	Messages      []MessageParam   `json:"messages"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	ToolChoice    *ToolChoice      `json:"tool_choice,omitempty"`
	Temperature   *float64         `json:"temperature,omitempty"`
	TopP          *float64         `json:"top_p,omitempty"`
	TopK          *int             `json:"top_k,omitempty"`
	StopSequences []string         `json:"stop_sequences,omitempty"`
	Metadata      *Metadata        `json:"metadata,omitempty"`
	Thinking      *Thinking        `json:"thinking,omitempty"`
	Stream        bool             `json:"stream,omitempty"`
}

// MessageParam is one input message: a role ("user" or "assistant") and its
// content blocks.
type MessageParam struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock is one block of a message's content. The Type field selects
// which of the remaining fields apply: "text", "image", "tool_use" or
// "tool_result". Set CacheControl to mark a cache breakpoint up to this block.
type ContentBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	Source       *Source         `json:"source,omitempty"`
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	Content      string          `json:"content,omitempty"`
	IsError      bool            `json:"is_error,omitempty"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

// Source is the origin of an image block: inline base64 data or a URL.
type Source struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// ToolDefinition declares a tool the model may call. InputSchema is a JSON
// Schema object describing the tool's arguments.
type ToolDefinition struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

// ToolChoice controls whether and how the model may call tools. Type is
// "auto", "any", "tool" (with Name) or "none".
type ToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// Metadata carries request metadata, such as an opaque end-user identifier for
// abuse monitoring.
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// Thinking enables extended thinking. Set Type to "enabled" and BudgetTokens to
// the number of tokens the model may spend reasoning before it answers.
type Thinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// CacheControl marks a prompt-caching breakpoint. Type is "ephemeral".
type CacheControl struct {
	Type string `json:"type"`
}

// MessagesResponse is the native Messages API response.
type MessagesResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Model      string         `json:"model"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

// Usage reports token counts for a request. Cache fields are populated when
// prompt caching is used.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// Messages sends a native Messages API request and returns the whole response.
// Use it for Anthropic-only options; use [Client.Generate] for the shared,
// provider-agnostic path.
func (c *Client) Messages(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
	out, _, err := c.messages(ctx, req)
	return out, err
}

func (c *Client) messages(ctx context.Context, req *MessagesRequest) (*MessagesResponse, []byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	data, status, err := c.send(ctx, http.MethodPost, "/v1/messages", body)
	if err != nil {
		return nil, nil, err
	}
	if status != http.StatusOK {
		return nil, data, parseError(status, data)
	}
	var out MessagesResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, data, err
	}
	return &out, data, nil
}

// Generate sends a single messages request and returns the whole response.
// It implements [ai.Client].
func (c *Client) Generate(ctx context.Context, req *ai.Request) (*ai.Response, error) {
	mreq, err := c.buildRequest(req, false)
	if err != nil {
		return nil, err
	}
	_, raw, err := c.messages(ctx, &mreq)
	if err != nil {
		return nil, err
	}
	return parseResponse(raw)
}
