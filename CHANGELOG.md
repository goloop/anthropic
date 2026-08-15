# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.3] - 2026-08-15

Patch release.

### Changed
- Depends on `ai` v1.2.0, which adds the `Capabilities.Images` hint. This driver
  does not generate images, so it reports `Images: false` (the default) - no
  behavior change.

## [1.1.1] - 2026-08-11

Patch release.

### Documentation
- The reference covers the driver's `Capabilities` and the translation of
  model-level 400 refusals into `ai.ErrNoHosted`/`ai.ErrNoFormat`, which until
  now lived only in the godoc.

## [1.1.0] - 2026-08-11

Minor release, on `ai` v1.1.0.

### Added
- `Capabilities` describes what this driver can be asked for: which hosted
  capabilities it runs, which `ai.HostedWeb` settings it can express, and
  whether a search survives in the same call as a structured format. It is a
  hint for the decision taken before a call, never a substitute for handling
  `ai.ErrNoHosted` - support also depends on the model, the account and the
  region. A test pins the table against what the driver actually refuses, so
  it stays honest mechanically rather than by discipline.
- The reported format modes are all emulated, including `Strict`: this provider
  has no `response_format`, and saying otherwise would promise enforcement that
  does not exist. That same emulation is why a search and a format hold
  together here in one call.
- A provider refusal that means "this model cannot do that" is now translated
  into the matching sentinel, so an application degrades with one `errors.Is`
  whether the driver knew in advance or learned from a 400. The provider's own
  `ai.APIError` is wrapped, not replaced, and stays reachable with `errors.As`.
  The rules are deliberately narrow - only a 400, only a capability the caller
  asked for, only an error naming that exact feature - because mistaking a
  genuinely bad request for a missing feature would retry it forever.

## [1.0.0] - 2026-08-11

First stable release, on `ai` v1.0.0.

### Added
- `ai.Request.Hosted` maps onto the server-side web search tool, which rides in
  the same tools list as the caller's own. The provider's own tool blocks never
  surface as `ai.ToolUse` parts, so a tool loop sees nothing new and has nothing
  extra to answer.
- Sources come back as `ai.Citation` values on the text they support. This
  provider reports the fragment of the source it used rather than a position in
  the answer, so `CitedText` is filled and the byte range stays zero.
- `ai.Response.Hosted` reports whether the search actually ran, with the count
  the provider bills for; `ai.HostedRequired` turns a search that did not happen
  into an error rather than an answer that only looks researched.
- A stream carries the sources as their events arrive and the report on the
  final chunk.
- `WithWebSearchTool` reaches a newer version of the tool than
  `WebSearchToolType`, which the provider versions by date.
- `ToolDefinition` gained the fields a server-side tool needs, and `Usage`
  gained `ServerToolUse`.

### Fixed
- A content block whose `content` is a list rather than a string no longer
  fails the whole reply. A web search result block is exactly that shape, so
  before this one search result would have cost the answer it came with; those
  results are now read into `ContentBlock.SearchResults`.

### Changed
- The provider takes either allowed or blocked domains, never both, so a
  request that sets both is `ai.ErrNoHosted` rather than a search that silently
  runs wider than it was told to.

## [0.3.1] - 2026-08-05

### Documentation
- The package documentation describes how `ai.Request.Format` reaches this
  provider, so it is on the first page a reader sees rather than only in the
  reference.

## [0.3.0] - 2026-08-05

### Added
- `ai.Request.Format` is honoured. This provider has no `response_format` of
  its own, so the request is put to the model in the system prompt, in the
  wording every driver without native support shares (`ai.Format.Instruction`),
  and `ai.Response.Format` reports `ai.FormatEmulated` - a request, not a
  guarantee. The caller's own system prompt is kept and the instruction follows
  it. `ai.Response.JSON` decodes the reply, unwrapping the code fence a model
  asked this way tends to add.

### Changed
- Requires `github.com/goloop/ai` v0.4.0.

## [0.2.0] - 2026-07-12

### Fixed
- Response bodies are now read under a 64 MiB ceiling (`send`, the streaming
  error path and `BatchResults`), so a malformed or hostile server cannot
  exhaust memory with an unbounded body.
- The native `MessagesStream` now surfaces a malformed SSE JSON payload as an
  error instead of silently skipping the event, matching the provider-agnostic
  `Stream`.
- A streamed tool call whose accumulated arguments are not valid JSON is now
  reported as an error rather than yielded as an unparseable `Input`.
- `GetModel`, `GetBatch` and `CancelBatch` now escape the path segment, so an
  ID with reserved characters cannot alter the request URL.
- `Messages`, `MessagesStream` and `BatchResults` return an error instead of
  panicking when passed a nil request or batch.

### Changed
- Requires `github.com/goloop/ai` v0.3.0.

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
