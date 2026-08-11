package anthropic

import (
	"fmt"

	"github.com/goloop/ai"
)

// WebSearchToolType is the identifier of Anthropic's server-side web search
// tool. Anthropic versions its server tools by date and keeps older versions
// working, so the identifier is a constant here and can be replaced with
// [WithWebSearchTool] when a newer one ships before this package names it.
const WebSearchToolType = "web_search_20250305"

// webSearchToolName is the name the tool answers to inside a request. It is
// fixed by the provider rather than chosen here.
const webSearchToolName = "web_search"

// hostedTools converts the capabilities a request asked for into Anthropic's
// server-side tool declarations. They join the caller's own tools in the same
// list, which is how the Messages API takes them.
//
// A constraint this provider cannot express is an error rather than a search
// that quietly runs wider than it was told to: the caller asked for a limit
// for a reason, and an answer built from sources they excluded is worse than
// no answer.
func (c *Client) hostedTools(req *ai.Request) ([]ToolDefinition, error) {
	var out []ToolDefinition

	for _, h := range req.Hosted {
		switch h.Kind {
		case ai.HostedWebSearch:
			t, err := c.webSearchTool(h)
			if err != nil {
				return nil, err
			}
			out = append(out, t)
		default:
			return nil, fmt.Errorf("%w: %s", ai.ErrNoHosted, h.Kind)
		}
	}

	return out, nil
}

// webSearchTool builds the web search declaration for one capability.
func (c *Client) webSearchTool(h ai.Hosted) (ToolDefinition, error) {
	t := ToolDefinition{Type: c.webSearchType, Name: webSearchToolName}
	if h.Web == nil {
		return t, nil
	}

	// Anthropic takes one list or the other. Choosing for the caller would
	// mean silently dropping half of what they asked for, and the half that
	// gets dropped decides which sources the answer is built from.
	if len(h.Web.AllowDomains) > 0 && len(h.Web.BlockDomains) > 0 {
		return ToolDefinition{}, fmt.Errorf(
			"%w: web search takes allowed or blocked domains, not both",
			ai.ErrNoHosted)
	}

	t.MaxUses = h.Web.MaxUses
	t.AllowedDomains = h.Web.AllowDomains
	t.BlockedDomains = h.Web.BlockDomains
	if h.Web.Region != "" {
		t.UserLocation = &UserLocation{
			Type:    "approximate",
			Country: h.Web.Region,
		}
	}

	return t, nil
}

// hostedCalls reads how many times Anthropic ran each of its own tools. A
// reply with no such counts means nothing ran, which is a real outcome: the
// model can be offered a search and answer without it.
func hostedCalls(u Usage) map[ai.HostedKind]int {
	if u.ServerToolUse == nil || u.ServerToolUse.WebSearchRequests == 0 {
		return nil
	}
	return map[ai.HostedKind]int{
		ai.HostedWebSearch: u.ServerToolUse.WebSearchRequests,
	}
}

// convCitations maps Anthropic's citations onto the shared ones. Anthropic
// reports the fragment of the source it relied on and no position in the
// answer, so the byte range stays zero, which the shared type reads as "this
// source supports the whole part".
func convCitations(cs []Citation) []ai.Citation {
	if len(cs) == 0 {
		return nil
	}
	out := make([]ai.Citation, 0, len(cs))
	for _, c := range cs {
		out = append(out, ai.Citation{
			URL:       c.URL,
			Title:     c.Title,
			CitedText: c.CitedText,
		})
	}
	return out
}
