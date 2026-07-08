package anthropic

import "github.com/goloop/ai"

// Defaults for a new Client.
const (
	// DefaultBaseURL is the Anthropic API base URL.
	DefaultBaseURL = "https://api.anthropic.com"
	// DefaultVersion is the anthropic-version header sent with every request.
	DefaultVersion = "2023-06-01"
	// DefaultMaxTokens is used when a Request leaves MaxTokens unset, which the
	// Messages API requires.
	DefaultMaxTokens = 1024
)

// Convenience model identifiers. Any model string is accepted; the "-latest"
// aliases track the newest release of a model line. Use Models to discover
// what the account can call.
const (
	ModelClaude37SonnetLatest = "claude-3-7-sonnet-latest"
	ModelClaude35HaikuLatest  = "claude-3-5-haiku-latest"
	ModelClaudeSonnet4        = "claude-sonnet-4-20250514"
	ModelClaudeOpus4          = "claude-opus-4-20250514"
)

// Client is an Anthropic API client. It implements [ai.Client] and adds the
// provider's native endpoints (token counting, models, message batches).
type Client struct {
	opts      ai.Options
	version   string
	beta      []string
	maxTokens int
}

var _ ai.Client = (*Client)(nil)

// New returns a Client for the given API key. Shared options (WithBaseURL,
// WithHTTPClient, WithTimeout, WithMaxRetries, WithHeader) and Anthropic
// options (WithVersion, WithBeta, WithMaxTokens) configure it.
func New(apiKey string, opts ...Option) *Client {
	s := settings{version: DefaultVersion, maxTokens: DefaultMaxTokens}
	for _, o := range opts {
		o(&s)
	}

	o := ai.NewOptions(apiKey, s.aiOpts...)
	if o.BaseURL == "" {
		o.BaseURL = DefaultBaseURL
	}

	return &Client{
		opts:      o,
		version:   s.version,
		beta:      s.beta,
		maxTokens: s.maxTokens,
	}
}
