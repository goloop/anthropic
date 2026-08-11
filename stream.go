package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/goloop/ai"
)

// StreamEvent is one raw event of a streaming Messages response. The Type
// field selects which fields are populated ("message_start", "content_block_
// start", "content_block_delta", "content_block_stop", "message_delta",
// "message_stop", "error").
type StreamEvent struct {
	Type         string            `json:"type"`
	Index        int               `json:"index"`
	Message      *MessagesResponse `json:"message"`
	ContentBlock *ContentBlock     `json:"content_block"`
	Delta        *StreamDelta      `json:"delta"`
	Usage        *Usage            `json:"usage"`
	Error        *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// StreamDelta is the incremental payload of a content_block_delta or
// message_delta event.
type StreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	StopReason  string `json:"stop_reason"`

	// Citation carries one source, on a delta of type "citations_delta".
	Citation *Citation `json:"citation,omitempty"`
}

// openMessagesStream opens the streaming /v1/messages connection for a native
// request. The caller owns the returned response body.
func (c *Client) openMessagesStream(ctx context.Context, req *MessagesRequest) (*http.Response, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	h := c.headers()
	h.Set("accept", "text/event-stream")
	resp, err := c.opts.Do(ctx, http.MethodPost, c.opts.BaseURL+"/v1/messages", body, h)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := readLimited(resp.Body)
		resp.Body.Close()
		return nil, parseError(resp.StatusCode, data)
	}
	return resp, nil
}

// MessagesStream sends a native streaming Messages request and yields each raw
// event as it arrives. Use it for Anthropic-only options; use [Client.Stream]
// for the shared, provider-agnostic chunk stream.
func (c *Client) MessagesStream(ctx context.Context, req *MessagesRequest) iter.Seq2[StreamEvent, error] {
	return func(yield func(StreamEvent, error) bool) {
		if req == nil {
			yield(StreamEvent{}, ai.ErrNoRequest)
			return
		}
		r := *req // do not mutate the caller's request
		resp, err := c.openMessagesStream(ctx, &r)
		if err != nil {
			yield(StreamEvent{}, err)
			return
		}
		defer resp.Body.Close()

		for data, err := range ai.SSEEvents(resp.Body) {
			if err != nil {
				yield(StreamEvent{}, err)
				return
			}
			var ev StreamEvent
			if e := json.Unmarshal([]byte(data), &ev); e != nil {
				yield(StreamEvent{}, e)
				return
			}
			if !yield(ev, nil) {
				return
			}
		}
	}
}

// Stream sends a messages request with streaming enabled and returns an
// iterator over response chunks. It implements [ai.Client]. Text deltas arrive
// as chunks with Text set; a completed tool call arrives as a chunk with
// ToolCall set; the final chunk has Done true and carries token usage.
func (c *Client) Stream(ctx context.Context, req *ai.Request) iter.Seq2[ai.Chunk, error] {
	return func(yield func(ai.Chunk, error) bool) {
		mreq, err := c.buildRequest(req, true)
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		resp, err := c.openMessagesStream(ctx, &mreq)
		if err != nil {
			yield(ai.Chunk{}, wrapUnsupportedCapability(req, err))
			return
		}
		defer resp.Body.Close()

		type toolAcc struct {
			id, name string
			buf      strings.Builder
		}
		tools := map[int]*toolAcc{}
		var usage ai.Usage

		// searches is what the provider reported it ran, and blocks is what
		// was seen going past. The reported count is authoritative because it
		// is what gets billed, but it does not arrive in every stream, so a
		// count of the server_tool_use blocks stands in for it. Proving a
		// search happened matters more than knowing exactly how many.
		var searches, blocks int

		for data, err := range ai.SSEEvents(resp.Body) {
			if err != nil {
				yield(ai.Chunk{}, err)
				return
			}

			var ev StreamEvent
			if e := json.Unmarshal([]byte(data), &ev); e != nil {
				yield(ai.Chunk{}, e)
				return
			}

			switch ev.Type {
			case "message_start":
				if ev.Message != nil {
					usage.InputTokens = ev.Message.Usage.InputTokens
					searches += hostedCalls(ev.Message.Usage)[ai.HostedWebSearch]
				}
			case "content_block_start":
				if ev.ContentBlock == nil {
					continue
				}
				switch ev.ContentBlock.Type {
				case "tool_use":
					tools[ev.Index] = &toolAcc{
						id:   ev.ContentBlock.ID,
						name: ev.ContentBlock.Name,
					}
				case "server_tool_use":
					// Not accumulated as a tool call: this one is Anthropic's
					// own, and nobody is waiting for an answer to it.
					if ev.ContentBlock.Name == webSearchToolName {
						blocks++
					}
				}
			case "content_block_delta":
				if ev.Delta == nil {
					continue
				}
				switch ev.Delta.Type {
				case "text_delta":
					if ev.Delta.Text != "" {
						if !yield(ai.Chunk{Text: ev.Delta.Text, Raw: json.RawMessage(data)}, nil) {
							return
						}
					}
				case "input_json_delta":
					if t := tools[ev.Index]; t != nil {
						t.buf.WriteString(ev.Delta.PartialJSON)
					}
				case "citations_delta":
					// A source arrives in its own event, after the text it
					// supports has already been yielded. It is passed on as
					// it comes rather than held back to be paired with that
					// text, because holding it back would mean buffering the
					// answer and giving up what a stream is for.
					if ev.Delta.Citation != nil {
						cs := convCitations([]Citation{*ev.Delta.Citation})
						if !yield(ai.Chunk{
							Citations: cs,
							Raw:       json.RawMessage(data),
						}, nil) {
							return
						}
					}
				}
			case "content_block_stop":
				if t := tools[ev.Index]; t != nil {
					input := t.buf.String()
					if input == "" {
						input = "{}"
					}
					if !json.Valid([]byte(input)) {
						yield(ai.Chunk{}, fmt.Errorf(
							"anthropic: tool call %q has invalid JSON arguments",
							t.name))
						return
					}
					call := ai.ToolUse{ID: t.id, Name: t.name, Input: json.RawMessage(input)}
					if !yield(ai.Chunk{ToolCall: &call, Raw: json.RawMessage(data)}, nil) {
						return
					}
					delete(tools, ev.Index)
				}
			case "message_delta":
				if ev.Usage != nil {
					usage.OutputTokens = ev.Usage.OutputTokens
					searches += hostedCalls(*ev.Usage)[ai.HostedWebSearch]
				}
			case "message_stop":
				if searches == 0 {
					searches = blocks
				}
				reports, err := req.HostedReports(
					map[ai.HostedKind]int{ai.HostedWebSearch: searches})
				if err != nil {
					yield(ai.Chunk{}, err)
					return
				}
				final := usage
				yield(ai.Chunk{
					Done:   true,
					Usage:  &final,
					Hosted: reports,
					Raw:    json.RawMessage(data),
				}, nil)
				return
			case "error":
				msg, typ := "", ""
				if ev.Error != nil {
					msg, typ = ev.Error.Message, ev.Error.Type
				}
				// The HTTP status is 200 for a mid-stream error event, so
				// map the error type to a meaningful status instead.
				yield(ai.Chunk{}, &ai.APIError{
					Status:  streamErrorStatus(typ),
					Type:    typ,
					Message: msg,
					Raw:     append(json.RawMessage(nil), data...),
				})
				return
			}
		}

		// The stream ended without a message_stop event: it was truncated.
		yield(ai.Chunk{}, io.ErrUnexpectedEOF)
	}
}

// streamErrorStatus maps an Anthropic stream error type to an HTTP-like status
// so callers can branch on APIError.Status as they would for a request error.
func streamErrorStatus(typ string) int {
	switch typ {
	case "overloaded_error":
		return 529
	case "rate_limit_error":
		return http.StatusTooManyRequests
	case "authentication_error", "permission_error":
		return http.StatusUnauthorized
	case "invalid_request_error":
		return http.StatusBadRequest
	default:
		return 0
	}
}
