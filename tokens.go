package anthropic

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/goloop/ai"
)

// CountTokens reports how many input tokens the given request would use,
// without generating a response.
func (c *Client) CountTokens(ctx context.Context, req *ai.Request) (int, error) {
	wr, err := c.buildRequest(req, false)
	if err != nil {
		return 0, err
	}

	payload := struct {
		Model    string           `json:"model"`
		System   string           `json:"system,omitempty"`
		Messages []MessageParam   `json:"messages"`
		Tools    []ToolDefinition `json:"tools,omitempty"`
	}{
		Model:    wr.Model,
		System:   wr.System,
		Messages: wr.Messages,
		Tools:    wr.Tools,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	data, status, err := c.send(
		ctx, http.MethodPost, "/v1/messages/count_tokens", body,
	)
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK {
		return 0, parseError(status, data)
	}

	var out struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return 0, err
	}
	return out.InputTokens, nil
}
