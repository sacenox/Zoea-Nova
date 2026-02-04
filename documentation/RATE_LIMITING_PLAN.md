# Provider Rate Limiting

## Overview

Zoea Nova enforces shared request limits per provider to prevent GPU overload (Ollama) and reduce the risk of upstream throttling (OpenCode Zen). Every mysis created with the same provider shares a single limiter.

## Implementation

- Each provider factory owns a `golang.org/x/time/rate` limiter created at startup.
- Providers created by the factory reuse that limiter, so the rate is shared across all myses for that provider.
- Requests in `Chat`, `ChatWithTools`, and `Stream` call `limiter.Wait(ctx)` before executing.

## Configuration

Rate limits are configured per provider in `config.toml`.

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

## Environment Overrides

- `ZOEA_OLLAMA_RATE_LIMIT`
- `ZOEA_OLLAMA_RATE_BURST`
- `ZOEA_OPENCODE_RATE_LIMIT`
- `ZOEA_OPENCODE_RATE_BURST`

## Request Flow

1. Mysis prepares a request.
2. Provider waits on the shared limiter with `limiter.Wait(ctx)`.
3. Provider executes the request if the limiter allows it.

## Behavior Notes

- When the limiter blocks, the call waits until tokens are available or the context is canceled.
- Limiter errors are returned to the caller without additional event emission.
