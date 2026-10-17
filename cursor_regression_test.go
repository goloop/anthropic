package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModelsRejectRepeatedCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[],"has_more":true,"last_id":"same"}`)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := New("", WithBaseURL(srv.URL), WithMaxRetries(0)).Models(ctx)
	if err == nil || !strings.Contains(err.Error(), "repeated pagination cursor") {
		t.Fatalf("got %v, want repeated cursor error", err)
	}
}
