# Provider Rate Limiting Plan

## Goal

Prevent provider overload and external API rate limiting by enforcing a shared request rate per provider. All myses using the same provider should share a single limiter.

## Research Findings

### Ollama (local)

- No built-in API rate limits for local deployments.
- Bottleneck is hardware throughput and concurrency.
- We should cap request rate to avoid GPU overload and reduce tail latency.

### OpenCode Zen (remote)

- Public documentation does not specify explicit request rate limits.
- Users report stalled requests that are consistent with server-side rate limiting.
- We should enforce a conservative request rate and handle 429 responses.

### Implementation Approach

Use `golang.org/x/time/rate` token buckets. The provider factory owns the limiter and injects it into each provider instance. This makes rate limiting shared per provider rather than per mysis.

## Proposed Design

### Shared Limiter Per Provider Factory

- Each provider factory is instantiated once at startup.
- Each factory owns a `rate.Limiter` configured from `config.toml`.
- Providers created by a factory share the same limiter reference.

### Request Flow

1. Mysis prepares a request.
2. Provider waits on limiter (`limiter.Wait(ctx)`).
3. Provider executes the request.
4. If the request fails with 429, emit a rate limit event and return error.

## Configuration Changes

Add two optional fields to `ProviderConfig`:

```toml
[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.7
rate_limit = 2.0
rate_burst = 3

[providers.opencode_zen]
endpoint = "https://api.opencode.ai/v1"
model = "glm-4.7-free"
temperature = 0.7
rate_limit = 10.0
rate_burst = 5
```

Defaults if omitted:

- `rate_limit`: 2.0 for Ollama, 10.0 for OpenCode Zen
- `rate_burst`: 3 for Ollama, 5 for OpenCode Zen

## Implementation Steps

### Phase 1: Config Layer

- Add `RateLimit` and `RateBurst` fields to `ProviderConfig`.
- Add environment overrides:
  - `ZOEA_OLLAMA_RATE_LIMIT`
  - `ZOEA_OLLAMA_RATE_BURST`
  - `ZOEA_OPENCODE_RATE_LIMIT`
  - `ZOEA_OPENCODE_RATE_BURST`

### Phase 2: Provider Factory

- Update `OllamaFactory` and `OpenCodeFactory` to own a limiter.
- Add constructors that accept `rate_limit` and `rate_burst`.
- Factory `Create()` injects the limiter into the provider instance.

### Phase 3: Provider Calls

- Add limiter field to `OllamaProvider` and `OpenCodeProvider`.
- Wrap all `Chat`, `ChatWithTools`, and `Stream` entry points with `limiter.Wait(ctx)`.
- If `Wait` returns an error, return immediately to caller.

### Phase 4: Error Reporting

- Add a dedicated event type (for UI/telemetry) when rate limit triggers.
- On HTTP 429, emit that event with provider name and model.

### Phase 5: Tests

- Config tests: verify parsing of `rate_limit` and `rate_burst`.
- Provider tests: ensure limiter blocks or delays requests.
- Commander tests: ensure myses sharing a provider are rate limited together.

## Files To Change

- `internal/config/config.go`
- `config.toml`
- `internal/provider/factory.go`
- `internal/provider/ollama.go`
- `internal/provider/opencode.go`
- `internal/provider/provider.go`
- `internal/core/events.go`
- `internal/core/commander.go`
- `documentation/KNOWN_ISSUES.md`
- Tests: `internal/config/config_test.go`, `internal/provider/*_test.go`, `internal/core/commander_test.go`

## Verification

1. `make test`
2. `make build`
3. Manual test: start two myses on the same provider, send rapid messages, observe throttling
