package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// Model describes a model returned by the models endpoint.
type Model struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	Type        string    `json:"type"`
}

// Models lists the models available to the account.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	return listAll[Model](ctx, c, "/v1/models", 1000)
}

// GetModel returns a single model by ID.
func (c *Client) GetModel(ctx context.Context, id string) (*Model, error) {
	data, status, err := c.send(ctx, http.MethodGet,
		"/v1/models/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, parseError(status, data)
	}

	var m Model
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
