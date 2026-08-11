package anthropic

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/goloop/ai"
)

// clientForCapabilities builds a client the way New does, so the test checks
// the same configuration a caller gets.
func clientForCapabilities(t *testing.T) *Client {
	t.Helper()
	return New("k")
}

// A 400 that names the hosted tool is the provider saying this model cannot
// search. Both readings must survive: errors.Is so the application can degrade
// with one check, errors.As so the provider's own message is still there to
// log.
func TestWrapUnsupportedKeepsBothReadings(t *testing.T) {
	original := &ai.APIError{
		Status:  http.StatusBadRequest,
		Message: "web_search is not supported by this model",
		Raw:     json.RawMessage(`{"error":{"type":"invalid_request_error"}}`),
	}

	req := &ai.Request{
		Model:    "the-model",
		Messages: []ai.Message{ai.UserText("hi")},
		Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
	}

	err := wrapUnsupportedCapability(req, original)
	if !errors.Is(err, ai.ErrNoHosted) {
		t.Fatalf("wrapUnsupportedCapability() = %v, want ai.ErrNoHosted", err)
	}

	var apiErr *ai.APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("the provider's APIError was lost")
	}
	if apiErr.Status != http.StatusBadRequest || apiErr.Message != original.Message {
		t.Errorf("APIError = %+v, want the original", apiErr)
	}
}

// This provider has no response_format: a Format is asked for in the system
// prompt, so the provider has nothing to reject and a 400 mentioning one is
// about something else. Translating it would send the caller retrying without
// a format that was never the problem.
func TestFormatRefusalIsNotTranslatedHere(t *testing.T) {
	req := &ai.Request{
		Model:    "the-model",
		Messages: []ai.Message{ai.UserText("hi")},
		Format:   &ai.Format{Type: ai.FormatJSON},
	}

	err := wrapUnsupportedCapability(req, &ai.APIError{
		Status:  http.StatusBadRequest,
		Message: "response_format is not supported by this model",
	})
	if errors.Is(err, ai.ErrNoFormat) {
		t.Error("a format refusal was invented for a provider that never " +
			"receives a format field")
	}
}

// The narrow rules matter more than the translation: a false positive turns a
// genuinely bad request into an endless "the provider cannot do this" retry.
func TestWrapUnsupportedIsNarrow(t *testing.T) {
	searching := func() *ai.Request {
		return &ai.Request{
			Model:    "the-model",
			Messages: []ai.Message{ai.UserText("hi")},
			Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
		}
	}
	plain := func() *ai.Request {
		return &ai.Request{
			Model:    "the-model",
			Messages: []ai.Message{ai.UserText("hi")},
		}
	}

	tests := []struct {
		name string
		req  *ai.Request
		err  error
	}{
		{
			name: "not a 400",
			req:  searching(),
			err: &ai.APIError{
				Status:  http.StatusInternalServerError,
				Message: "web_search is not supported by this model",
			},
		},
		{
			name: "the caller never asked for it",
			req:  plain(),
			err: &ai.APIError{
				Status:  http.StatusBadRequest,
				Message: "web_search is not supported by this model",
			},
		},
		{
			name: "a 400 about something else",
			req:  searching(),
			err: &ai.APIError{
				Status:  http.StatusBadRequest,
				Message: "messages: at least one message is required",
			},
		},
		{
			name: "names the feature but is not a refusal",
			req:  searching(),
			err: &ai.APIError{
				Status:  http.StatusBadRequest,
				Message: "web_search max_uses must be a positive integer",
			},
		},
		{
			name: "not an APIError at all",
			req:  searching(),
			err:  errors.New("dial tcp: connection refused"),
		},
		{
			name: "no request to check against",
			req:  nil,
			err: &ai.APIError{
				Status:  http.StatusBadRequest,
				Message: "web_search is not supported by this model",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapUnsupportedCapability(tt.req, tt.err)
			if errors.Is(got, ai.ErrNoFormat) || errors.Is(got, ai.ErrNoHosted) {
				t.Errorf("wrapUnsupportedCapability() translated %v", tt.err)
			}
			if got != tt.err {
				t.Errorf("the error was replaced: %v", got)
			}
		})
	}
}

// A structured field beats prose: it survives rewording and localisation.
func TestWrapUnsupportedPrefersStructuredFields(t *testing.T) {
	req := &ai.Request{
		Model:    "the-model",
		Messages: []ai.Message{ai.UserText("hi")},
		Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
	}

	// A message in a language nobody matched, with the blamed parameter named
	// in the body where the provider reports it.
	err := wrapUnsupportedCapability(req, &ai.APIError{
		Status:  http.StatusBadRequest,
		Message: "непідтримуване значення",
		Raw:     json.RawMessage(`{"error":{"param":"web_search"}}`),
	})
	if !errors.Is(err, ai.ErrNoHosted) {
		t.Errorf("a named parameter was ignored in favour of prose: %v", err)
	}
}

// The table has to stay honest mechanically, not by discipline: every setting
// Capabilities reports as unsupported must actually be refused before the
// request leaves, and every supported one must actually go through.
func TestCapabilitiesMatchWhatTheDriverDoes(t *testing.T) {
	web := (&Client{}).Capabilities().Hosted[ai.HostedWebSearch].Web
	if web == nil {
		t.Fatal("this driver reports no web search capability")
	}

	tests := []struct {
		name string
		web  ai.HostedWeb
		ok   bool
	}{
		{"use limit", ai.HostedWeb{MaxUses: 2}, web.MaxUses},
		{"allowed domains", ai.HostedWeb{AllowDomains: []string{"a.example"}}, web.AllowDomains},
		{"blocked domains", ai.HostedWeb{BlockDomains: []string{"b.example"}}, web.BlockDomains},
		{"region", ai.HostedWeb{Region: "UA"}, web.Region},
		{
			"both domain lists",
			ai.HostedWeb{
				AllowDomains: []string{"a.example"},
				BlockDomains: []string{"b.example"},
			},
			web.BothDomainLists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := tt.web
			req := &ai.Request{
				Model:    "the-model",
				Messages: []ai.Message{ai.UserText("hi")},
				Hosted: []ai.Hosted{{
					Kind: ai.HostedWebSearch,
					Web:  &settings,
				}},
			}

			_, err := clientForCapabilities(t).buildRequest(req, false)
			refused := errors.Is(err, ai.ErrNoHosted)
			switch {
			case tt.ok && refused:
				t.Errorf("Capabilities claims this is supported, but the "+
					"request was refused: %v", err)
			case !tt.ok && !refused:
				t.Errorf("Capabilities claims this is unsupported, but the "+
					"request was accepted (err = %v)", err)
			}

			if got := ai.SupportsHosted(clientForCapabilities(t), req.Hosted[0]); got != tt.ok {
				t.Errorf("ai.SupportsHosted() = %v, want %v", got, tt.ok)
			}
		})
	}
}
