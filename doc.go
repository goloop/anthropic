// Package anthropic is a client for the Anthropic (Claude) API, built on the
// goloop/ai interface.
//
// The Client implements ai.Client, so Generate and Stream work the same as
// with any other goloop AI provider. On top of that it exposes Anthropic's
// native endpoints: the Messages API with Anthropic-only options (top_k,
// extended thinking, metadata and prompt caching), token counting, model
// listing and the message batches API.
//
//	c := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
//	resp, err := c.Generate(ctx, &ai.Request{
//	    Model:    anthropic.ModelClaudeSonnet5,
//	    Messages: []ai.Message{ai.UserText("Say hello in one word.")},
//	})
//
// # Structured output
//
// ai.Request.Format is honoured, but this provider has no response_format of
// its own, so the request is put to the model in the system prompt - in the
// wording every driver without native support shares - and ai.Response.Format
// reports ai.FormatEmulated. Read that literally: the model was asked, not
// constrained. ai.Response.JSON decodes the reply, unwrapping the code fence a
// model asked this way tends to add.
//
// # Hosted web search
//
// ai.Request.Hosted maps onto Anthropic's server-side web search tool, which
// rides in the same tools list as the caller's own:
//
//	resp, err := c.Generate(ctx, &ai.Request{
//	    Model:    anthropic.ModelClaudeSonnet5,
//	    Messages: []ai.Message{ai.UserText("What shipped this week?")},
//	    Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
//	})
//	for _, c := range resp.Citations() { ... }
//
// The provider runs the search itself, so its own tool blocks never surface as
// ai.ToolUse parts: a tool loop sees nothing new and has nothing extra to
// answer. The sources come back as ai.Citation values on the text they
// support. Anthropic reports the fragment of the source it used rather than a
// position in the answer, so Citation.CitedText is filled and the byte range
// stays zero.
//
// ai.Response.Hosted says whether the search actually ran. A model offered a
// search can answer without it, and the two answers are indistinguishable from
// the outside; ai.HostedRequired turns that into ai.ErrHostedRequired instead
// of an answer that only looks researched.
//
// Anthropic takes either allowed or blocked domains, never both, so a request
// that sets both is ai.ErrNoHosted before it leaves. The tool is versioned by
// date; WithWebSearchTool reaches a version newer than [WebSearchToolType].
//
// Search combines with a Format here, because this driver has no native
// structured output to conflict with it: the format is asked for in the system
// prompt either way.
//
// # Asking what this driver can do
//
// Capabilities describes this driver for the decision taken before a call:
// whether to offer a feature at all, and whether it needs one request or two.
//
//	if ai.SupportsHosted(c, ai.Hosted{Kind: ai.HostedWebSearch}) { ... }
//
// It is a hint and not a permission - support also depends on the model, the
// account and the region - so ai.ErrNoHosted and ai.ErrNoFormat remain the
// source of truth and a caller still handles them. What changes is that a
// refusal the provider only reports as a 400 now arrives as those same
// sentinels, wrapped around the original ai.APIError, so one errors.Is covers
// a limitation this driver knew in advance and one it learned over the wire.
//
// It speaks the Messages API, including system prompts, multimodal image
// input, tool use and streaming, and depends only on goloop/ai and the
// standard library.
package anthropic
