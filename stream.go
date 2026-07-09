package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/goloop/ai"
)

type streamEvent struct {
	Type         string        `json:"type"`
	Index        int           `json:"index"`
	Message      *wireResponse `json:"message"`
	ContentBlock *wireBlock    `json:"content_block"`
	Delta        *streamDelta  `json:"delta"`
	Usage        *wireUsage    `json:"usage"`
	Error        *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type streamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	StopReason  string `json:"stop_reason"`
}

// Stream sends a messages request with streaming enabled and returns an
// iterator over response chunks. It implements [ai.Client]. Text deltas arrive
// as chunks with Text set; a completed tool call arrives as a chunk with
// ToolCall set; the final chunk has Done true and carries token usage.
func (c *Client) Stream(ctx context.Context, req *ai.Request) iter.Seq2[ai.Chunk, error] {
	return func(yield func(ai.Chunk, error) bool) {
		wr, err := c.buildRequest(req, true)
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		body, err := json.Marshal(wr)
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}

		h := c.headers()
		h.Set("accept", "text/event-stream")
		resp, err := c.opts.Do(
			ctx, http.MethodPost, c.opts.BaseURL+"/v1/messages", body, h,
		)
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			yield(ai.Chunk{}, parseError(resp.StatusCode, data))
			return
		}

		type toolAcc struct {
			id, name string
			buf      strings.Builder
		}
		tools := map[int]*toolAcc{}
		var usage ai.Usage

		for data, err := range ai.SSEEvents(resp.Body) {
			if err != nil {
				yield(ai.Chunk{}, err)
				return
			}

			var ev streamEvent
			if e := json.Unmarshal([]byte(data), &ev); e != nil {
				yield(ai.Chunk{}, e)
				return
			}

			switch ev.Type {
			case "message_start":
				if ev.Message != nil {
					usage.InputTokens = ev.Message.Usage.InputTokens
				}
			case "content_block_start":
				if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {
					tools[ev.Index] = &toolAcc{
						id:   ev.ContentBlock.ID,
						name: ev.ContentBlock.Name,
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
				}
			case "content_block_stop":
				if t := tools[ev.Index]; t != nil {
					input := t.buf.String()
					if input == "" {
						input = "{}"
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
				}
			case "message_stop":
				final := usage
				yield(ai.Chunk{Done: true, Usage: &final, Raw: json.RawMessage(data)}, nil)
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
