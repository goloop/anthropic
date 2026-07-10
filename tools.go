package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// headers returns the headers common to every Anthropic request.
func (c *Client) headers() http.Header {
	h := http.Header{}
	h.Set("x-api-key", c.opts.APIKey)
	h.Set("anthropic-version", c.version)
	h.Set("content-type", "application/json")
	if len(c.beta) > 0 {
		h.Set("anthropic-beta", strings.Join(c.beta, ","))
	}
	return h
}

// send performs a request against a path under the base URL and returns the
// response body and status code.
func (c *Client) send(
	ctx context.Context,
	method, path string,
	body []byte,
) ([]byte, int, error) {
	resp, err := c.opts.Do(ctx, method, c.opts.BaseURL+path, body, c.headers())
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// listAll fetches every page of a cursor-paginated list endpoint and returns
// the concatenated items. Anthropic returns {"data":[...],"has_more":bool,
// "last_id":"..."}; pages are walked with the after_id query parameter until
// has_more is false, so callers never silently see only the first page.
func listAll[T any](ctx context.Context, c *Client, path string, limit int) ([]T, error) {
	var all []T
	afterID := ""
	for {
		u := fmt.Sprintf("%s?limit=%d", path, limit)
		if afterID != "" {
			u += "&after_id=" + url.QueryEscape(afterID)
		}
		data, status, err := c.send(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, parseError(status, data)
		}
		var page struct {
			Data    []T    `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if !page.HasMore || page.LastID == "" {
			return all, nil
		}
		afterID = page.LastID
	}
}
