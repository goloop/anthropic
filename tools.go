package anthropic

import (
	"context"
	"io"
	"net/http"
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
