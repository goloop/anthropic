package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

func askForSearch(h ...ai.Hosted) *ai.Request {
	return &ai.Request{
		Model:    "the-model",
		Messages: []ai.Message{ai.UserText("What happened today?")},
		Hosted:   h,
	}
}

func TestHostedWebSearchReachesTheRequest(t *testing.T) {
	c := &Client{maxTokens: 1024, webSearchType: WebSearchToolType}

	wr, err := c.buildRequest(askForSearch(ai.Hosted{
		Kind: ai.HostedWebSearch,
		Web: &ai.HostedWeb{
			MaxUses:      3,
			AllowDomains: []string{"example.org"},
			Region:       "UA",
		},
	}), false)
	if err != nil {
		t.Fatal(err)
	}

	if len(wr.Tools) != 1 {
		t.Fatalf("Tools = %+v, want one server tool", wr.Tools)
	}
	got := wr.Tools[0]
	if got.Type != WebSearchToolType || got.Name != webSearchToolName {
		t.Errorf("tool = %+v, want the server web search tool", got)
	}
	if got.MaxUses != 3 {
		t.Errorf("MaxUses = %d, want 3", got.MaxUses)
	}
	if len(got.AllowedDomains) != 1 || got.AllowedDomains[0] != "example.org" {
		t.Errorf("AllowedDomains = %v", got.AllowedDomains)
	}
	if got.UserLocation == nil || got.UserLocation.Country != "UA" {
		t.Errorf("UserLocation = %+v, want country UA", got.UserLocation)
	}

	// A server tool carries no schema, and a stray "input_schema": null would
	// be rejected by the provider.
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "input_schema") {
		t.Errorf("server tool carries a schema: %s", body)
	}
}

// A request that asks for nothing hosted must serialize exactly as it did
// before hosted capabilities existed.
func TestNoHostedLeavesTheRequestAlone(t *testing.T) {
	c := &Client{maxTokens: 1024, webSearchType: WebSearchToolType}

	wr, err := c.buildRequest(askForSearch(), false)
	if err != nil {
		t.Fatal(err)
	}
	if wr.Tools != nil {
		t.Errorf("Tools = %+v, want none", wr.Tools)
	}
}

// The caller's own tools and the provider's live in one list on the wire, but
// the tool choice is only about the caller's: a forced choice must not be
// satisfiable by a search the caller never has to answer.
func TestHostedJoinsCallerToolsWithoutChangingTheChoice(t *testing.T) {
	c := &Client{maxTokens: 1024, webSearchType: WebSearchToolType}

	req := askForSearch(ai.Hosted{Kind: ai.HostedWebSearch})
	req.Tools = []ai.Tool{{Name: "lookup", Schema: json.RawMessage(`{"type":"object"}`)}}
	req.ToolChoice = ai.ToolRequired

	wr, err := c.buildRequest(req, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(wr.Tools) != 2 || wr.Tools[0].Name != "lookup" {
		t.Fatalf("Tools = %+v, want the caller's tool first", wr.Tools)
	}
	if wr.ToolChoice == nil || wr.ToolChoice.Type != "any" {
		t.Errorf("ToolChoice = %+v, want the caller's choice untouched", wr.ToolChoice)
	}
}

// Anthropic takes allowed or blocked domains, never both. Picking one would
// silently widen a search past what the caller allowed.
func TestHostedRejectsBothDomainLists(t *testing.T) {
	c := &Client{maxTokens: 1024, webSearchType: WebSearchToolType}

	_, err := c.buildRequest(askForSearch(ai.Hosted{
		Kind: ai.HostedWebSearch,
		Web: &ai.HostedWeb{
			AllowDomains: []string{"a.example"},
			BlockDomains: []string{"b.example"},
		},
	}), false)
	if !errors.Is(err, ai.ErrNoHosted) {
		t.Errorf("buildRequest() error = %v, want ErrNoHosted", err)
	}
}

func TestWithWebSearchToolOverridesTheVersion(t *testing.T) {
	c := New("k", WithWebSearchTool("web_search_20990101"))

	wr, err := c.buildRequest(askForSearch(ai.Hosted{Kind: ai.HostedWebSearch}), false)
	if err != nil {
		t.Fatal(err)
	}
	if wr.Tools[0].Type != "web_search_20990101" {
		t.Errorf("tool type = %q, want the override", wr.Tools[0].Type)
	}
}

// One block puts a string in "content" and another puts a list there, and both
// arrive in the same reply.
func TestContentBlockTakesEitherContentShape(t *testing.T) {
	const blocks = `[
	  {"type":"tool_result","tool_use_id":"t1","content":"42"},
	  {"type":"web_search_tool_result","tool_use_id":"s1",
	   "content":[{"type":"web_search_result","url":"https://example.org",
	               "title":"A"}]},
	  {"type":"text","text":"done"}
	]`

	var got []ContentBlock
	if err := json.Unmarshal([]byte(blocks), &got); err != nil {
		t.Fatal(err)
	}
	if got[0].Content != "42" {
		t.Errorf("tool result content = %q, want 42", got[0].Content)
	}
	if len(got[1].SearchResults) != 1 ||
		got[1].SearchResults[0].URL != "https://example.org" {
		t.Errorf("search results = %+v", got[1].SearchResults)
	}
	if got[2].Text != "done" {
		t.Errorf("text = %q", got[2].Text)
	}
}

// searchReply is a Messages response of the shape a search produces: citations
// on the text block, the provider's own tool blocks, and the search count.
const searchReply = `{
  "model": "the-model",
  "stop_reason": "end_turn",
  "content": [
    {"type": "server_tool_use", "id": "srvtoolu_1", "name": "web_search",
     "input": {"query": "today"}},
    {"type": "web_search_tool_result", "tool_use_id": "srvtoolu_1",
     "content": [{"type": "web_search_result", "url": "https://example.org/a",
                  "title": "A"}]},
    {"type": "text", "text": "Сьогодні тепло.",
     "citations": [{"type": "web_search_result_location",
                    "url": "https://example.org/a", "title": "A",
                    "cited_text": "тепло і сонячно"}]}
  ],
  "usage": {"input_tokens": 10, "output_tokens": 5,
            "server_tool_use": {"web_search_requests": 2}}
}`

func TestGenerateReportsSearchAndCitations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(searchReply))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))
	resp, err := c.Generate(context.Background(),
		askForSearch(ai.Hosted{Kind: ai.HostedWebSearch}))
	if err != nil {
		t.Fatal(err)
	}

	// The provider's own tool blocks must not become tool calls: nobody is
	// waiting to answer them, and a tool loop would hang on one.
	if calls := resp.ToolCalls(); len(calls) != 0 {
		t.Errorf("ToolCalls() = %+v, want none", calls)
	}

	want := ai.HostedReport{
		Kind: ai.HostedWebSearch, Mode: ai.HostedNative, Calls: 2,
	}
	if len(resp.Hosted) != 1 || resp.Hosted[0] != want {
		t.Errorf("Hosted = %+v, want %+v", resp.Hosted, want)
	}

	cs := resp.Citations()
	if len(cs) != 1 {
		t.Fatalf("Citations() = %+v, want one", cs)
	}
	if cs[0].URL != "https://example.org/a" || cs[0].CitedText != "тепло і сонячно" {
		t.Errorf("citation = %+v", cs[0])
	}
	if cs[0].StartByte != 0 || cs[0].EndByte != 0 {
		t.Errorf("citation carries a byte range this provider never reported: %+v",
			cs[0])
	}
}

// A provider that accepts the tool and answers without it is the case the
// whole report exists for: the answer looks the same either way.
func TestGenerateReportsASkippedSearch(t *testing.T) {
	const reply = `{"model":"m","stop_reason":"end_turn",
	  "content":[{"type":"text","text":"I already know."}],
	  "usage":{"input_tokens":1,"output_tokens":1}}`

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(reply))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))

	resp, err := c.Generate(context.Background(),
		askForSearch(ai.Hosted{Kind: ai.HostedWebSearch}))
	if err != nil {
		t.Fatal(err)
	}
	want := ai.HostedReport{Kind: ai.HostedWebSearch, Mode: ai.HostedSkipped}
	if len(resp.Hosted) != 1 || resp.Hosted[0] != want {
		t.Errorf("Hosted = %+v, want %+v", resp.Hosted, want)
	}

	// The same reply, asked for with HostedRequired, is not an answer to the
	// question that was asked.
	_, err = c.Generate(context.Background(), askForSearch(ai.Hosted{
		Kind:   ai.HostedWebSearch,
		Policy: ai.HostedRequired,
	}))
	if !errors.Is(err, ai.ErrHostedRequired) {
		t.Errorf("Generate() error = %v, want ErrHostedRequired", err)
	}
}

func TestStreamCarriesCitationsAndReport(t *testing.T) {
	const events = `event: message_start
data: {"type":"message_start","message":{"model":"m","usage":{"input_tokens":4}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"s1","name":"web_search"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Сьогодні"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"citations_delta","citation":{"type":"web_search_result_location","url":"https://example.org/a","title":"A","cited_text":"тепло"}}}

event: message_delta
data: {"type":"message_delta","usage":{"output_tokens":9,"server_tool_use":{"web_search_requests":1}}}

event: message_stop
data: {"type":"message_stop"}

`

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "text/event-stream")
			_, _ = w.Write([]byte(events))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))

	var text strings.Builder
	var cites []ai.Citation
	var done ai.Chunk
	for chunk, err := range c.Stream(context.Background(),
		askForSearch(ai.Hosted{Kind: ai.HostedWebSearch})) {
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(chunk.Text)
		cites = append(cites, chunk.Citations...)
		if chunk.Done {
			done = chunk
		}
	}

	if text.String() != "Сьогодні" {
		t.Errorf("text = %q", text.String())
	}
	if len(cites) != 1 || cites[0].CitedText != "тепло" {
		t.Errorf("citations = %+v", cites)
	}
	want := ai.HostedReport{
		Kind: ai.HostedWebSearch, Mode: ai.HostedNative, Calls: 1,
	}
	if len(done.Hosted) != 1 || done.Hosted[0] != want {
		t.Errorf("Hosted on the final chunk = %+v, want %+v", done.Hosted, want)
	}
}

// Without a reported count, the provider's own tool blocks still prove the
// search happened. Proving it ran matters more than the exact number.
func TestStreamFallsBackToCountingBlocks(t *testing.T) {
	const events = `event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"s1","name":"web_search"}}

event: message_stop
data: {"type":"message_stop"}

`

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "text/event-stream")
			_, _ = w.Write([]byte(events))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))

	var done ai.Chunk
	for chunk, err := range c.Stream(context.Background(),
		askForSearch(ai.Hosted{Kind: ai.HostedWebSearch})) {
		if err != nil {
			t.Fatal(err)
		}
		if chunk.Done {
			done = chunk
		}
	}

	if len(done.Hosted) != 1 || done.Hosted[0].Mode != ai.HostedNative {
		t.Errorf("Hosted = %+v, want a native search", done.Hosted)
	}
}
