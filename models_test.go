package anthropic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestModels(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/models") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"data":[{"id":"claude-x","display_name":"Claude X",`+
			`"created_at":"2025-01-01T00:00:00Z","type":"model"}],"has_more":false}`)
	})
	defer done()

	models, err := c.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "claude-x" {
		t.Fatalf("models = %+v", models)
	}
	if models[0].DisplayName != "Claude X" {
		t.Errorf("display name = %q", models[0].DisplayName)
	}
	if models[0].CreatedAt.Year() != 2025 {
		t.Errorf("created_at = %v", models[0].CreatedAt)
	}
}

func TestGetModel(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/models/claude-x") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"id":"claude-x","display_name":"Claude X","type":"model"}`)
	})
	defer done()

	m, err := c.GetModel(context.Background(), "claude-x")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "claude-x" {
		t.Errorf("id = %q", m.ID)
	}
}
