package anthropic

import (
	"net/http"
	"time"

	"github.com/goloop/ai"
)

// settings accumulates configuration during New. Shared options are collected
// as ai.Options; the rest are Anthropic-specific.
type settings struct {
	aiOpts    []ai.Option
	version   string
	beta      []string
	maxTokens int
}

// Option configures a Client in New.
type Option func(*settings)

// WithBaseURL overrides the API base URL (proxies, gateways, mock servers).
func WithBaseURL(u string) Option {
	return func(s *settings) { s.aiOpts = append(s.aiOpts, ai.WithBaseURL(u)) }
}

// WithHTTPClient sets the HTTP client used for requests.
func WithHTTPClient(c *http.Client) Option {
	return func(s *settings) { s.aiOpts = append(s.aiOpts, ai.WithHTTPClient(c)) }
}

// WithTimeout sets the per-request timeout when no custom HTTP client is set.
func WithTimeout(d time.Duration) Option {
	return func(s *settings) { s.aiOpts = append(s.aiOpts, ai.WithTimeout(d)) }
}

// WithMaxRetries sets how many times a request is retried on 429 and 5xx.
func WithMaxRetries(n int) Option {
	return func(s *settings) { s.aiOpts = append(s.aiOpts, ai.WithMaxRetries(n)) }
}

// WithHeader adds a header sent with every request.
func WithHeader(key, value string) Option {
	return func(s *settings) { s.aiOpts = append(s.aiOpts, ai.WithHeader(key, value)) }
}

// WithVersion overrides the anthropic-version header.
func WithVersion(v string) Option {
	return func(s *settings) { s.version = v }
}

// WithBeta enables one or more anthropic-beta feature flags.
func WithBeta(features ...string) Option {
	return func(s *settings) { s.beta = append(s.beta, features...) }
}

// WithMaxTokens sets the default max_tokens used when a Request leaves it unset.
func WithMaxTokens(n int) Option {
	return func(s *settings) { s.maxTokens = n }
}
