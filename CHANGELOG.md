# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.4] - 2026-07-10

### Documentation
- `DOC.md`/`DOC.UK.md` note that `Models` and `ListBatches` follow the API's
  cursor pagination and return every page in one call.

## [0.1.3] - 2026-07-10

### Fixed
- `ListBatches` and `Models` now return every page. They previously returned
  only the first page (up to the limit), silently dropping the rest.

### Changed
- Require `goloop/ai` v0.2.0 (500 no longer retried; jittered backoff).

## [0.1.2] - 2026-07-09

### Changed
- Model convenience constants now point at the current generation:
  `ModelClaudeSonnet5`, `ModelClaudeOpus48` and `ModelClaudeHaiku45`.

### Removed
- The `ModelClaude37SonnetLatest`, `ModelClaude35HaikuLatest`,
  `ModelClaudeSonnet4` and `ModelClaudeOpus4` constants, whose model names the
  API no longer serves. Any model string is still accepted directly, and
  `Models` lists what an account can call.

## [0.1.1] - 2026-07-09

### Changed
- Require `goloop/ai` v0.1.1, so exhausted retries now surface the provider's
  error body instead of a bare status.

### Added
- Native Messages API: `Messages` and `MessagesStream` over exported request
  and response types (`MessagesRequest`, `MessageParam`, `ContentBlock`, ...),
  exposing Anthropic-only options - `TopK`, `Thinking` (extended thinking),
  `Metadata` and prompt caching via `CacheControl`. `Usage` now reports cache
  token counts.

### Fixed
- A mid-stream `error` event now reports a meaningful status (for example 529
  for `overloaded_error`) instead of the HTTP 200 of the open connection.
- A stream that ends before `message_stop` now surfaces `io.ErrUnexpectedEOF`
  instead of ending silently.

## [0.1.0]

First release: a client for the Anthropic (Claude) API on the `goloop/ai`
interface.

### Added
- `Client` implementing `ai.Client`: `Generate` for a single response and
  `Stream` for token-by-token streaming over the Messages API.
- System prompts, multimodal image input and tool use (function calling),
  including streamed tool calls.
- Native endpoints: `CountTokens`, `Models`/`GetModel` and the message batches
  API (`CreateBatch`, `GetBatch`, `ListBatches`, `CancelBatch`, `BatchResults`).
- Functional options: `WithBaseURL`, `WithHTTPClient`, `WithTimeout`,
  `WithMaxRetries`, `WithHeader`, `WithVersion`, `WithBeta`, `WithMaxTokens`.
- Retries on 429 and 5xx with backoff; normalized `*ai.APIError` errors.
