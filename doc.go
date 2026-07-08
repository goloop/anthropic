// Package anthropic is a client for the Anthropic (Claude) API, built on the
// goloop/ai interface.
//
// The Client implements ai.Client, so Generate and Stream work the same as
// with any other goloop AI provider. On top of that it exposes Anthropic's
// native endpoints: token counting, model listing and the message batches API.
//
//	c := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
//	resp, err := c.Generate(ctx, &ai.Request{
//	    Model:    anthropic.ModelClaude37SonnetLatest,
//	    Messages: []ai.Message{ai.UserText("Say hello in one word.")},
//	})
//
// It speaks the Messages API, including system prompts, multimodal image
// input, tool use and streaming, and depends only on goloop/ai and the
// standard library.
package anthropic
