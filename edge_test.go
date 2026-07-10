package anthropic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/goloop/ai"
)

func TestStreamErrorEvent(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `data: {"type":"error","error":{"type":"overloaded_error","message":"busy"}}`+"\n\n")
	})
	defer done()

	var gotErr error
	for _, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			gotErr = err
		}
	}
	var apiErr *ai.APIError
	if !errors.As(gotErr, &apiErr) || apiErr.Message != "busy" {
		t.Fatalf("err = %v", gotErr)
	}
}

func TestStreamHTTPError(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"type":"authentication_error","message":"no"}}`)
	})
	defer done()

	var gotErr error
	for _, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			gotErr = err
		}
	}
	var apiErr *ai.APIError
	if !errors.As(gotErr, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("err = %v", gotErr)
	}
}

func TestImageURLMapping(t *testing.T) {
	c := New("key")
	wr, err := c.buildRequest(&ai.Request{
		Model: "m",
		Messages: []ai.Message{{Role: ai.RoleUser, Parts: []ai.Part{
			ai.Image{URL: "https://example.com/a.png"},
		}}},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	src := wr.Messages[0].Content[0].Source
	if src == nil || src.Type != "url" || src.URL != "https://example.com/a.png" {
		t.Errorf("source = %+v", src)
	}
}

func TestToolChoiceNone(t *testing.T) {
	c := New("key")
	wr, _ := c.buildRequest(&ai.Request{
		Model:      "m",
		Messages:   []ai.Message{ai.UserText("hi")},
		Tools:      []ai.Tool{{Name: "t"}},
		ToolChoice: ai.ToolNone,
	}, false)
	if wr.ToolChoice == nil || wr.ToolChoice.Type != "none" {
		t.Errorf("tool_choice = %+v", wr.ToolChoice)
	}
}

func TestRetryOnTransient(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503, retriable
			return
		}
		io.WriteString(w, `{"model":"m","content":[{"type":"text","text":"ok"}],`+
			`"usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer srv.Close()

	c := New("key", WithBaseURL(srv.URL), WithMaxRetries(2))
	resp, err := c.Generate(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "ok" || calls.Load() != 2 {
		t.Errorf("text=%q calls=%d", resp.Text(), calls.Load())
	}
}

func TestErrorPaths(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"bad"}}`)
	})
	defer done()

	ctx := context.Background()
	req := &ai.Request{Model: "m", Messages: []ai.Message{ai.UserText("hi")}}

	if _, err := c.CountTokens(ctx, req); err == nil {
		t.Error("CountTokens: want error")
	}
	if _, err := c.Models(ctx); err == nil {
		t.Error("Models: want error")
	}
	if _, err := c.GetModel(ctx, "x"); err == nil {
		t.Error("GetModel: want error")
	}
	if _, err := c.ListBatches(ctx); err == nil {
		t.Error("ListBatches: want error")
	}
	if _, err := c.CreateBatch(ctx, []BatchItem{{CustomID: "a", Request: req}}); err == nil {
		t.Error("CreateBatch: want error")
	}
}

func TestHeadersBetaVersion(t *testing.T) {
	c, done := newTestClientRaw(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") != "2099-01-01" {
			t.Errorf("version = %q", r.Header.Get("anthropic-version"))
		}
		if r.Header.Get("anthropic-beta") != "feature-1" {
			t.Errorf("beta = %q", r.Header.Get("anthropic-beta"))
		}
		io.WriteString(w, `{"input_tokens":1}`)
	}, WithVersion("2099-01-01"), WithBeta("feature-1"))
	defer done()

	if _, err := c.CountTokens(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}); err != nil {
		t.Fatal(err)
	}
}
