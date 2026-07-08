package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/goloop/ai"
)

// buildRequest converts a provider-agnostic ai.Request into the Anthropic wire
// request. RoleSystem messages are folded into the top-level system prompt.
func (c *Client) buildRequest(req *ai.Request, stream bool) (wireRequest, error) {
	if err := req.Validate(); err != nil {
		return wireRequest{}, err
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.maxTokens
	}

	wr := wireRequest{
		Model:         req.Model,
		MaxTokens:     maxTokens,
		System:        req.System,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.Stop,
		Stream:        stream,
	}

	for _, m := range req.Messages {
		if m.Role == ai.RoleSystem {
			if s := systemText(m); s != "" {
				if wr.System != "" {
					wr.System += "\n\n"
				}
				wr.System += s
			}
			continue
		}
		wr.Messages = append(wr.Messages, wireMessage{
			Role:    wireRole(m.Role),
			Content: convParts(m.Parts),
		})
	}

	for _, t := range req.Tools {
		schema := t.Schema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		wr.Tools = append(wr.Tools, wireTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	if len(wr.Tools) > 0 {
		wr.ToolChoice = convToolChoice(req.ToolChoice)
	}

	return wr, nil
}

// wireRole maps an ai.Role to an Anthropic role. Tool-result messages are sent
// as user turns, which is how the Messages API expects them.
func wireRole(r ai.Role) string {
	if r == ai.RoleAssistant {
		return "assistant"
	}
	return "user"
}

func systemText(m ai.Message) string {
	var b strings.Builder
	for _, p := range m.Parts {
		if t, ok := p.(ai.Text); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

func convParts(parts []ai.Part) []wireBlock {
	out := make([]wireBlock, 0, len(parts))
	for _, p := range parts {
		out = append(out, convPart(p))
	}
	return out
}

func convPart(p ai.Part) wireBlock {
	switch v := p.(type) {
	case ai.Text:
		return wireBlock{Type: "text", Text: v.Text}
	case ai.Image:
		if len(v.Data) > 0 {
			return wireBlock{Type: "image", Source: &wireSource{
				Type:      "base64",
				MediaType: v.MIME,
				Data:      base64.StdEncoding.EncodeToString(v.Data),
			}}
		}
		return wireBlock{Type: "image", Source: &wireSource{Type: "url", URL: v.URL}}
	case ai.ToolUse:
		return wireBlock{Type: "tool_use", ID: v.ID, Name: v.Name, Input: v.Input}
	case ai.ToolResult:
		return wireBlock{
			Type:      "tool_result",
			ToolUseID: v.ID,
			Content:   v.Content,
			IsError:   v.IsError,
		}
	}
	return wireBlock{}
}

func convToolChoice(tc ai.ToolChoice) *wireToolChoice {
	switch tc {
	case ai.ToolNone:
		return &wireToolChoice{Type: "none"}
	case ai.ToolRequired:
		return &wireToolChoice{Type: "any"}
	default:
		return &wireToolChoice{Type: "auto"}
	}
}

// parseResponse converts an Anthropic messages response into an ai.Response.
func parseResponse(body []byte) (*ai.Response, error) {
	var wr wireResponse
	if err := json.Unmarshal(body, &wr); err != nil {
		return nil, err
	}

	resp := &ai.Response{
		Model:      wr.Model,
		StopReason: wr.StopReason,
		Usage: ai.Usage{
			InputTokens:  wr.Usage.InputTokens,
			OutputTokens: wr.Usage.OutputTokens,
		},
		Raw: append(json.RawMessage(nil), body...),
	}
	for _, b := range wr.Content {
		switch b.Type {
		case "text":
			resp.Parts = append(resp.Parts, ai.Text{Text: b.Text})
		case "tool_use":
			resp.Parts = append(resp.Parts, ai.ToolUse{
				ID:    b.ID,
				Name:  b.Name,
				Input: b.Input,
			})
		}
	}
	return resp, nil
}
