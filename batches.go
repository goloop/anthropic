package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/goloop/ai"
)

// BatchItem is one request in a message batch, tagged with a custom ID you
// choose so results can be correlated back to it.
type BatchItem struct {
	CustomID string
	Request  *ai.Request
}

// Batch is the state of a message batch.
type Batch struct {
	ID               string      `json:"id"`
	Type             string      `json:"type"`
	ProcessingStatus string      `json:"processing_status"`
	RequestCounts    BatchCounts `json:"request_counts"`
	CreatedAt        time.Time   `json:"created_at"`
	EndedAt          *time.Time  `json:"ended_at"`
	ExpiresAt        *time.Time  `json:"expires_at"`
	ResultsURL       string      `json:"results_url"`
}

// BatchCounts breaks down how many requests are in each processing state.
type BatchCounts struct {
	Processing int `json:"processing"`
	Succeeded  int `json:"succeeded"`
	Errored    int `json:"errored"`
	Canceled   int `json:"canceled"`
	Expired    int `json:"expired"`
}

// BatchResult is one line of a batch's results: a custom ID and the raw result
// object for that request.
type BatchResult struct {
	CustomID string          `json:"custom_id"`
	Result   json.RawMessage `json:"result"`
}

// CreateBatch submits a set of message requests for asynchronous processing.
func (c *Client) CreateBatch(ctx context.Context, items []BatchItem) (*Batch, error) {
	type reqItem struct {
		CustomID string      `json:"custom_id"`
		Params   wireRequest `json:"params"`
	}
	var payload struct {
		Requests []reqItem `json:"requests"`
	}
	for _, it := range items {
		wr, err := c.buildRequest(it.Request, false)
		if err != nil {
			return nil, err
		}
		payload.Requests = append(payload.Requests, reqItem{
			CustomID: it.CustomID,
			Params:   wr,
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.batchCall(ctx, http.MethodPost, "/v1/messages/batches", body)
}

// GetBatch returns the current state of a batch.
func (c *Client) GetBatch(ctx context.Context, id string) (*Batch, error) {
	return c.batchCall(ctx, http.MethodGet, "/v1/messages/batches/"+id, nil)
}

// CancelBatch requests cancellation of a batch still in progress.
func (c *Client) CancelBatch(ctx context.Context, id string) (*Batch, error) {
	return c.batchCall(ctx, http.MethodPost, "/v1/messages/batches/"+id+"/cancel", nil)
}

// ListBatches returns batches for the account, most recent first.
func (c *Client) ListBatches(ctx context.Context) ([]Batch, error) {
	data, status, err := c.send(ctx, http.MethodGet, "/v1/messages/batches?limit=100", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, parseError(status, data)
	}

	var out struct {
		Data []Batch `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// BatchResults fetches and parses the JSONL results of a finished batch. The
// batch must have ended and carry a ResultsURL; otherwise ErrNoResults is
// returned.
func (c *Client) BatchResults(ctx context.Context, b *Batch) ([]BatchResult, error) {
	if b.ResultsURL == "" {
		return nil, ErrNoResults
	}

	resp, err := c.opts.Do(ctx, http.MethodGet, b.ResultsURL, nil, c.headers())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, data)
	}

	var results []BatchResult
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var r BatchResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (c *Client) batchCall(
	ctx context.Context,
	method, path string,
	body []byte,
) (*Batch, error) {
	data, status, err := c.send(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, parseError(status, data)
	}

	var b Batch
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
