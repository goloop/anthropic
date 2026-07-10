package anthropic

import (
	"context"
	"io"
	"net/http"
	"testing"
)

// TestListBatchesPaginates verifies ListBatches walks every page via after_id
// instead of returning only the first page.
func TestListBatchesPaginates(t *testing.T) {
	var gotAfter []string
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAfter = append(gotAfter, r.URL.Query().Get("after_id"))
		switch r.URL.Query().Get("after_id") {
		case "":
			io.WriteString(w, `{"data":[{"id":"b1"},{"id":"b2"}],"has_more":true,"last_id":"b2"}`)
		case "b2":
			io.WriteString(w, `{"data":[{"id":"b3"}],"has_more":false,"last_id":"b3"}`)
		default:
			t.Errorf("unexpected after_id %q", r.URL.Query().Get("after_id"))
		}
	})
	defer done()

	batches, err := c.ListBatches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 {
		t.Fatalf("got %d batches, want 3", len(batches))
	}
	if batches[2].ID != "b3" {
		t.Errorf("last batch = %q, want b3", batches[2].ID)
	}
	if len(gotAfter) != 2 || gotAfter[0] != "" || gotAfter[1] != "b2" {
		t.Errorf("after_id sequence = %v, want [\"\" \"b2\"]", gotAfter)
	}
}

// TestModelsPaginates verifies Models walks every page too.
func TestModelsPaginates(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("after_id") == "" {
			io.WriteString(w, `{"data":[{"id":"m1"}],"has_more":true,"last_id":"m1"}`)
			return
		}
		io.WriteString(w, `{"data":[{"id":"m2"}],"has_more":false,"last_id":"m2"}`)
	})
	defer done()

	models, err := c.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[1].ID != "m2" {
		t.Fatalf("models = %+v", models)
	}
}
