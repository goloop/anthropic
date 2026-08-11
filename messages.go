package anthropic

import (
	"bytes"
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

	// Citations are the sources a server-side tool attached to a text block.
	// They arrive on the reply only; nothing sends them back.
	Citations []Citation `json:"citations,omitempty"`

	// SearchResults holds what a "web_search_tool_result" block carried. That
	// block puts a list where every other block puts a string, which is why
	// it needs a field of its own rather than sharing Content. It is filled
	// on decoding and never sent.
	SearchResults []WebSearchResult `json:"-"`
}

// WebSearchResult is one page a server-side search found.
type WebSearchResult struct {
	Type             string `json:"type"`
	URL              string `json:"url,omitempty"`
	Title            string `json:"title,omitempty"`
	PageAge          string `json:"page_age,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`
}

// UnmarshalJSON decodes a content block, accepting either shape of "content".
// A tool result the caller sent carries a string there; a search result the
// provider produced carries a list of pages. Without this, one search result
// block would fail the whole reply, and the reply is the answer.
func (b *ContentBlock) UnmarshalJSON(data []byte) error {
	// The alias sheds this method so the embedded decoding is the plain one.
	type plain ContentBlock
	var w struct {
		plain
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	*b = ContentBlock(w.plain)

	content := bytes.TrimSpace(w.Content)
	if len(content) == 0 || string(content) == "null" {
		return nil
	}
	if content[0] == '[' {
		return json.Unmarshal(content, &b.SearchResults)
	}
	return json.Unmarshal(content, &b.Content)
}

// Citation is one source a server-side tool attached to a text block. Anthropic
// reports the fragment of the source it used rather than offsets into the
// answer, which is why [ai.Citation] carries CitedText and leaves its byte
// range at zero here.
type Citation struct {
	Type           string `json:"type"`
	URL            string `json:"url,omitempty"`
	Title          string `json:"title,omitempty"`
	CitedText      string `json:"cited_text,omitempty"`
	EncryptedIndex string `json:"encrypted_index,omitempty"`
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
//
// The same list also carries Anthropic's own server-side tools, which is how
// the Messages API takes them: those set Type to a versioned tool identifier
// such as [WebSearchToolType], carry no schema, and are answered by Anthropic
// rather than by the caller. The fields below Type only apply to those.
type ToolDefinition struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`

	// Type names a server-side tool. It is empty for a caller's own tool,
	// which is how the Messages API tells the two apart.
	Type string `json:"type,omitempty"`

	// MaxUses bounds how many times Anthropic may run the tool.
	MaxUses int `json:"max_uses,omitempty"`

	// AllowedDomains and BlockedDomains narrow a search. Anthropic rejects a
	// request that sets both.
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`

	// UserLocation biases results towards a place.
	UserLocation *UserLocation `json:"user_location,omitempty"`
}

// UserLocation biases search results towards a place. Type is "approximate".
type UserLocation struct {
	Type     string `json:"type"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
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

	// ServerToolUse counts work Anthropic did on its own side. It is billed
	// apart from tokens, so it is the only place the cost of a search shows.
	ServerToolUse *ServerToolUse `json:"server_tool_use,omitempty"`
}

// ServerToolUse counts how many times Anthropic ran each of its own tools for
// a request.
type ServerToolUse struct {
	WebSearchRequests int `json:"web_search_requests,omitempty"`
}

// Messages sends a native Messages API request and returns the whole response.
// Use it for Anthropic-only options; use [Client.Generate] for the shared,
// provider-agnostic path.
func (c *Client) Messages(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
	out, _, err := c.messages(ctx, req)
	return out, err
}

func (c *Client) messages(ctx context.Context, req *MessagesRequest) (*MessagesResponse, []byte, error) {
	if req == nil {
		return nil, nil, ai.ErrNoRequest
	}
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
		return nil, wrapUnsupportedCapability(req, err)
	}
	resp, calls, err := parseResponse(raw)
	if err != nil {
		return nil, err
	}
	resp.Format = formatMode(req.Format)
	if resp.Hosted, err = req.HostedReports(calls); err != nil {
		return nil, err
	}
	return resp, nil
}
