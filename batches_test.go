package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

func TestCreateAndGetBatch(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/v1/messages/batches"):
			var payload struct {
				Requests []struct {
					CustomID string          `json:"custom_id"`
					Params   MessagesRequest `json:"params"`
				} `json:"requests"`
			}
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Requests) != 1 || payload.Requests[0].CustomID != "a" {
				t.Errorf("requests = %+v", payload.Requests)
			}
			if payload.Requests[0].Params.Model != "m" {
				t.Errorf("model = %q", payload.Requests[0].Params.Model)
			}
			io.WriteString(w, `{"id":"batch_1","type":"message_batch",`+
				`"processing_status":"in_progress","request_counts":{"processing":1}}`)
		case strings.HasSuffix(r.URL.Path, "/v1/messages/batches/batch_1"):
			io.WriteString(w, `{"id":"batch_1","processing_status":"ended",`+
				`"request_counts":{"succeeded":1},"results_url":"`+
				"http://"+r.Host+`/results"}`)
		}
	})
	defer done()

	b, err := c.CreateBatch(context.Background(), []BatchItem{
		{CustomID: "a", Request: &ai.Request{Model: "m", Messages: []ai.Message{ai.UserText("hi")}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != "batch_1" || b.RequestCounts.Processing != 1 {
		t.Fatalf("batch = %+v", b)
	}

	got, err := c.GetBatch(context.Background(), "batch_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProcessingStatus != "ended" || got.RequestCounts.Succeeded != 1 {
		t.Errorf("batch = %+v", got)
	}
	if got.ResultsURL == "" {
		t.Errorf("no results url")
	}
}

func TestBatchResults(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"custom_id":"a","result":{"type":"succeeded"}}`+"\n"+
			`{"custom_id":"b","result":{"type":"errored"}}`+"\n")
	})
	defer done()

	// The Batch carries an absolute results URL, which the client fetches
	// directly. Point it at the test server.
	results, err := c.BatchResults(context.Background(), &Batch{ResultsURL: batchResultsURL(c)})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].CustomID != "a" || results[1].CustomID != "b" {
		t.Fatalf("results = %+v", results)
	}
}

// batchResultsURL returns the base URL configured on the client, used by the
// results test to hit the same test server.
func batchResultsURL(c *Client) string {
	return c.opts.BaseURL + "/results"
}

func TestBatchResultsNotReady(t *testing.T) {
	c := New("key")
	_, err := c.BatchResults(context.Background(), &Batch{})
	if err != ErrNoResults {
		t.Errorf("want ErrNoResults, got %v", err)
	}
}

func TestCancelAndListBatches(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cancel"):
			io.WriteString(w, `{"id":"batch_1","processing_status":"canceling"}`)
		case strings.Contains(r.URL.Path, "/v1/messages/batches"):
			io.WriteString(w, `{"data":[{"id":"batch_1","processing_status":"ended"}]}`)
		}
	})
	defer done()

	b, err := c.CancelBatch(context.Background(), "batch_1")
	if err != nil {
		t.Fatal(err)
	}
	if b.ProcessingStatus != "canceling" {
		t.Errorf("status = %q", b.ProcessingStatus)
	}

	list, err := c.ListBatches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "batch_1" {
		t.Errorf("list = %+v", list)
	}
}
